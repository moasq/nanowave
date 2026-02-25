package orchestration

import (
	"fmt"
	"strings"
)

// generateProjectYAML produces the full project.yml content for XcodeGen.
// When no extensions are present, generates a single-target project.
// With extensions, generates multi-target YAML with proper dependencies.
// deviceFamilyBuildSettings returns TARGETED_DEVICE_FAMILY and orientation settings for the given device family.
func deviceFamilyBuildSettings(b *strings.Builder, family string) {
	switch family {
	case "ipad":
		b.WriteString("        TARGETED_DEVICE_FAMILY: \"2\"\n")
		b.WriteString("        INFOPLIST_KEY_UISupportedInterfaceOrientations_iPad: UIInterfaceOrientationPortrait UIInterfaceOrientationPortraitUpsideDown UIInterfaceOrientationLandscapeLeft UIInterfaceOrientationLandscapeRight\n")
	case "universal":
		b.WriteString("        TARGETED_DEVICE_FAMILY: \"1,2\"\n")
		b.WriteString("        INFOPLIST_KEY_UISupportedInterfaceOrientations_iPhone: UIInterfaceOrientationPortrait\n")
		b.WriteString("        INFOPLIST_KEY_UISupportedInterfaceOrientations_iPad: UIInterfaceOrientationPortrait UIInterfaceOrientationPortraitUpsideDown UIInterfaceOrientationLandscapeLeft UIInterfaceOrientationLandscapeRight\n")
	default: // "iphone"
		b.WriteString("        TARGETED_DEVICE_FAMILY: \"1\"\n")
		b.WriteString("        INFOPLIST_KEY_UISupportedInterfaceOrientations_iPhone: UIInterfaceOrientationPortrait\n")
	}
}

// appearanceBuildSettings writes Info.plist keys to lock the app to a single
// appearance mode when dark mode is not supported. Only iOS and tvOS get locked
// via INFOPLIST_KEY_UIUserInterfaceStyle. macOS and visionOS are excluded:
// macOS apps should always follow system appearance, and visionOS has no
// light/dark concept (glass auto-adapts).
func appearanceBuildSettings(b *strings.Builder, plan *PlannerResult, platform string) {
	if plan != nil && plan.HasRuleKey("dark-mode") {
		return // app supports both modes — don't lock
	}
	switch platform {
	case PlatformMacOS:
		// macOS: handled via info: properties (no INFOPLIST_KEY equivalent)
		return
	case PlatformVisionOS:
		// visionOS: no light/dark concept — glass material auto-adapts to environment
		return
	default:
		// iOS, tvOS: lock to Light via INFOPLIST_KEY
		b.WriteString("        INFOPLIST_KEY_UIUserInterfaceStyle: Light\n")
	}
}

// appearanceInfoPlistMacOS is intentionally a no-op. macOS apps should always
// follow the system appearance (dark/light). Unlike iOS where apps often have
// branded light backgrounds, macOS users expect apps to respect their system
// preference. Forcing Aqua with a dark custom palette creates visual mismatch.
func appearanceInfoPlistMacOS(b *strings.Builder, appName, sourceDir string, plan *PlannerResult) {
	// No-op: macOS apps follow system appearance by default.
}

// writeIOSDestinationSettings constrains Xcode "Supported Destinations" for iOS apps.
// supportedDestinations removes Mac/Vision "Designed for iPad" defaults, while
// destinationFilters narrows iOS devices (iPhone/iPad) based on the planned family.
func writeIOSDestinationSettings(b *strings.Builder, family string) {
	b.WriteString("    platform: iOS\n")
	b.WriteString("    supportedDestinations:\n")
	b.WriteString("      - iOS\n")
	switch family {
	case "ipad":
		b.WriteString("    destinationFilters:\n")
		b.WriteString("      - device: iPad\n")
	case "universal":
		b.WriteString("    destinationFilters:\n")
		b.WriteString("      - device: iPhone\n")
		b.WriteString("      - device: iPad\n")
	default: // "iphone"
		b.WriteString("    destinationFilters:\n")
		b.WriteString("      - device: iPhone\n")
	}
}

func generateProjectYAML(appName string, plan *PlannerResult) string {
	if plan != nil && plan.IsMultiPlatform() {
		return generateMultiPlatformProjectYAML(appName, plan)
	}

	platform := PlatformIOS
	if plan != nil {
		platform = plan.GetPlatform()
	}

	if IsWatchOS(platform) {
		shape := WatchShapeStandalone
		if plan != nil {
			shape = plan.GetWatchProjectShape()
		}
		if shape == WatchShapePaired {
			return generatePairedYAML(appName, plan)
		}
		return generateWatchOnlyYAML(appName, plan)
	}

	if IsTvOS(platform) {
		return generateTvOSProjectYAML(appName, plan)
	}

	if IsVisionOS(platform) {
		return generateVisionOSProjectYAML(appName, plan)
	}

	if IsMacOS(platform) {
		return generateMacOSProjectYAML(appName, plan)
	}

	return generateIOSProjectYAML(appName, plan)
}

// generateMultiPlatformProjectYAML produces a project.yml with targets for all platforms.
func generateMultiPlatformProjectYAML(appName string, plan *PlannerResult) string {
	var b strings.Builder

	bundleID := fmt.Sprintf("%s.%s", bundleIDPrefix(), strings.ToLower(appName))
	platforms := plan.GetPlatforms()
	hasWatchOS := HasPlatform(platforms, PlatformWatchOS)
	hasTvOS := HasPlatform(platforms, PlatformTvOS)
	hasVisionOS := HasPlatform(platforms, PlatformVisionOS)
	hasMacOS := HasPlatform(platforms, PlatformMacOS)
	hasExtensions := plan != nil && len(plan.Extensions) > 0

	// Header
	fmt.Fprintf(&b, "name: %s\n", appName)
	b.WriteString("options:\n")
	fmt.Fprintf(&b, "  bundleIdPrefix: %s\n", bundleIDPrefix())
	b.WriteString("  deploymentTarget:\n")
	b.WriteString("    iOS: \"26.0\"\n")
	if hasWatchOS {
		b.WriteString("    watchOS: \"26.0\"\n")
	}
	if hasTvOS {
		b.WriteString("    tvOS: \"26.0\"\n")
	}
	if hasVisionOS {
		b.WriteString("    visionOS: \"26.0\"\n")
	}
	if hasMacOS {
		b.WriteString("    macOS: \"26.0\"\n")
	}
	b.WriteString("  xcodeVersion: \"16.0\"\n")
	b.WriteString("  createIntermediateGroups: true\n")
	b.WriteString("  generateEmptyDirectories: true\n")
	b.WriteString("  useBaseInternationalization: false\n")

	if plan != nil && len(plan.Localizations) > 0 {
		b.WriteString("  knownRegions:\n")
		for _, lang := range plan.Localizations {
			fmt.Fprintf(&b, "    - %s\n", lang)
		}
	}
	b.WriteString("\n")

	b.WriteString("targets:\n")

	// iOS main target
	fmt.Fprintf(&b, "  %s:\n", appName)
	b.WriteString("    type: application\n")
	writeIOSDestinationSettings(&b, plan.GetDeviceFamily())
	b.WriteString("    sources:\n")
	fmt.Fprintf(&b, "      - path: %s\n", appName)
	b.WriteString("        type: syncedFolder\n")
	b.WriteString("      - path: Shared\n")
	b.WriteString("        type: syncedFolder\n")
	b.WriteString("        optional: true\n")

	b.WriteString("    settings:\n")
	b.WriteString("      base:\n")
	b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
	fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", bundleID)
	b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
	b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
	b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
	b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
	b.WriteString("        INFOPLIST_KEY_UIApplicationSceneManifest_Generation: YES\n")
	b.WriteString("        INFOPLIST_KEY_UIApplicationSupportsIndirectInputEvents: YES\n")
	b.WriteString("        INFOPLIST_KEY_UILaunchScreen_Generation: YES\n")
	deviceFamilyBuildSettings(&b, plan.GetDeviceFamily())
	b.WriteString("        ASSETCATALOG_COMPILER_APPICON_NAME: AppIcon\n")
	b.WriteString("        ASSETCATALOG_COMPILER_GLOBAL_ACCENT_COLOR_NAME: AccentColor\n")
	b.WriteString("        ENABLE_PREVIEWS: YES\n")
	b.WriteString("        SWIFT_EMIT_LOC_STRINGS: YES\n")
	b.WriteString("        LD_RUNPATH_SEARCH_PATHS:\n")
	b.WriteString("          - \"$(inherited)\"\n")
	b.WriteString("          - \"@executable_path/Frameworks\"\n")
	b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
	b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")
	appearanceBuildSettings(&b, plan, PlatformIOS)

	if plan != nil {
		for _, perm := range plan.Permissions {
			fmt.Fprintf(&b, "        INFOPLIST_KEY_%s: %s\n", perm.Key, xcodeYAMLQuote(perm.Description))
		}
	}

	b.WriteString("    entitlements:\n")
	fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", appName, appName)
	b.WriteString("      properties: {}\n")

	// iOS dependencies: embed watch target and iOS extensions
	hasIOSDeps := hasWatchOS
	if !hasIOSDeps && hasExtensions {
		for _, ext := range plan.Extensions {
			if ext.Platform == "" || ext.Platform == PlatformIOS {
				hasIOSDeps = true
				break
			}
		}
	}
	if hasIOSDeps {
		b.WriteString("    dependencies:\n")
		if hasWatchOS {
			watchAppName := watchAppTargetName(appName)
			fmt.Fprintf(&b, "      - target: %s\n", watchAppName)
			b.WriteString("        embed: true\n")
		}
		if hasExtensions {
			for _, ext := range plan.Extensions {
				if ext.Platform != "" && ext.Platform != PlatformIOS {
					continue // skip non-iOS extensions
				}
				name := extensionTargetName(ext, appName)
				fmt.Fprintf(&b, "      - target: %s\n", name)
				b.WriteString("        embed: true\n")
			}
		}
	}

	// watchOS targets (when present)
	if hasWatchOS {
		watchAppName := watchAppTargetName(appName)
		watchBundleID := bundleID + ".watchkitapp"
		watchExtName := watchExtensionTargetName(appName)
		watchExtBundleID := watchBundleID + ".watchkitextension"

		b.WriteString("\n")
		fmt.Fprintf(&b, "  %s:\n", watchAppName)
		b.WriteString("    type: application.watchapp2\n")
		b.WriteString("    platform: watchOS\n")
		b.WriteString("    sources:\n")
		writeSyncedSourceEntry(&b, appName+"Watch", []string{"**/*.swift", "*.plist", "*.entitlements"}, false)
		b.WriteString("    settings:\n")
		b.WriteString("      base:\n")
		b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
		fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", watchBundleID)
		b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
		b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
		b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
		b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
		writeWatchOSBuildSettings(&b)
		b.WriteString("    entitlements:\n")
		fmt.Fprintf(&b, "      path: %sWatch/%s.entitlements\n", appName, watchAppName)
		b.WriteString("      properties: {}\n")
		b.WriteString("    info:\n")
		fmt.Fprintf(&b, "      path: %sWatch/Info.plist\n", appName)
		b.WriteString("      properties:\n")
		fmt.Fprintf(&b, "        WKCompanionAppBundleIdentifier: %s\n", xcodeYAMLQuote(bundleID))
		b.WriteString("        WKRunsIndependentlyOfCompanionApp: true\n")
		b.WriteString("    dependencies:\n")
		fmt.Fprintf(&b, "      - target: %s\n", watchExtName)
		b.WriteString("        embed: true\n")

		// Watch extension target
		writeIntrinsicWatchExtensionTargetYAML(&b, watchExtName, appName+"Watch", watchExtBundleID, watchBundleID, true)
	}

	// tvOS target (when present)
	if hasTvOS {
		tvTargetName := appName + "TV"
		tvBundleID := bundleID + ".tv"

		b.WriteString("\n")
		fmt.Fprintf(&b, "  %s:\n", tvTargetName)
		b.WriteString("    type: application\n")
		b.WriteString("    platform: tvOS\n")
		b.WriteString("    supportedDestinations:\n")
		b.WriteString("      - tvOS\n")
		b.WriteString("    sources:\n")
		fmt.Fprintf(&b, "      - path: %sTV\n", appName)
		b.WriteString("        type: syncedFolder\n")
		b.WriteString("      - path: Shared\n")
		b.WriteString("        type: syncedFolder\n")
		b.WriteString("        optional: true\n")

		b.WriteString("    settings:\n")
		b.WriteString("      base:\n")
		b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
		fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", tvBundleID)
		b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
		b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
		b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
		b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
		b.WriteString("        TARGETED_DEVICE_FAMILY: \"3\"\n")
		b.WriteString("        ASSETCATALOG_COMPILER_APPICON_NAME: AppIcon\n")
		b.WriteString("        ASSETCATALOG_COMPILER_GLOBAL_ACCENT_COLOR_NAME: AccentColor\n")
		b.WriteString("        ENABLE_PREVIEWS: YES\n")
		b.WriteString("        SWIFT_EMIT_LOC_STRINGS: YES\n")
		b.WriteString("        LD_RUNPATH_SEARCH_PATHS:\n")
		b.WriteString("          - \"$(inherited)\"\n")
		b.WriteString("          - \"@executable_path/Frameworks\"\n")
		b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
		b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")
		appearanceBuildSettings(&b, plan, PlatformTvOS)

		b.WriteString("    entitlements:\n")
		fmt.Fprintf(&b, "      path: %sTV/%s.entitlements\n", appName, tvTargetName)
		b.WriteString("      properties: {}\n")

		// tvOS extensions
		if hasExtensions {
			hasTvExtensions := false
			for _, ext := range plan.Extensions {
				if ext.Platform == PlatformTvOS {
					hasTvExtensions = true
					break
				}
			}
			if hasTvExtensions {
				b.WriteString("    dependencies:\n")
				for _, ext := range plan.Extensions {
					if ext.Platform != PlatformTvOS {
						continue
					}
					name := extensionTargetName(ext, appName)
					fmt.Fprintf(&b, "      - target: %s\n", name)
					b.WriteString("        embed: true\n")
				}
			}
		}
	}

	// visionOS target (when present)
	if hasVisionOS {
		visionTargetName := appName + "Vision"
		visionBundleID := bundleID + ".vision"

		b.WriteString("\n")
		fmt.Fprintf(&b, "  %s:\n", visionTargetName)
		b.WriteString("    type: application\n")
		b.WriteString("    platform: visionOS\n")
		b.WriteString("    supportedDestinations:\n")
		b.WriteString("      - visionOS\n")
		b.WriteString("    sources:\n")
		fmt.Fprintf(&b, "      - path: %sVision\n", appName)
		b.WriteString("        type: syncedFolder\n")
		b.WriteString("      - path: Shared\n")
		b.WriteString("        type: syncedFolder\n")
		b.WriteString("        optional: true\n")

		b.WriteString("    settings:\n")
		b.WriteString("      base:\n")
		b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
		fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", visionBundleID)
		b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
		b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
		b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
		b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
		b.WriteString("        TARGETED_DEVICE_FAMILY: \"7\"\n")
		b.WriteString("        ASSETCATALOG_COMPILER_APPICON_NAME: AppIcon\n")
		b.WriteString("        ASSETCATALOG_COMPILER_GLOBAL_ACCENT_COLOR_NAME: AccentColor\n")
		b.WriteString("        ENABLE_PREVIEWS: YES\n")
		b.WriteString("        SWIFT_EMIT_LOC_STRINGS: YES\n")
		b.WriteString("        LD_RUNPATH_SEARCH_PATHS:\n")
		b.WriteString("          - \"$(inherited)\"\n")
		b.WriteString("          - \"@executable_path/Frameworks\"\n")
		b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
		b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")
		appearanceBuildSettings(&b, plan, PlatformVisionOS)

		b.WriteString("    entitlements:\n")
		fmt.Fprintf(&b, "      path: %sVision/%s.entitlements\n", appName, visionTargetName)
		b.WriteString("      properties: {}\n")

		// visionOS extensions
		if hasExtensions {
			hasVisionExtensions := false
			for _, ext := range plan.Extensions {
				if ext.Platform == PlatformVisionOS {
					hasVisionExtensions = true
					break
				}
			}
			if hasVisionExtensions {
				b.WriteString("    dependencies:\n")
				for _, ext := range plan.Extensions {
					if ext.Platform != PlatformVisionOS {
						continue
					}
					name := extensionTargetName(ext, appName)
					fmt.Fprintf(&b, "      - target: %s\n", name)
					b.WriteString("        embed: true\n")
				}
			}
		}
	}

	// macOS target (when present)
	if hasMacOS {
		macTargetName := appName + "Mac"
		macBundleID := bundleID + ".mac"

		b.WriteString("\n")
		fmt.Fprintf(&b, "  %s:\n", macTargetName)
		b.WriteString("    type: application\n")
		b.WriteString("    platform: macOS\n")
		b.WriteString("    supportedDestinations:\n")
		b.WriteString("      - macOS\n")
		b.WriteString("    sources:\n")
		fmt.Fprintf(&b, "      - path: %sMac\n", appName)
		b.WriteString("        type: syncedFolder\n")
		b.WriteString("      - path: Shared\n")
		b.WriteString("        type: syncedFolder\n")
		b.WriteString("        optional: true\n")

		b.WriteString("    settings:\n")
		b.WriteString("      base:\n")
		b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
		fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", macBundleID)
		b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
		b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
		b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
		b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
		b.WriteString("        ASSETCATALOG_COMPILER_APPICON_NAME: AppIcon\n")
		b.WriteString("        ASSETCATALOG_COMPILER_GLOBAL_ACCENT_COLOR_NAME: AccentColor\n")
		b.WriteString("        ENABLE_PREVIEWS: YES\n")
		b.WriteString("        SWIFT_EMIT_LOC_STRINGS: YES\n")
		b.WriteString("        COMBINE_HIDPI_IMAGES: YES\n")
		b.WriteString("        LD_RUNPATH_SEARCH_PATHS:\n")
		b.WriteString("          - \"$(inherited)\"\n")
		b.WriteString("          - \"@executable_path/../Frameworks\"\n")
		b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
		b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")

		appearanceInfoPlistMacOS(&b, appName, appName+"Mac", plan)

		b.WriteString("    entitlements:\n")
		fmt.Fprintf(&b, "      path: %sMac/%s.entitlements\n", appName, macTargetName)
		b.WriteString("      properties: {}\n")

		// macOS extensions
		if hasExtensions {
			hasMacExtensions := false
			for _, ext := range plan.Extensions {
				if ext.Platform == PlatformMacOS {
					hasMacExtensions = true
					break
				}
			}
			if hasMacExtensions {
				b.WriteString("    dependencies:\n")
				for _, ext := range plan.Extensions {
					if ext.Platform != PlatformMacOS {
						continue
					}
					name := extensionTargetName(ext, appName)
					fmt.Fprintf(&b, "      - target: %s\n", name)
					b.WriteString("        embed: true\n")
				}
			}
		}
	}

	// Extension targets (per-platform)
	if hasExtensions {
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			kind := ext.Kind
			kindForBundleID := strings.ReplaceAll(kind, "_", "")
			if kindForBundleID == "" {
				kindForBundleID = strings.ToLower(name)
			}
			extBundleID := fmt.Sprintf("%s.%s", bundleID, kindForBundleID)
			sourcePath := fmt.Sprintf("Targets/%s", name)

			extPlatform := ext.Platform
			if extPlatform == "" {
				extPlatform = PlatformIOS
			}

			b.WriteString("\n")
			fmt.Fprintf(&b, "  %s:\n", name)
			fmt.Fprintf(&b, "    type: %s\n", xcodegenTargetType(kind))
			fmt.Fprintf(&b, "    platform: %s\n", PlatformXcodegenValue(extPlatform))
			b.WriteString("    sources:\n")
			fmt.Fprintf(&b, "      - path: %s\n", sourcePath)
			b.WriteString("        type: syncedFolder\n")
			b.WriteString("      - path: Shared\n")
			b.WriteString("        type: syncedFolder\n")
			b.WriteString("        optional: true\n")
			b.WriteString("    settings:\n")
			b.WriteString("      base:\n")
			fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", extBundleID)
			b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
			b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
			b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
			b.WriteString("        SKIP_INSTALL: YES\n")
			b.WriteString("        DEAD_CODE_STRIPPING: NO\n")
			b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
			b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
			b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
			b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")

			for k, v := range ext.Settings {
				fmt.Fprintf(&b, "        %s: %s\n", k, xcodeYAMLQuote(v))
			}

			infoPlist := mergeInfoPlistDefaults(kind, ext.InfoPlist)
			if len(infoPlist) > 0 {
				b.WriteString("    info:\n")
				fmt.Fprintf(&b, "      path: %s/Info.plist\n", sourcePath)
				b.WriteString("      properties:\n")
				writeXcodeYAMLMap(&b, infoPlist, 8)
			}

			entitlements := mergeEntitlementDefaults(kind, ext.Entitlements, bundleID)
			if len(entitlements) > 0 {
				b.WriteString("    entitlements:\n")
				fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", sourcePath, name)
				b.WriteString("      properties:\n")
				writeXcodeYAMLMap(&b, entitlements, 8)
			}
		}
	}

	// Schemes
	b.WriteString("\nschemes:\n")

	// iOS scheme
	fmt.Fprintf(&b, "  %s:\n", appName)
	b.WriteString("    build:\n")
	b.WriteString("      targets:\n")
	fmt.Fprintf(&b, "        %s: all\n", appName)
	if hasWatchOS {
		fmt.Fprintf(&b, "        %s: all\n", watchAppTargetName(appName))
		fmt.Fprintf(&b, "        %s: all\n", watchExtensionTargetName(appName))
	}
	if hasExtensions {
		for _, ext := range plan.Extensions {
			if ext.Platform != "" && ext.Platform != PlatformIOS {
				continue
			}
			name := extensionTargetName(ext, appName)
			fmt.Fprintf(&b, "        %s: all\n", name)
		}
	}
	b.WriteString("    run:\n")
	fmt.Fprintf(&b, "      executable: %s\n", appName)

	// tvOS scheme
	if hasTvOS {
		tvTargetName := appName + "TV"
		fmt.Fprintf(&b, "\n  %s:\n", tvTargetName)
		b.WriteString("    build:\n")
		b.WriteString("      targets:\n")
		fmt.Fprintf(&b, "        %s: all\n", tvTargetName)
		if hasExtensions {
			for _, ext := range plan.Extensions {
				if ext.Platform != PlatformTvOS {
					continue
				}
				name := extensionTargetName(ext, appName)
				fmt.Fprintf(&b, "        %s: all\n", name)
			}
		}
		b.WriteString("    run:\n")
		fmt.Fprintf(&b, "      executable: %s\n", tvTargetName)
	}

	// visionOS scheme
	if hasVisionOS {
		visionTargetName := appName + "Vision"
		fmt.Fprintf(&b, "\n  %s:\n", visionTargetName)
		b.WriteString("    build:\n")
		b.WriteString("      targets:\n")
		fmt.Fprintf(&b, "        %s: all\n", visionTargetName)
		if hasExtensions {
			for _, ext := range plan.Extensions {
				if ext.Platform != PlatformVisionOS {
					continue
				}
				name := extensionTargetName(ext, appName)
				fmt.Fprintf(&b, "        %s: all\n", name)
			}
		}
		b.WriteString("    run:\n")
		fmt.Fprintf(&b, "      executable: %s\n", visionTargetName)
	}

	// macOS scheme
	if hasMacOS {
		macTargetName := appName + "Mac"
		fmt.Fprintf(&b, "\n  %s:\n", macTargetName)
		b.WriteString("    build:\n")
		b.WriteString("      targets:\n")
		fmt.Fprintf(&b, "        %s: all\n", macTargetName)
		if hasExtensions {
			for _, ext := range plan.Extensions {
				if ext.Platform != PlatformMacOS {
					continue
				}
				name := extensionTargetName(ext, appName)
				fmt.Fprintf(&b, "        %s: all\n", name)
			}
		}
		b.WriteString("    run:\n")
		fmt.Fprintf(&b, "      executable: %s\n", macTargetName)
	}

	return b.String()
}

// generateMacOSProjectYAML produces a native macOS project.yml.
func generateMacOSProjectYAML(appName string, plan *PlannerResult) string {
	var b strings.Builder

	bundleID := fmt.Sprintf("%s.%s", bundleIDPrefix(), strings.ToLower(appName))
	hasExtensions := plan != nil && len(plan.Extensions) > 0

	fmt.Fprintf(&b, "name: %s\n", appName)
	b.WriteString("options:\n")
	fmt.Fprintf(&b, "  bundleIdPrefix: %s\n", bundleIDPrefix())
	b.WriteString("  deploymentTarget:\n")
	b.WriteString("    macOS: \"26.0\"\n")
	b.WriteString("  xcodeVersion: \"16.0\"\n")
	b.WriteString("  createIntermediateGroups: true\n")
	b.WriteString("  generateEmptyDirectories: true\n")
	b.WriteString("  useBaseInternationalization: false\n")

	if plan != nil && len(plan.Localizations) > 0 {
		b.WriteString("  knownRegions:\n")
		for _, lang := range plan.Localizations {
			fmt.Fprintf(&b, "    - %s\n", lang)
		}
	}
	b.WriteString("\n")

	b.WriteString("targets:\n")

	// Main app target
	fmt.Fprintf(&b, "  %s:\n", appName)
	b.WriteString("    type: application\n")
	b.WriteString("    platform: macOS\n")
	b.WriteString("    supportedDestinations:\n")
	b.WriteString("      - macOS\n")
	b.WriteString("    sources:\n")
	fmt.Fprintf(&b, "      - path: %s\n", appName)
	b.WriteString("        type: syncedFolder\n")
	if hasExtensions {
		b.WriteString("      - path: Shared\n")
		b.WriteString("        type: syncedFolder\n")
		b.WriteString("        optional: true\n")
	}

	// Settings — no TARGETED_DEVICE_FAMILY, no UILaunchScreen, no UIApplicationSceneManifest, no orientations
	b.WriteString("    settings:\n")
	b.WriteString("      base:\n")
	b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
	fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", bundleID)
	b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
	b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
	b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
	b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
	b.WriteString("        ASSETCATALOG_COMPILER_APPICON_NAME: AppIcon\n")
	b.WriteString("        ASSETCATALOG_COMPILER_GLOBAL_ACCENT_COLOR_NAME: AccentColor\n")
	b.WriteString("        ENABLE_PREVIEWS: YES\n")
	b.WriteString("        SWIFT_EMIT_LOC_STRINGS: YES\n")
	b.WriteString("        COMBINE_HIDPI_IMAGES: YES\n")
	b.WriteString("        LD_RUNPATH_SEARCH_PATHS:\n")
	b.WriteString("          - \"$(inherited)\"\n")
	b.WriteString("          - \"@executable_path/../Frameworks\"\n")
	b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
	b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")

	if plan != nil {
		for _, perm := range plan.Permissions {
			fmt.Fprintf(&b, "        INFOPLIST_KEY_%s: %s\n", perm.Key, xcodeYAMLQuote(perm.Description))
		}
	}

	appearanceInfoPlistMacOS(&b, appName, appName, plan)

	// Entitlements
	b.WriteString("    entitlements:\n")
	fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", appName, appName)
	b.WriteString("      properties: {}\n")

	// Dependencies: embed extension targets
	if hasExtensions {
		b.WriteString("    dependencies:\n")
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			fmt.Fprintf(&b, "      - target: %s\n", name)
			b.WriteString("        embed: true\n")
		}
	}

	// Extension targets (widget, share, notification_service supported on macOS)
	if plan != nil {
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			kind := ext.Kind
			kindForBundleID := strings.ReplaceAll(kind, "_", "")
			if kindForBundleID == "" {
				kindForBundleID = strings.ToLower(name)
			}
			extBundleID := fmt.Sprintf("%s.%s", bundleID, kindForBundleID)
			sourcePath := fmt.Sprintf("Targets/%s", name)

			b.WriteString("\n")
			fmt.Fprintf(&b, "  %s:\n", name)
			fmt.Fprintf(&b, "    type: %s\n", xcodegenTargetType(kind))
			b.WriteString("    platform: macOS\n")
			b.WriteString("    sources:\n")
			fmt.Fprintf(&b, "      - path: %s\n", sourcePath)
			b.WriteString("        type: syncedFolder\n")
			b.WriteString("      - path: Shared\n")
			b.WriteString("        type: syncedFolder\n")
			b.WriteString("        optional: true\n")
			b.WriteString("    settings:\n")
			b.WriteString("      base:\n")
			fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", extBundleID)
			b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
			b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
			b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
			b.WriteString("        SKIP_INSTALL: YES\n")
			b.WriteString("        DEAD_CODE_STRIPPING: NO\n")
			b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
			b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
			b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
			b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")

			for k, v := range ext.Settings {
				fmt.Fprintf(&b, "        %s: %s\n", k, xcodeYAMLQuote(v))
			}

			infoPlist := mergeInfoPlistDefaults(kind, ext.InfoPlist)
			if len(infoPlist) > 0 {
				b.WriteString("    info:\n")
				fmt.Fprintf(&b, "      path: %s/Info.plist\n", sourcePath)
				b.WriteString("      properties:\n")
				writeXcodeYAMLMap(&b, infoPlist, 8)
			}

			entitlements := mergeEntitlementDefaults(kind, ext.Entitlements, bundleID)
			if len(entitlements) > 0 {
				b.WriteString("    entitlements:\n")
				fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", sourcePath, name)
				b.WriteString("      properties:\n")
				writeXcodeYAMLMap(&b, entitlements, 8)
			}
		}
	}

	// Scheme
	b.WriteString("\nschemes:\n")
	fmt.Fprintf(&b, "  %s:\n", appName)
	b.WriteString("    build:\n")
	b.WriteString("      targets:\n")
	fmt.Fprintf(&b, "        %s: all\n", appName)
	if hasExtensions {
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			fmt.Fprintf(&b, "        %s: all\n", name)
		}
	}
	b.WriteString("    run:\n")
	fmt.Fprintf(&b, "      executable: %s\n", appName)

	return b.String()
}

// generateIOSProjectYAML produces the iOS project.yml (existing behavior).
func generateIOSProjectYAML(appName string, plan *PlannerResult) string {
	var b strings.Builder

	bundleID := fmt.Sprintf("%s.%s", bundleIDPrefix(), strings.ToLower(appName))
	hasExtensions := plan != nil && len(plan.Extensions) > 0
	// Check if any extension needs data sharing (app groups)
	needsAppGroups := false
	if hasExtensions {
		for _, ext := range plan.Extensions {
			switch ext.Kind {
			case "widget", "live_activity", "share":
				needsAppGroups = true
			}
			if needsAppGroups {
				break
			}
		}
	}

	fmt.Fprintf(&b, "name: %s\n", appName)
	b.WriteString("options:\n")
	fmt.Fprintf(&b, "  bundleIdPrefix: %s\n", bundleIDPrefix())
	b.WriteString("  deploymentTarget:\n")
	b.WriteString("    iOS: \"26.0\"\n")
	b.WriteString("  xcodeVersion: \"16.0\"\n")
	b.WriteString("  createIntermediateGroups: true\n")
	b.WriteString("  generateEmptyDirectories: true\n")
	b.WriteString("  useBaseInternationalization: false\n")

	if plan != nil && len(plan.Localizations) > 0 {
		b.WriteString("  knownRegions:\n")
		for _, lang := range plan.Localizations {
			fmt.Fprintf(&b, "    - %s\n", lang)
		}
	}
	b.WriteString("\n")

	// Targets
	b.WriteString("targets:\n")

	// Main app target
	fmt.Fprintf(&b, "  %s:\n", appName)
	b.WriteString("    type: application\n")
	writeIOSDestinationSettings(&b, plan.GetDeviceFamily())
	b.WriteString("    sources:\n")
	fmt.Fprintf(&b, "      - path: %s\n", appName)
	b.WriteString("        type: syncedFolder\n")
	// Include Shared/ directory for types shared between main app and extensions
	if hasExtensions {
		b.WriteString("      - path: Shared\n")
		b.WriteString("        type: syncedFolder\n")
		b.WriteString("        optional: true\n")
	}

	// Settings
	b.WriteString("    settings:\n")
	b.WriteString("      base:\n")
	b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
	fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", bundleID)
	b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
	b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
	b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
	b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
	b.WriteString("        INFOPLIST_KEY_UIApplicationSceneManifest_Generation: YES\n")
	b.WriteString("        INFOPLIST_KEY_UIApplicationSupportsIndirectInputEvents: YES\n")
	b.WriteString("        INFOPLIST_KEY_UILaunchScreen_Generation: YES\n")
	deviceFamilyBuildSettings(&b, plan.GetDeviceFamily())
	b.WriteString("        ASSETCATALOG_COMPILER_APPICON_NAME: AppIcon\n")
	b.WriteString("        ASSETCATALOG_COMPILER_GLOBAL_ACCENT_COLOR_NAME: AccentColor\n")
	b.WriteString("        ENABLE_PREVIEWS: YES\n")
	b.WriteString("        SWIFT_EMIT_LOC_STRINGS: YES\n")
	b.WriteString("        LD_RUNPATH_SEARCH_PATHS:\n")
	b.WriteString("          - \"$(inherited)\"\n")
	b.WriteString("          - \"@executable_path/Frameworks\"\n")
	b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
	b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")

	appearanceBuildSettings(&b, plan, PlatformIOS)

	if plan != nil {
		// Permissions as INFOPLIST_KEY_* build settings
		for _, perm := range plan.Permissions {
			fmt.Fprintf(&b, "        INFOPLIST_KEY_%s: %s\n", perm.Key, xcodeYAMLQuote(perm.Description))
		}
	}

	// Main app entitlements
	b.WriteString("    entitlements:\n")
	fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", appName, appName)
	if needsAppGroups {
		b.WriteString("      properties:\n")
		fmt.Fprintf(&b, "        com.apple.security.application-groups:\n")
		fmt.Fprintf(&b, "          - group.%s\n", bundleID)
	} else {
		b.WriteString("      properties: {}\n")
	}

	// Main app Info.plist — for keys that can't be expressed as INFOPLIST_KEY_* build settings
	mainInfoPlist := make(map[string]any)
	if plan != nil {
		for _, ext := range plan.Extensions {
			if ext.Kind == "live_activity" {
				mainInfoPlist["NSSupportsLiveActivities"] = true
				break
			}
		}
	}
	if len(mainInfoPlist) > 0 {
		b.WriteString("    info:\n")
		fmt.Fprintf(&b, "      path: %s/Info.plist\n", appName)
		b.WriteString("      properties:\n")
		writeXcodeYAMLMap(&b, mainInfoPlist, 8)
	}

	// Dependencies: embed extension targets
	if hasExtensions {
		b.WriteString("    dependencies:\n")
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			fmt.Fprintf(&b, "      - target: %s\n", name)
			b.WriteString("        embed: true\n")
		}
	}

	// Extension targets
	if plan != nil {
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			kind := ext.Kind
			// Bundle IDs cannot contain underscores — remove them.
			// If kind is empty, derive a bundle-safe suffix from the target name.
			kindForBundleID := strings.ReplaceAll(kind, "_", "")
			if kindForBundleID == "" {
				kindForBundleID = strings.ToLower(name)
			}
			extBundleID := fmt.Sprintf("%s.%s", bundleID, kindForBundleID)
			sourcePath := fmt.Sprintf("Targets/%s", name)

			b.WriteString("\n")
			fmt.Fprintf(&b, "  %s:\n", name)
			fmt.Fprintf(&b, "    type: %s\n", xcodegenTargetType(kind))
			b.WriteString("    platform: iOS\n")
			b.WriteString("    sources:\n")
			fmt.Fprintf(&b, "      - path: %s\n", sourcePath)
			b.WriteString("        type: syncedFolder\n")
			// Include Shared/ directory so extension can access shared types (e.g. ActivityAttributes)
			b.WriteString("      - path: Shared\n")
			b.WriteString("        type: syncedFolder\n")
			b.WriteString("        optional: true\n")
			b.WriteString("    settings:\n")
			b.WriteString("      base:\n")
			fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", extBundleID)
			b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
			b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
			b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
			b.WriteString("        SKIP_INSTALL: YES\n")
			b.WriteString("        DEAD_CODE_STRIPPING: NO\n")
			b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
			b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
			b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
			b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")

			// Extra build settings from plan
			for k, v := range ext.Settings {
				fmt.Fprintf(&b, "        %s: %s\n", k, xcodeYAMLQuote(v))
			}

			// Info.plist — merge defaults with plan-specified values
			infoPlist := mergeInfoPlistDefaults(kind, ext.InfoPlist)
			if len(infoPlist) > 0 {
				b.WriteString("    info:\n")
				fmt.Fprintf(&b, "      path: %s/Info.plist\n", sourcePath)
				b.WriteString("      properties:\n")
				writeXcodeYAMLMap(&b, infoPlist, 8)
			}

			// Entitlements — merge defaults with plan-specified values
			entitlements := mergeEntitlementDefaults(kind, ext.Entitlements, bundleID)
			if len(entitlements) > 0 {
				b.WriteString("    entitlements:\n")
				fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", sourcePath, name)
				b.WriteString("      properties:\n")
				writeXcodeYAMLMap(&b, entitlements, 8)
			}
		}
	}

	// Explicit scheme — prevents Xcode from trying to debug/show extension widgets on launch
	if hasExtensions {
		b.WriteString("\nschemes:\n")
		fmt.Fprintf(&b, "  %s:\n", appName)
		b.WriteString("    build:\n")
		b.WriteString("      targets:\n")
		fmt.Fprintf(&b, "        %s: all\n", appName)
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			fmt.Fprintf(&b, "        %s: all\n", name)
		}
		b.WriteString("    run:\n")
		fmt.Fprintf(&b, "      executable: %s\n", appName)
	}

	return b.String()
}

// generateTvOSProjectYAML produces the tvOS project.yml for XcodeGen.
// tvOS apps use TARGETED_DEVICE_FAMILY "3", no orientation settings,
// no launch screen, and focus-based navigation.
func generateTvOSProjectYAML(appName string, plan *PlannerResult) string {
	var b strings.Builder

	bundleID := fmt.Sprintf("%s.%s", bundleIDPrefix(), strings.ToLower(appName))
	hasExtensions := plan != nil && len(plan.Extensions) > 0

	fmt.Fprintf(&b, "name: %s\n", appName)
	b.WriteString("options:\n")
	fmt.Fprintf(&b, "  bundleIdPrefix: %s\n", bundleIDPrefix())
	b.WriteString("  deploymentTarget:\n")
	b.WriteString("    tvOS: \"26.0\"\n")
	b.WriteString("  xcodeVersion: \"16.0\"\n")
	b.WriteString("  createIntermediateGroups: true\n")
	b.WriteString("  generateEmptyDirectories: true\n")
	b.WriteString("  useBaseInternationalization: false\n")

	if plan != nil && len(plan.Localizations) > 0 {
		b.WriteString("  knownRegions:\n")
		for _, lang := range plan.Localizations {
			fmt.Fprintf(&b, "    - %s\n", lang)
		}
	}
	b.WriteString("\n")

	b.WriteString("targets:\n")

	// Main app target
	fmt.Fprintf(&b, "  %s:\n", appName)
	b.WriteString("    type: application\n")
	b.WriteString("    platform: tvOS\n")
	b.WriteString("    supportedDestinations:\n")
	b.WriteString("      - tvOS\n")
	b.WriteString("    sources:\n")
	fmt.Fprintf(&b, "      - path: %s\n", appName)
	b.WriteString("        type: syncedFolder\n")
	if hasExtensions {
		b.WriteString("      - path: Shared\n")
		b.WriteString("        type: syncedFolder\n")
		b.WriteString("        optional: true\n")
	}

	// Settings
	b.WriteString("    settings:\n")
	b.WriteString("      base:\n")
	b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
	fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", bundleID)
	b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
	b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
	b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
	b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
	b.WriteString("        TARGETED_DEVICE_FAMILY: \"3\"\n")
	b.WriteString("        ASSETCATALOG_COMPILER_APPICON_NAME: AppIcon\n")
	b.WriteString("        ASSETCATALOG_COMPILER_GLOBAL_ACCENT_COLOR_NAME: AccentColor\n")
	b.WriteString("        ENABLE_PREVIEWS: YES\n")
	b.WriteString("        SWIFT_EMIT_LOC_STRINGS: YES\n")
	b.WriteString("        LD_RUNPATH_SEARCH_PATHS:\n")
	b.WriteString("          - \"$(inherited)\"\n")
	b.WriteString("          - \"@executable_path/Frameworks\"\n")
	b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
	b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")
	appearanceBuildSettings(&b, plan, PlatformTvOS)

	if plan != nil {
		for _, perm := range plan.Permissions {
			fmt.Fprintf(&b, "        INFOPLIST_KEY_%s: %s\n", perm.Key, xcodeYAMLQuote(perm.Description))
		}
	}

	// Entitlements
	b.WriteString("    entitlements:\n")
	fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", appName, appName)
	b.WriteString("      properties: {}\n")

	// Dependencies: embed extension targets
	if hasExtensions {
		b.WriteString("    dependencies:\n")
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			fmt.Fprintf(&b, "      - target: %s\n", name)
			b.WriteString("        embed: true\n")
		}
	}

	// Extension targets (only tv-top-shelf supported on tvOS)
	if plan != nil {
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			kind := ext.Kind
			kindForBundleID := strings.ReplaceAll(kind, "_", "")
			if kindForBundleID == "" {
				kindForBundleID = strings.ToLower(name)
			}
			extBundleID := fmt.Sprintf("%s.%s", bundleID, kindForBundleID)
			sourcePath := fmt.Sprintf("Targets/%s", name)

			b.WriteString("\n")
			fmt.Fprintf(&b, "  %s:\n", name)
			fmt.Fprintf(&b, "    type: %s\n", xcodegenTargetType(kind))
			b.WriteString("    platform: tvOS\n")
			b.WriteString("    sources:\n")
			fmt.Fprintf(&b, "      - path: %s\n", sourcePath)
			b.WriteString("        type: syncedFolder\n")
			b.WriteString("      - path: Shared\n")
			b.WriteString("        type: syncedFolder\n")
			b.WriteString("        optional: true\n")
			b.WriteString("    settings:\n")
			b.WriteString("      base:\n")
			fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", extBundleID)
			b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
			b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
			b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
			b.WriteString("        SKIP_INSTALL: YES\n")
			b.WriteString("        DEAD_CODE_STRIPPING: NO\n")
			b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
			b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
			b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
			b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")

			for k, v := range ext.Settings {
				fmt.Fprintf(&b, "        %s: %s\n", k, xcodeYAMLQuote(v))
			}

			infoPlist := mergeInfoPlistDefaults(kind, ext.InfoPlist)
			if len(infoPlist) > 0 {
				b.WriteString("    info:\n")
				fmt.Fprintf(&b, "      path: %s/Info.plist\n", sourcePath)
				b.WriteString("      properties:\n")
				writeXcodeYAMLMap(&b, infoPlist, 8)
			}

			entitlements := mergeEntitlementDefaults(kind, ext.Entitlements, bundleID)
			if len(entitlements) > 0 {
				b.WriteString("    entitlements:\n")
				fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", sourcePath, name)
				b.WriteString("      properties:\n")
				writeXcodeYAMLMap(&b, entitlements, 8)
			}
		}
	}

	// Explicit scheme
	if hasExtensions {
		b.WriteString("\nschemes:\n")
		fmt.Fprintf(&b, "  %s:\n", appName)
		b.WriteString("    build:\n")
		b.WriteString("      targets:\n")
		fmt.Fprintf(&b, "        %s: all\n", appName)
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			fmt.Fprintf(&b, "        %s: all\n", name)
		}
		b.WriteString("    run:\n")
		fmt.Fprintf(&b, "      executable: %s\n", appName)
	}

	return b.String()
}

// generateVisionOSProjectYAML produces the visionOS project.yml.
func generateVisionOSProjectYAML(appName string, plan *PlannerResult) string {
	var b strings.Builder

	bundleID := fmt.Sprintf("%s.%s", bundleIDPrefix(), strings.ToLower(appName))
	hasExtensions := plan != nil && len(plan.Extensions) > 0

	fmt.Fprintf(&b, "name: %s\n", appName)
	b.WriteString("options:\n")
	fmt.Fprintf(&b, "  bundleIdPrefix: %s\n", bundleIDPrefix())
	b.WriteString("  deploymentTarget:\n")
	b.WriteString("    visionOS: \"26.0\"\n")
	b.WriteString("  xcodeVersion: \"16.0\"\n")
	b.WriteString("  createIntermediateGroups: true\n")
	b.WriteString("  generateEmptyDirectories: true\n")
	b.WriteString("  useBaseInternationalization: false\n")

	if plan != nil && len(plan.Localizations) > 0 {
		b.WriteString("  knownRegions:\n")
		for _, lang := range plan.Localizations {
			fmt.Fprintf(&b, "    - %s\n", lang)
		}
	}
	b.WriteString("\n")

	b.WriteString("targets:\n")

	// Main app target
	fmt.Fprintf(&b, "  %s:\n", appName)
	b.WriteString("    type: application\n")
	b.WriteString("    platform: visionOS\n")
	b.WriteString("    supportedDestinations:\n")
	b.WriteString("      - visionOS\n")
	b.WriteString("    sources:\n")
	fmt.Fprintf(&b, "      - path: %s\n", appName)
	b.WriteString("        type: syncedFolder\n")
	if hasExtensions {
		b.WriteString("      - path: Shared\n")
		b.WriteString("        type: syncedFolder\n")
		b.WriteString("        optional: true\n")
	}

	// Settings
	b.WriteString("    settings:\n")
	b.WriteString("      base:\n")
	b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
	fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", bundleID)
	b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
	b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
	b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
	b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
	b.WriteString("        TARGETED_DEVICE_FAMILY: \"7\"\n")
	b.WriteString("        ASSETCATALOG_COMPILER_APPICON_NAME: AppIcon\n")
	b.WriteString("        ASSETCATALOG_COMPILER_GLOBAL_ACCENT_COLOR_NAME: AccentColor\n")
	b.WriteString("        ENABLE_PREVIEWS: YES\n")
	b.WriteString("        SWIFT_EMIT_LOC_STRINGS: YES\n")
	b.WriteString("        LD_RUNPATH_SEARCH_PATHS:\n")
	b.WriteString("          - \"$(inherited)\"\n")
	b.WriteString("          - \"@executable_path/Frameworks\"\n")
	b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
	b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")
	appearanceBuildSettings(&b, plan, PlatformVisionOS)

	if plan != nil {
		for _, perm := range plan.Permissions {
			fmt.Fprintf(&b, "        INFOPLIST_KEY_%s: %s\n", perm.Key, xcodeYAMLQuote(perm.Description))
		}
	}

	// Entitlements
	b.WriteString("    entitlements:\n")
	fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", appName, appName)
	b.WriteString("      properties: {}\n")

	// Dependencies: embed extension targets
	if hasExtensions {
		b.WriteString("    dependencies:\n")
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			fmt.Fprintf(&b, "      - target: %s\n", name)
			b.WriteString("        embed: true\n")
		}
	}

	// Extension targets (only widget supported on visionOS)
	if plan != nil {
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			kind := ext.Kind
			kindForBundleID := strings.ReplaceAll(kind, "_", "")
			if kindForBundleID == "" {
				kindForBundleID = strings.ToLower(name)
			}
			extBundleID := fmt.Sprintf("%s.%s", bundleID, kindForBundleID)
			sourcePath := fmt.Sprintf("Targets/%s", name)

			b.WriteString("\n")
			fmt.Fprintf(&b, "  %s:\n", name)
			fmt.Fprintf(&b, "    type: %s\n", xcodegenTargetType(kind))
			b.WriteString("    platform: visionOS\n")
			b.WriteString("    sources:\n")
			fmt.Fprintf(&b, "      - path: %s\n", sourcePath)
			b.WriteString("        type: syncedFolder\n")
			b.WriteString("      - path: Shared\n")
			b.WriteString("        type: syncedFolder\n")
			b.WriteString("        optional: true\n")
			b.WriteString("    settings:\n")
			b.WriteString("      base:\n")
			fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", extBundleID)
			b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
			b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
			b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
			b.WriteString("        SKIP_INSTALL: YES\n")
			b.WriteString("        DEAD_CODE_STRIPPING: NO\n")
			b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
			b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
			b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
			b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")

			for k, v := range ext.Settings {
				fmt.Fprintf(&b, "        %s: %s\n", k, xcodeYAMLQuote(v))
			}

			infoPlist := mergeInfoPlistDefaults(kind, ext.InfoPlist)
			if len(infoPlist) > 0 {
				b.WriteString("    info:\n")
				fmt.Fprintf(&b, "      path: %s/Info.plist\n", sourcePath)
				b.WriteString("      properties:\n")
				writeXcodeYAMLMap(&b, infoPlist, 8)
			}

			entitlements := mergeEntitlementDefaults(kind, ext.Entitlements, bundleID)
			if len(entitlements) > 0 {
				b.WriteString("    entitlements:\n")
				fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", sourcePath, name)
				b.WriteString("      properties:\n")
				writeXcodeYAMLMap(&b, entitlements, 8)
			}
		}
	}

	// Explicit scheme
	if hasExtensions {
		b.WriteString("\nschemes:\n")
		fmt.Fprintf(&b, "  %s:\n", appName)
		b.WriteString("    build:\n")
		b.WriteString("      targets:\n")
		fmt.Fprintf(&b, "        %s: all\n", appName)
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			fmt.Fprintf(&b, "        %s: all\n", name)
		}
		b.WriteString("    run:\n")
		fmt.Fprintf(&b, "      executable: %s\n", appName)
	}

	return b.String()
}

// writeWatchOSBuildSettings writes watchOS-specific build settings.
func writeWatchOSBuildSettings(b *strings.Builder) {
	b.WriteString("        ASSETCATALOG_COMPILER_APPICON_NAME: AppIcon\n")
	b.WriteString("        ASSETCATALOG_COMPILER_GLOBAL_ACCENT_COLOR_NAME: AccentColor\n")
	b.WriteString("        ENABLE_PREVIEWS: YES\n")
	b.WriteString("        SWIFT_EMIT_LOC_STRINGS: YES\n")
	b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
	b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")
}

func watchAppTargetName(appName string) string {
	return appName + "Watch"
}

func watchExtensionTargetName(appName string) string {
	return appName + "WatchExtension"
}

func writeSyncedSourceEntry(b *strings.Builder, path string, excludes []string, optional bool) {
	fmt.Fprintf(b, "      - path: %s\n", path)
	b.WriteString("        type: syncedFolder\n")
	if optional {
		b.WriteString("        optional: true\n")
	}
	if len(excludes) == 0 {
		return
	}
	b.WriteString("        excludes:\n")
	for _, pattern := range excludes {
		fmt.Fprintf(b, "          - %s\n", xcodeYAMLQuote(pattern))
	}
}

func writeIntrinsicWatchExtensionTargetYAML(b *strings.Builder, targetName, sourcePath, extBundleID, watchAppBundleID string, includeShared bool) {
	b.WriteString("\n")
	fmt.Fprintf(b, "  %s:\n", targetName)
	b.WriteString("    type: watchkit2-extension\n")
	b.WriteString("    platform: watchOS\n")
	b.WriteString("    sources:\n")
	writeSyncedSourceEntry(b, sourcePath, []string{"*.plist", "*.entitlements"}, false)
	if includeShared {
		writeSyncedSourceEntry(b, "Shared", nil, true)
	}

	b.WriteString("    settings:\n")
	b.WriteString("      base:\n")
	fmt.Fprintf(b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", extBundleID)
	b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
	b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
	b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
	b.WriteString("        SKIP_INSTALL: YES\n")
	b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
	b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
	b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
	b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")

	b.WriteString("    entitlements:\n")
	fmt.Fprintf(b, "      path: %s/%s.entitlements\n", sourcePath, targetName)
	b.WriteString("      properties: {}\n")

	b.WriteString("    info:\n")
	fmt.Fprintf(b, "      path: %s/WatchExtension-Info.plist\n", sourcePath)
	b.WriteString("      properties:\n")
	b.WriteString("        NSExtension:\n")
	b.WriteString("          NSExtensionPointIdentifier: com.apple.watchkit\n")
	b.WriteString("          NSExtensionAttributes:\n")
	fmt.Fprintf(b, "            WKAppBundleIdentifier: %s\n", xcodeYAMLQuote(watchAppBundleID))
}

// generateWatchOnlyYAML produces project.yml for a standalone watchOS app.
func generateWatchOnlyYAML(appName string, plan *PlannerResult) string {
	var b strings.Builder

	bundleID := fmt.Sprintf("%s.%s", bundleIDPrefix(), strings.ToLower(appName))
	watchAppName := watchAppTargetName(appName)
	watchBundleID := bundleID + ".watchkitapp"
	watchExtName := watchExtensionTargetName(appName)
	watchExtBundleID := watchBundleID + ".watchkitextension"
	hasExtensions := plan != nil && len(plan.Extensions) > 0

	fmt.Fprintf(&b, "name: %s\n", appName)
	b.WriteString("options:\n")
	fmt.Fprintf(&b, "  bundleIdPrefix: %s\n", bundleIDPrefix())
	b.WriteString("  deploymentTarget:\n")
	b.WriteString("    watchOS: \"26.0\"\n")
	b.WriteString("  xcodeVersion: \"16.0\"\n")
	b.WriteString("  createIntermediateGroups: true\n")
	b.WriteString("  generateEmptyDirectories: true\n")
	b.WriteString("  useBaseInternationalization: false\n")

	if plan != nil && len(plan.Localizations) > 0 {
		b.WriteString("  knownRegions:\n")
		for _, lang := range plan.Localizations {
			fmt.Fprintf(&b, "    - %s\n", lang)
		}
	}
	b.WriteString("\n")

	b.WriteString("targets:\n")

	// Watch container target
	fmt.Fprintf(&b, "  %s:\n", appName)
	b.WriteString("    type: application.watchapp2-container\n")
	b.WriteString("    platform: watchOS\n")
	b.WriteString("    sources:\n")
	writeSyncedSourceEntry(&b, appName, []string{"**/*.swift", "*.plist", "*.entitlements"}, false)

	b.WriteString("    settings:\n")
	b.WriteString("      base:\n")
	b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
	fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", bundleID)
	b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
	b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
	b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
	b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")

	if plan != nil {
		for _, perm := range plan.Permissions {
			fmt.Fprintf(&b, "        INFOPLIST_KEY_%s: %s\n", perm.Key, xcodeYAMLQuote(perm.Description))
		}
	}

	// Entitlements
	b.WriteString("    entitlements:\n")
	fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", appName, appName)
	b.WriteString("      properties: {}\n")

	// Dependencies: embed watch app (and any optional extension targets)
	b.WriteString("    dependencies:\n")
	fmt.Fprintf(&b, "      - target: %s\n", watchAppName)
	b.WriteString("        embed: true\n")
	if hasExtensions {
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			fmt.Fprintf(&b, "      - target: %s\n", name)
			b.WriteString("        embed: true\n")
		}
	}

	// Watch app target (wrapper app bundle)
	b.WriteString("\n")
	fmt.Fprintf(&b, "  %s:\n", watchAppName)
	b.WriteString("    type: application.watchapp2\n")
	b.WriteString("    platform: watchOS\n")
	b.WriteString("    sources:\n")
	writeSyncedSourceEntry(&b, appName, []string{"**/*.swift", "*.plist", "*.entitlements"}, false)
	b.WriteString("    settings:\n")
	b.WriteString("      base:\n")
	b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
	fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", watchBundleID)
	b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
	b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
	b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
	b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
	writeWatchOSBuildSettings(&b)

	b.WriteString("    entitlements:\n")
	fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", appName, watchAppName)
	b.WriteString("      properties: {}\n")

	b.WriteString("    info:\n")
	fmt.Fprintf(&b, "      path: %s/WatchApp-Info.plist\n", appName)
	b.WriteString("      properties:\n")
	b.WriteString("        WKWatchOnly: true\n")
	b.WriteString("        WKRunsIndependentlyOfCompanionApp: true\n")

	b.WriteString("    dependencies:\n")
	fmt.Fprintf(&b, "      - target: %s\n", watchExtName)
	b.WriteString("        embed: true\n")

	// Intrinsic watch runtime extension target
	writeIntrinsicWatchExtensionTargetYAML(&b, watchExtName, appName, watchExtBundleID, watchBundleID, hasExtensions)

	// Extension targets (only widget is supported on watchOS)
	if plan != nil {
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			kind := ext.Kind
			kindForBundleID := strings.ReplaceAll(kind, "_", "")
			if kindForBundleID == "" {
				kindForBundleID = strings.ToLower(name)
			}
			extBundleID := fmt.Sprintf("%s.%s", bundleID, kindForBundleID)
			sourcePath := fmt.Sprintf("Targets/%s", name)

			b.WriteString("\n")
			fmt.Fprintf(&b, "  %s:\n", name)
			fmt.Fprintf(&b, "    type: %s\n", xcodegenTargetType(kind))
			b.WriteString("    platform: watchOS\n")
			b.WriteString("    sources:\n")
			writeSyncedSourceEntry(&b, sourcePath, nil, false)
			writeSyncedSourceEntry(&b, "Shared", nil, true)
			b.WriteString("    settings:\n")
			b.WriteString("      base:\n")
			fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", extBundleID)
			b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
			b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
			b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
			b.WriteString("        SKIP_INSTALL: YES\n")
			b.WriteString("        DEAD_CODE_STRIPPING: NO\n")
			b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
			b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
			b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
			b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")

			for k, v := range ext.Settings {
				fmt.Fprintf(&b, "        %s: %s\n", k, xcodeYAMLQuote(v))
			}

			infoPlist := mergeInfoPlistDefaults(kind, ext.InfoPlist)
			if len(infoPlist) > 0 {
				b.WriteString("    info:\n")
				fmt.Fprintf(&b, "      path: %s/Info.plist\n", sourcePath)
				b.WriteString("      properties:\n")
				writeXcodeYAMLMap(&b, infoPlist, 8)
			}

			entitlements := mergeEntitlementDefaults(kind, ext.Entitlements, bundleID)
			if len(entitlements) > 0 {
				b.WriteString("    entitlements:\n")
				fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", sourcePath, name)
				b.WriteString("      properties:\n")
				writeXcodeYAMLMap(&b, entitlements, 8)
			}
		}
	}

	b.WriteString("\nschemes:\n")
	fmt.Fprintf(&b, "  %s:\n", appName)
	b.WriteString("    build:\n")
	b.WriteString("      targets:\n")
	fmt.Fprintf(&b, "        %s: all\n", appName)
	fmt.Fprintf(&b, "        %s: all\n", watchAppName)
	fmt.Fprintf(&b, "        %s: all\n", watchExtName)
	if plan != nil {
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			fmt.Fprintf(&b, "        %s: all\n", name)
		}
	}
	b.WriteString("    run:\n")
	fmt.Fprintf(&b, "      executable: %s\n", appName)

	return b.String()
}

// generatePairedYAML produces project.yml for a paired iOS+watchOS app.
func generatePairedYAML(appName string, plan *PlannerResult) string {
	var b strings.Builder

	bundleID := fmt.Sprintf("%s.%s", bundleIDPrefix(), strings.ToLower(appName))
	watchAppName := watchAppTargetName(appName)
	watchBundleID := bundleID + ".watchkitapp"
	watchExtName := watchExtensionTargetName(appName)
	watchExtBundleID := watchBundleID + ".watchkitextension"
	hasExtensions := plan != nil && len(plan.Extensions) > 0

	fmt.Fprintf(&b, "name: %s\n", appName)
	b.WriteString("options:\n")
	fmt.Fprintf(&b, "  bundleIdPrefix: %s\n", bundleIDPrefix())
	b.WriteString("  deploymentTarget:\n")
	b.WriteString("    iOS: \"26.0\"\n")
	b.WriteString("    watchOS: \"26.0\"\n")
	b.WriteString("  xcodeVersion: \"16.0\"\n")
	b.WriteString("  createIntermediateGroups: true\n")
	b.WriteString("  generateEmptyDirectories: true\n")
	b.WriteString("  useBaseInternationalization: false\n")

	if plan != nil && len(plan.Localizations) > 0 {
		b.WriteString("  knownRegions:\n")
		for _, lang := range plan.Localizations {
			fmt.Fprintf(&b, "    - %s\n", lang)
		}
	}
	b.WriteString("\n")

	b.WriteString("targets:\n")

	// iOS parent target
	fmt.Fprintf(&b, "  %s:\n", appName)
	b.WriteString("    type: application\n")
	b.WriteString("    platform: iOS\n")
	b.WriteString("    supportedDestinations:\n")
	b.WriteString("      - iOS\n")
	b.WriteString("    sources:\n")
	fmt.Fprintf(&b, "      - path: %s\n", appName)
	b.WriteString("        type: syncedFolder\n")
	if hasExtensions {
		b.WriteString("      - path: Shared\n")
		b.WriteString("        type: syncedFolder\n")
		b.WriteString("        optional: true\n")
	}

	b.WriteString("    settings:\n")
	b.WriteString("      base:\n")
	b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
	fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", bundleID)
	b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
	b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
	b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
	b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
	b.WriteString("        INFOPLIST_KEY_UIApplicationSceneManifest_Generation: YES\n")
	b.WriteString("        INFOPLIST_KEY_UIApplicationSupportsIndirectInputEvents: YES\n")
	b.WriteString("        INFOPLIST_KEY_UILaunchScreen_Generation: YES\n")
	b.WriteString("        TARGETED_DEVICE_FAMILY: \"1\"\n")
	b.WriteString("        INFOPLIST_KEY_UISupportedInterfaceOrientations_iPhone: UIInterfaceOrientationPortrait\n")
	b.WriteString("        ASSETCATALOG_COMPILER_APPICON_NAME: AppIcon\n")
	b.WriteString("        ASSETCATALOG_COMPILER_GLOBAL_ACCENT_COLOR_NAME: AccentColor\n")
	b.WriteString("        ENABLE_PREVIEWS: YES\n")
	b.WriteString("        SWIFT_EMIT_LOC_STRINGS: YES\n")
	b.WriteString("        LD_RUNPATH_SEARCH_PATHS:\n")
	b.WriteString("          - \"$(inherited)\"\n")
	b.WriteString("          - \"@executable_path/Frameworks\"\n")
	b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
	b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")

	if plan != nil {
		for _, perm := range plan.Permissions {
			fmt.Fprintf(&b, "        INFOPLIST_KEY_%s: %s\n", perm.Key, xcodeYAMLQuote(perm.Description))
		}
	}

	b.WriteString("    entitlements:\n")
	fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", appName, appName)
	b.WriteString("      properties: {}\n")

	// iOS target depends on watch target
	b.WriteString("    dependencies:\n")
	fmt.Fprintf(&b, "      - target: %s\n", watchAppName)
	b.WriteString("        embed: true\n")
	if hasExtensions {
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			fmt.Fprintf(&b, "      - target: %s\n", name)
			b.WriteString("        embed: true\n")
		}
	}

	// Watch target
	b.WriteString("\n")
	fmt.Fprintf(&b, "  %s:\n", watchAppName)
	b.WriteString("    type: application.watchapp2\n")
	b.WriteString("    platform: watchOS\n")
	b.WriteString("    sources:\n")
	writeSyncedSourceEntry(&b, watchAppName, []string{"**/*.swift", "*.plist", "*.entitlements"}, false)

	b.WriteString("    settings:\n")
	b.WriteString("      base:\n")
	b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
	fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", watchBundleID)
	b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
	b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
	b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
	b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
	writeWatchOSBuildSettings(&b)

	b.WriteString("    entitlements:\n")
	fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", watchAppName, watchAppName)
	b.WriteString("      properties: {}\n")

	// Info.plist with WKRunsIndependentlyOfCompanionApp
	b.WriteString("    info:\n")
	fmt.Fprintf(&b, "      path: %s/Info.plist\n", watchAppName)
	b.WriteString("      properties:\n")
	fmt.Fprintf(&b, "        WKCompanionAppBundleIdentifier: %s\n", xcodeYAMLQuote(bundleID))
	b.WriteString("        WKRunsIndependentlyOfCompanionApp: true\n")
	b.WriteString("    dependencies:\n")
	fmt.Fprintf(&b, "      - target: %s\n", watchExtName)
	b.WriteString("        embed: true\n")

	// Intrinsic watch runtime extension target
	writeIntrinsicWatchExtensionTargetYAML(&b, watchExtName, watchAppName, watchExtBundleID, watchBundleID, hasExtensions)

	// Watch extension targets (widget only on watchOS)
	if plan != nil {
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			kind := ext.Kind
			kindForBundleID := strings.ReplaceAll(kind, "_", "")
			if kindForBundleID == "" {
				kindForBundleID = strings.ToLower(name)
			}
			extBundleID := fmt.Sprintf("%s.%s", watchBundleID, kindForBundleID)
			sourcePath := fmt.Sprintf("Targets/%s", name)

			b.WriteString("\n")
			fmt.Fprintf(&b, "  %s:\n", name)
			fmt.Fprintf(&b, "    type: %s\n", xcodegenTargetType(kind))
			b.WriteString("    platform: watchOS\n")
			b.WriteString("    sources:\n")
			writeSyncedSourceEntry(&b, sourcePath, nil, false)
			writeSyncedSourceEntry(&b, "Shared", nil, true)
			b.WriteString("    settings:\n")
			b.WriteString("      base:\n")
			fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", extBundleID)
			b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
			b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
			b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
			b.WriteString("        SKIP_INSTALL: YES\n")
			b.WriteString("        DEAD_CODE_STRIPPING: NO\n")
			b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
			b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
			b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
			b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")

			for k, v := range ext.Settings {
				fmt.Fprintf(&b, "        %s: %s\n", k, xcodeYAMLQuote(v))
			}

			infoPlist := mergeInfoPlistDefaults(kind, ext.InfoPlist)
			if len(infoPlist) > 0 {
				b.WriteString("    info:\n")
				fmt.Fprintf(&b, "      path: %s/Info.plist\n", sourcePath)
				b.WriteString("      properties:\n")
				writeXcodeYAMLMap(&b, infoPlist, 8)
			}

			entitlements := mergeEntitlementDefaults(kind, ext.Entitlements, watchBundleID)
			if len(entitlements) > 0 {
				b.WriteString("    entitlements:\n")
				fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", sourcePath, name)
				b.WriteString("      properties:\n")
				writeXcodeYAMLMap(&b, entitlements, 8)
			}
		}
	}

	// Scheme — run the iOS app by default
	b.WriteString("\nschemes:\n")
	fmt.Fprintf(&b, "  %s:\n", appName)
	b.WriteString("    build:\n")
	b.WriteString("      targets:\n")
	fmt.Fprintf(&b, "        %s: all\n", appName)
	fmt.Fprintf(&b, "        %s: all\n", watchAppName)
	fmt.Fprintf(&b, "        %s: all\n", watchExtName)
	if plan != nil {
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			fmt.Fprintf(&b, "        %s: all\n", name)
		}
	}
	b.WriteString("    run:\n")
	fmt.Fprintf(&b, "      executable: %s\n", appName)

	return b.String()
}

// extensionTargetName returns the Xcode target name for an extension.
func extensionTargetName(ext ExtensionPlan, appName string) string {
	if ext.Name != "" {
		return ext.Name
	}
	kindStr := ext.Kind
	if len(kindStr) > 0 {
		kindStr = strings.ToUpper(kindStr[:1]) + kindStr[1:]
	}
	return appName + strings.ReplaceAll(kindStr, "_", "")
}

// xcodegenTargetType maps a kind string to the XcodeGen target type.
func xcodegenTargetType(kind string) string {
	if kind == "app_clip" {
		return "app-clip"
	}
	return "app-extension"
}

// mergeInfoPlistDefaults fills in known-required Info.plist keys per extension kind.
// Plan-specified values take priority over defaults.
func mergeInfoPlistDefaults(kind string, planValues map[string]any) map[string]any {
	m := make(map[string]any)

	switch kind {
	case "widget":
		m["NSExtension"] = map[string]any{
			"NSExtensionPointIdentifier": "com.apple.widgetkit-extension",
		}
	case "live_activity":
		m["NSExtension"] = map[string]any{
			"NSExtensionPointIdentifier": "com.apple.widgetkit-extension",
		}
	case "share":
		m["NSExtension"] = map[string]any{
			"NSExtensionPointIdentifier": "com.apple.share-services",
			"NSExtensionPrincipalClass":  "$(PRODUCT_MODULE_NAME).ShareViewController",
			"NSExtensionAttributes": map[string]any{
				"NSExtensionActivationSupportsWebURLWithMaxCount": 1,
				"NSExtensionActivationSupportsText":               true,
			},
		}
	case "notification_service":
		m["NSExtension"] = map[string]any{
			"NSExtensionPointIdentifier": "com.apple.usernotifications.service",
			"NSExtensionPrincipalClass":  "$(PRODUCT_MODULE_NAME).NotificationService",
		}
	case "safari":
		m["NSExtension"] = map[string]any{
			"NSExtensionPointIdentifier": "com.apple.Safari.web-extension",
			"NSExtensionPrincipalClass":  "$(PRODUCT_MODULE_NAME).SafariWebExtensionHandler",
		}
	case "app_clip":
		m["NSAppClip"] = map[string]any{
			"NSAppClipRequestEphemeralUserNotification": false,
			"NSAppClipRequestLocationConfirmation":      false,
		}
	}

	// Plan values override defaults
	for k, v := range planValues {
		m[k] = v
	}

	return m
}

// mergeEntitlementDefaults fills in known-required entitlements per extension kind.
func mergeEntitlementDefaults(kind string, planValues map[string]any, mainBundleID string) map[string]any {
	m := make(map[string]any)

	switch kind {
	case "widget", "live_activity", "share":
		m["com.apple.security.application-groups"] = []any{"group." + mainBundleID}
	case "app_clip":
		m["com.apple.developer.parent-application-identifiers"] = []any{"$(AppIdentifierPrefix)" + mainBundleID}
		m["com.apple.developer.associated-domains"] = []any{"appclips:" + mainBundleID}
	}

	for k, v := range planValues {
		m[k] = v
	}

	return m
}

// xcodeYAMLQuote wraps a string in quotes if it contains special YAML characters.
func xcodeYAMLQuote(s string) string {
	if strings.ContainsAny(s, ":{}[]|>&*!%#@,") || strings.Contains(s, "  ") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

// writeXcodeYAMLMap writes a map[string]any as YAML properties at the given indent level.
func writeXcodeYAMLMap(b *strings.Builder, m map[string]any, indent int) {
	prefix := strings.Repeat(" ", indent)
	for k, v := range m {
		switch val := v.(type) {
		case bool:
			fmt.Fprintf(b, "%s%s: %t\n", prefix, k, val)
		case string:
			fmt.Fprintf(b, "%s%s: %s\n", prefix, k, xcodeYAMLQuote(val))
		case float64:
			if val == float64(int(val)) {
				fmt.Fprintf(b, "%s%s: %d\n", prefix, k, int(val))
			} else {
				fmt.Fprintf(b, "%s%s: %g\n", prefix, k, val)
			}
		case int:
			fmt.Fprintf(b, "%s%s: %d\n", prefix, k, val)
		case []any:
			fmt.Fprintf(b, "%s%s:\n", prefix, k)
			for _, item := range val {
				fmt.Fprintf(b, "%s  - %v\n", prefix, item)
			}
		case []string:
			fmt.Fprintf(b, "%s%s:\n", prefix, k)
			for _, item := range val {
				fmt.Fprintf(b, "%s  - %s\n", prefix, item)
			}
		case map[string]any:
			fmt.Fprintf(b, "%s%s:\n", prefix, k)
			writeXcodeYAMLMap(b, val, indent+2)
		default:
			fmt.Fprintf(b, "%s%s: %v\n", prefix, k, val)
		}
	}
}
