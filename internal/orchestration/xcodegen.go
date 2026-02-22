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
			// Bundle IDs cannot contain underscores — remove them
			extBundleID := fmt.Sprintf("%s.%s", bundleID, strings.ReplaceAll(kind, "_", ""))
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
