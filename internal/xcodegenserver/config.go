package xcodegenserver

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"unicode"
)

// ProjectConfig is the source of truth for Xcode project configuration.
// The MCP server reads/writes this as project_config.json, then regenerates project.yml from it.
type ProjectConfig struct {
	AppName           string            `json:"app_name"`
	BundleID          string            `json:"bundle_id"`
	Platform          string            `json:"platform,omitempty"`
	WatchProjectShape string            `json:"watch_project_shape,omitempty"`
	DeviceFamily      string            `json:"device_family,omitempty"`
	Permissions       []Permission      `json:"permissions,omitempty"`
	Extensions        []ExtensionPlan   `json:"extensions,omitempty"`
	Localizations     []string          `json:"localizations,omitempty"`
	Entitlements      []Entitlement     `json:"entitlements,omitempty"`
	BuildSettings     map[string]string `json:"build_settings,omitempty"`
	Packages          []PackageDep      `json:"packages,omitempty"`
}

// Permission describes a required iOS permission.
type Permission struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Framework   string `json:"framework"`
}

// Entitlement describes a single entitlement entry.
type Entitlement struct {
	Key    string `json:"key"`
	Value  any    `json:"value"`
	Target string `json:"target,omitempty"` // empty = main app
}

// ExtensionPlan describes a secondary Xcode target.
type ExtensionPlan struct {
	Kind         string            `json:"kind"`
	Name         string            `json:"name"`
	Purpose      string            `json:"purpose"`
	InfoPlist    map[string]any    `json:"info_plist,omitempty"`
	Entitlements map[string]any    `json:"entitlements,omitempty"`
	Settings     map[string]string `json:"settings,omitempty"`
}

// PackageDep describes an SPM package dependency for the Xcode project.
type PackageDep struct {
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	MinVersion string   `json:"min_version"`
	Products   []string `json:"products,omitempty"`
}

// loadConfig reads project_config.json from the working directory.
func loadConfig(workDir string) (*ProjectConfig, error) {
	path := filepath.Join(workDir, "project_config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read project_config.json: %w", err)
	}
	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse project_config.json: %w", err)
	}
	return &cfg, nil
}

// saveConfig writes project_config.json to the working directory.
func saveConfig(workDir string, cfg *ProjectConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(filepath.Join(workDir, "project_config.json"), data, 0o644)
}

// generateProjectYAML produces the full project.yml content from the config.
// This reuses the same logic as the orchestration package's xcodegen.go.
func generateProjectYAML(cfg *ProjectConfig) string {
	platform := cfg.Platform
	if platform == "" {
		platform = "ios"
	}

	if platform == "watchos" {
		shape := cfg.WatchProjectShape
		if shape == "" {
			shape = "watch_only"
		}
		if shape == "paired_ios_watch" {
			return generatePairedYAMLCfg(cfg)
		}
		return generateWatchOnlyYAMLCfg(cfg)
	}

	return generateIOSProjectYAMLCfg(cfg)
}

// generateIOSProjectYAMLCfg produces iOS project.yml from config (existing behavior).
func generateIOSProjectYAMLCfg(cfg *ProjectConfig) string {
	var b strings.Builder
	appName := cfg.AppName
	bundleID := cfg.BundleID
	hasExtensions := len(cfg.Extensions) > 0
	hasLocalizations := len(cfg.Localizations) > 1

	needsAppGroups := false
	if hasExtensions {
		for _, ext := range cfg.Extensions {
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

	if len(cfg.Localizations) > 0 {
		b.WriteString("  knownRegions:\n")
		for _, lang := range cfg.Localizations {
			fmt.Fprintf(&b, "    - %s\n", lang)
		}
	}
	b.WriteString("\n")

	writePackagesSectionCfg(&b, cfg.Packages)

	// Targets
	b.WriteString("targets:\n")

	// Main app target
	fmt.Fprintf(&b, "  %s:\n", appName)
	b.WriteString("    type: application\n")
	writeIOSDestinationSettingsCfg(&b, cfg.DeviceFamily)
	b.WriteString("    sources:\n")
	fmt.Fprintf(&b, "      - path: %s\n", appName)
	b.WriteString("        type: folder\n")
	if hasLocalizations {
		b.WriteString("        excludes:\n")
		b.WriteString("          - \"*.lproj\"\n")
		fmt.Fprintf(&b, "      - path: %s\n", appName)
		b.WriteString("        type: folder\n")
		b.WriteString("        includes:\n")
		b.WriteString("          - \"*.lproj\"\n")
		b.WriteString("        buildPhase: resources\n")
	}
	if hasExtensions {
		b.WriteString("      - path: Shared\n")
		b.WriteString("        type: folder\n")
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
	deviceFamilyBuildSettingsCfg(&b, cfg.DeviceFamily)
	b.WriteString("        ASSETCATALOG_COMPILER_APPICON_NAME: AppIcon\n")
	b.WriteString("        ASSETCATALOG_COMPILER_GLOBAL_ACCENT_COLOR_NAME: AccentColor\n")
	b.WriteString("        ENABLE_PREVIEWS: YES\n")
	b.WriteString("        SWIFT_EMIT_LOC_STRINGS: YES\n")
	b.WriteString("        LD_RUNPATH_SEARCH_PATHS:\n")
	b.WriteString("          - \"$(inherited)\"\n")
	b.WriteString("          - \"@executable_path/Frameworks\"\n")
	b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
	b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")

	// Permissions as INFOPLIST_KEY_* build settings
	for _, perm := range cfg.Permissions {
		fmt.Fprintf(&b, "        INFOPLIST_KEY_%s: %s\n", perm.Key, yamlQuote(perm.Description))
	}

	// Extra build settings
	for k, v := range cfg.BuildSettings {
		fmt.Fprintf(&b, "        %s: %s\n", k, yamlQuote(v))
	}

	// Main app entitlements
	b.WriteString("    entitlements:\n")
	fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", appName, appName)

	mainEntitlements := make(map[string]any)
	if needsAppGroups {
		mainEntitlements["com.apple.security.application-groups"] = []any{"group." + bundleID}
	}
	for _, ent := range cfg.Entitlements {
		if ent.Target == "" || ent.Target == appName {
			mainEntitlements[ent.Key] = ent.Value
		}
	}
	if len(mainEntitlements) > 0 {
		b.WriteString("      properties:\n")
		writeYAMLMap(&b, mainEntitlements, 8)
	} else {
		b.WriteString("      properties: {}\n")
	}

	// Main app Info.plist
	mainInfoPlist := make(map[string]any)
	for _, ext := range cfg.Extensions {
		if ext.Kind == "live_activity" {
			mainInfoPlist["NSSupportsLiveActivities"] = true
			break
		}
	}
	if len(mainInfoPlist) > 0 {
		b.WriteString("    info:\n")
		fmt.Fprintf(&b, "      path: %s/Info.plist\n", appName)
		b.WriteString("      properties:\n")
		writeYAMLMap(&b, mainInfoPlist, 8)
	}

	// Dependencies: embed extension targets + SPM packages
	if hasExtensions || len(cfg.Packages) > 0 {
		b.WriteString("    dependencies:\n")
		for _, ext := range cfg.Extensions {
			name := extensionTargetName(ext, appName)
			fmt.Fprintf(&b, "      - target: %s\n", name)
			b.WriteString("        embed: true\n")
		}
		writePackageDependenciesCfg(&b, cfg.Packages)
	}

	// Extension targets
	for _, ext := range cfg.Extensions {
		name := extensionTargetName(ext, appName)
		kind := ext.Kind
		extBundleID := fmt.Sprintf("%s.%s", bundleID, strings.ReplaceAll(kind, "_", ""))
		sourcePath := fmt.Sprintf("Targets/%s", name)

		b.WriteString("\n")
		fmt.Fprintf(&b, "  %s:\n", name)
		fmt.Fprintf(&b, "    type: %s\n", xcodegenTargetType(kind))
		b.WriteString("    platform: iOS\n")
		b.WriteString("    sources:\n")
		fmt.Fprintf(&b, "      - path: %s\n", sourcePath)
		b.WriteString("        type: folder\n")
		b.WriteString("      - path: Shared\n")
		b.WriteString("        type: folder\n")
		b.WriteString("        optional: true\n")
		b.WriteString("    settings:\n")
		b.WriteString("      base:\n")
		fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", extBundleID)
		b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
		b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
		b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
		b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
		b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")

		for k, v := range ext.Settings {
			fmt.Fprintf(&b, "        %s: %s\n", k, yamlQuote(v))
		}

		infoPlist := mergeInfoPlistDefaults(kind, ext.InfoPlist)
		if len(infoPlist) > 0 {
			b.WriteString("    info:\n")
			fmt.Fprintf(&b, "      path: %s/Info.plist\n", sourcePath)
			b.WriteString("      properties:\n")
			writeYAMLMap(&b, infoPlist, 8)
		}

		entitlements := mergeEntitlementDefaults(kind, ext.Entitlements, bundleID)
		// Merge any config-level entitlements targeting this extension
		for _, ent := range cfg.Entitlements {
			if ent.Target == name {
				entitlements[ent.Key] = ent.Value
			}
		}
		if len(entitlements) > 0 {
			b.WriteString("    entitlements:\n")
			fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", sourcePath, name)
			b.WriteString("      properties:\n")
			writeYAMLMap(&b, entitlements, 8)
		}
	}

	return b.String()
}

// generateWatchOnlyYAMLCfg produces project.yml for a standalone watchOS app from config.
func generateWatchOnlyYAMLCfg(cfg *ProjectConfig) string {
	var b strings.Builder
	appName := cfg.AppName
	bundleID := cfg.BundleID
	watchAppName := watchAppTargetName(appName)
	watchBundleID := bundleID + ".watchkitapp"
	watchExtName := watchExtensionTargetName(appName)
	watchExtBundleID := watchBundleID + ".watchkitextension"
	hasExtensions := len(cfg.Extensions) > 0

	fmt.Fprintf(&b, "name: %s\n", appName)
	b.WriteString("options:\n")
	fmt.Fprintf(&b, "  bundleIdPrefix: %s\n", bundleIDPrefix())
	b.WriteString("  deploymentTarget:\n")
	b.WriteString("    watchOS: \"26.0\"\n")
	b.WriteString("  xcodeVersion: \"16.0\"\n")
	b.WriteString("  createIntermediateGroups: true\n")
	b.WriteString("  generateEmptyDirectories: true\n")
	b.WriteString("  useBaseInternationalization: false\n")

	if len(cfg.Localizations) > 0 {
		b.WriteString("  knownRegions:\n")
		for _, lang := range cfg.Localizations {
			fmt.Fprintf(&b, "    - %s\n", lang)
		}
	}
	b.WriteString("\n")

	writePackagesSectionCfg(&b, cfg.Packages)

	b.WriteString("targets:\n")

	// Watch container target
	fmt.Fprintf(&b, "  %s:\n", appName)
	b.WriteString("    type: application.watchapp2-container\n")
	b.WriteString("    platform: watchOS\n")
	b.WriteString("    sources:\n")
	writeFolderSourceEntryCfg(&b, appName, []string{"**/*.swift", "*.plist", "*.entitlements"}, false)

	b.WriteString("    settings:\n")
	b.WriteString("      base:\n")
	b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
	fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", bundleID)
	b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
	b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
	b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
	b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")

	for _, perm := range cfg.Permissions {
		fmt.Fprintf(&b, "        INFOPLIST_KEY_%s: %s\n", perm.Key, yamlQuote(perm.Description))
	}
	for k, v := range cfg.BuildSettings {
		fmt.Fprintf(&b, "        %s: %s\n", k, yamlQuote(v))
	}

	b.WriteString("    entitlements:\n")
	fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", appName, appName)
	b.WriteString("      properties: {}\n")

	b.WriteString("    dependencies:\n")
	fmt.Fprintf(&b, "      - target: %s\n", watchAppName)
	b.WriteString("        embed: true\n")
	if hasExtensions {
		for _, ext := range cfg.Extensions {
			name := extensionTargetName(ext, appName)
			fmt.Fprintf(&b, "      - target: %s\n", name)
			b.WriteString("        embed: true\n")
		}
	}
	writePackageDependenciesCfg(&b, cfg.Packages)

	// Watch app target (wrapper app bundle)
	b.WriteString("\n")
	fmt.Fprintf(&b, "  %s:\n", watchAppName)
	b.WriteString("    type: application.watchapp2\n")
	b.WriteString("    platform: watchOS\n")
	b.WriteString("    sources:\n")
	writeFolderSourceEntryCfg(&b, appName, []string{"**/*.swift", "*.plist", "*.entitlements"}, false)
	b.WriteString("    settings:\n")
	b.WriteString("      base:\n")
	b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
	fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", watchBundleID)
	b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
	b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
	b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
	b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
	writeWatchOSBuildSettingsCfg(&b)
	for _, perm := range cfg.Permissions {
		fmt.Fprintf(&b, "        INFOPLIST_KEY_%s: %s\n", perm.Key, yamlQuote(perm.Description))
	}

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
	writeIntrinsicWatchExtensionTargetYAMLCfg(&b, watchExtName, appName, watchExtBundleID, watchBundleID, hasExtensions)

	for _, ext := range cfg.Extensions {
		name := extensionTargetName(ext, appName)
		kind := ext.Kind
		extBundleID := fmt.Sprintf("%s.%s", bundleID, strings.ReplaceAll(kind, "_", ""))
		sourcePath := fmt.Sprintf("Targets/%s", name)

		b.WriteString("\n")
		fmt.Fprintf(&b, "  %s:\n", name)
		fmt.Fprintf(&b, "    type: %s\n", xcodegenTargetType(kind))
		b.WriteString("    platform: watchOS\n")
		b.WriteString("    sources:\n")
		writeFolderSourceEntryCfg(&b, sourcePath, nil, false)
		writeFolderSourceEntryCfg(&b, "Shared", nil, true)
		b.WriteString("    settings:\n")
		b.WriteString("      base:\n")
		fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", extBundleID)
		b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
		b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
		b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
		b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
		b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")

		for k, v := range ext.Settings {
			fmt.Fprintf(&b, "        %s: %s\n", k, yamlQuote(v))
		}

		infoPlist := mergeInfoPlistDefaults(kind, ext.InfoPlist)
		if len(infoPlist) > 0 {
			b.WriteString("    info:\n")
			fmt.Fprintf(&b, "      path: %s/Info.plist\n", sourcePath)
			b.WriteString("      properties:\n")
			writeYAMLMap(&b, infoPlist, 8)
		}

		entitlements := mergeEntitlementDefaults(kind, ext.Entitlements, bundleID)
		for _, ent := range cfg.Entitlements {
			if ent.Target == name {
				entitlements[ent.Key] = ent.Value
			}
		}
		if len(entitlements) > 0 {
			b.WriteString("    entitlements:\n")
			fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", sourcePath, name)
			b.WriteString("      properties:\n")
			writeYAMLMap(&b, entitlements, 8)
		}
	}

	b.WriteString("\nschemes:\n")
	fmt.Fprintf(&b, "  %s:\n", appName)
	b.WriteString("    build:\n")
	b.WriteString("      targets:\n")
	fmt.Fprintf(&b, "        %s: all\n", appName)
	fmt.Fprintf(&b, "        %s: all\n", watchAppName)
	fmt.Fprintf(&b, "        %s: all\n", watchExtName)
	for _, ext := range cfg.Extensions {
		name := extensionTargetName(ext, appName)
		fmt.Fprintf(&b, "        %s: all\n", name)
	}
	b.WriteString("    run:\n")
	fmt.Fprintf(&b, "      executable: %s\n", appName)

	return b.String()
}

// generatePairedYAMLCfg produces project.yml for paired iOS+watchOS from config.
func generatePairedYAMLCfg(cfg *ProjectConfig) string {
	var b strings.Builder
	appName := cfg.AppName
	bundleID := cfg.BundleID
	watchAppName := watchAppTargetName(appName)
	watchBundleID := bundleID + ".watchkitapp"
	watchExtName := watchExtensionTargetName(appName)
	watchExtBundleID := watchBundleID + ".watchkitextension"
	hasExtensions := len(cfg.Extensions) > 0

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

	if len(cfg.Localizations) > 0 {
		b.WriteString("  knownRegions:\n")
		for _, lang := range cfg.Localizations {
			fmt.Fprintf(&b, "    - %s\n", lang)
		}
	}
	b.WriteString("\n")

	writePackagesSectionCfg(&b, cfg.Packages)

	b.WriteString("targets:\n")

	// iOS parent target
	fmt.Fprintf(&b, "  %s:\n", appName)
	b.WriteString("    type: application\n")
	b.WriteString("    platform: iOS\n")
	b.WriteString("    supportedDestinations:\n")
	b.WriteString("      - iOS\n")
	b.WriteString("    sources:\n")
	fmt.Fprintf(&b, "      - path: %s\n", appName)
	b.WriteString("        type: folder\n")
	if hasExtensions {
		b.WriteString("      - path: Shared\n")
		b.WriteString("        type: folder\n")
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

	for _, perm := range cfg.Permissions {
		fmt.Fprintf(&b, "        INFOPLIST_KEY_%s: %s\n", perm.Key, yamlQuote(perm.Description))
	}
	for k, v := range cfg.BuildSettings {
		fmt.Fprintf(&b, "        %s: %s\n", k, yamlQuote(v))
	}

	b.WriteString("    entitlements:\n")
	fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", appName, appName)
	b.WriteString("      properties: {}\n")

	b.WriteString("    dependencies:\n")
	fmt.Fprintf(&b, "      - target: %s\n", watchAppName)
	b.WriteString("        embed: true\n")
	if hasExtensions {
		for _, ext := range cfg.Extensions {
			name := extensionTargetName(ext, appName)
			fmt.Fprintf(&b, "      - target: %s\n", name)
			b.WriteString("        embed: true\n")
		}
	}
	writePackageDependenciesCfg(&b, cfg.Packages)

	// Watch target
	b.WriteString("\n")
	fmt.Fprintf(&b, "  %s:\n", watchAppName)
	b.WriteString("    type: application.watchapp2\n")
	b.WriteString("    platform: watchOS\n")
	b.WriteString("    sources:\n")
	writeFolderSourceEntryCfg(&b, watchAppName, []string{"**/*.swift", "*.plist", "*.entitlements"}, false)

	b.WriteString("    settings:\n")
	b.WriteString("      base:\n")
	b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
	fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", watchBundleID)
	b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
	b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
	b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
	b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
	writeWatchOSBuildSettingsCfg(&b)

	b.WriteString("    entitlements:\n")
	fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", watchAppName, watchAppName)
	b.WriteString("      properties: {}\n")

	b.WriteString("    info:\n")
	fmt.Fprintf(&b, "      path: %s/Info.plist\n", watchAppName)
	b.WriteString("      properties:\n")
	fmt.Fprintf(&b, "        WKCompanionAppBundleIdentifier: %s\n", yamlQuote(bundleID))
	b.WriteString("        WKRunsIndependentlyOfCompanionApp: true\n")
	b.WriteString("    dependencies:\n")
	fmt.Fprintf(&b, "      - target: %s\n", watchExtName)
	b.WriteString("        embed: true\n")

	// Intrinsic watch runtime extension target
	writeIntrinsicWatchExtensionTargetYAMLCfg(&b, watchExtName, watchAppName, watchExtBundleID, watchBundleID, hasExtensions)

	// Watch extension targets
	for _, ext := range cfg.Extensions {
		name := extensionTargetName(ext, appName)
		kind := ext.Kind
		extBundleID := fmt.Sprintf("%s.%s", watchBundleID, strings.ReplaceAll(kind, "_", ""))
		sourcePath := fmt.Sprintf("Targets/%s", name)

		b.WriteString("\n")
		fmt.Fprintf(&b, "  %s:\n", name)
		fmt.Fprintf(&b, "    type: %s\n", xcodegenTargetType(kind))
		b.WriteString("    platform: watchOS\n")
		b.WriteString("    sources:\n")
		writeFolderSourceEntryCfg(&b, sourcePath, nil, false)
		writeFolderSourceEntryCfg(&b, "Shared", nil, true)
		b.WriteString("    settings:\n")
		b.WriteString("      base:\n")
		fmt.Fprintf(&b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", extBundleID)
		b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
		b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
		b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
		b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
		b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")

		for k, v := range ext.Settings {
			fmt.Fprintf(&b, "        %s: %s\n", k, yamlQuote(v))
		}

		infoPlist := mergeInfoPlistDefaults(kind, ext.InfoPlist)
		if len(infoPlist) > 0 {
			b.WriteString("    info:\n")
			fmt.Fprintf(&b, "      path: %s/Info.plist\n", sourcePath)
			b.WriteString("      properties:\n")
			writeYAMLMap(&b, infoPlist, 8)
		}

		entitlements := mergeEntitlementDefaults(kind, ext.Entitlements, watchBundleID)
		for _, ent := range cfg.Entitlements {
			if ent.Target == name {
				entitlements[ent.Key] = ent.Value
			}
		}
		if len(entitlements) > 0 {
			b.WriteString("    entitlements:\n")
			fmt.Fprintf(&b, "      path: %s/%s.entitlements\n", sourcePath, name)
			b.WriteString("      properties:\n")
			writeYAMLMap(&b, entitlements, 8)
		}
	}

	// Scheme
	b.WriteString("\nschemes:\n")
	fmt.Fprintf(&b, "  %s:\n", appName)
	b.WriteString("    build:\n")
	b.WriteString("      targets:\n")
	fmt.Fprintf(&b, "        %s: all\n", appName)
	fmt.Fprintf(&b, "        %s: all\n", watchAppName)
	fmt.Fprintf(&b, "        %s: all\n", watchExtName)
	for _, ext := range cfg.Extensions {
		name := extensionTargetName(ext, appName)
		fmt.Fprintf(&b, "        %s: all\n", name)
	}
	b.WriteString("    run:\n")
	fmt.Fprintf(&b, "      executable: %s\n", appName)

	return b.String()
}

func writeWatchOSBuildSettingsCfg(b *strings.Builder) {
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

func writeFolderSourceEntryCfg(b *strings.Builder, path string, excludes []string, optional bool) {
	fmt.Fprintf(b, "      - path: %s\n", path)
	b.WriteString("        type: folder\n")
	if optional {
		b.WriteString("        optional: true\n")
	}
	if len(excludes) == 0 {
		return
	}
	b.WriteString("        excludes:\n")
	for _, pattern := range excludes {
		fmt.Fprintf(b, "          - %s\n", yamlQuote(pattern))
	}
}

func writeIntrinsicWatchExtensionTargetYAMLCfg(b *strings.Builder, targetName, sourcePath, extBundleID, watchAppBundleID string, includeShared bool) {
	b.WriteString("\n")
	fmt.Fprintf(b, "  %s:\n", targetName)
	b.WriteString("    type: watchkit2-extension\n")
	b.WriteString("    platform: watchOS\n")
	b.WriteString("    sources:\n")
	writeFolderSourceEntryCfg(b, sourcePath, []string{"*.plist", "*.entitlements"}, false)
	if includeShared {
		writeFolderSourceEntryCfg(b, "Shared", nil, true)
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
	fmt.Fprintf(b, "            WKAppBundleIdentifier: %s\n", yamlQuote(watchAppBundleID))
}

// writeIOSDestinationSettingsCfg constrains Xcode "Supported Destinations" for iOS apps.
func writeIOSDestinationSettingsCfg(b *strings.Builder, family string) {
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

func xcodegenTargetType(kind string) string {
	if kind == "app_clip" {
		return "app-clip"
	}
	return "app-extension"
}

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
	for k, v := range planValues {
		m[k] = v
	}
	return m
}

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

func deviceFamilyBuildSettingsCfg(b *strings.Builder, family string) {
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

func bundleIDPrefix() string {
	u, err := user.Current()
	if err != nil || u.Username == "" {
		return "com.app"
	}
	var b strings.Builder
	for _, r := range strings.ToLower(u.Username) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	name := b.String()
	if name == "" {
		return "com.app"
	}
	return "com." + name
}

// writePackagesSectionCfg writes the top-level `packages:` block for SPM dependencies.
func writePackagesSectionCfg(b *strings.Builder, packages []PackageDep) {
	if len(packages) == 0 {
		return
	}
	b.WriteString("packages:\n")
	for _, pkg := range packages {
		fmt.Fprintf(b, "  %s:\n", pkg.Name)
		fmt.Fprintf(b, "    url: %s\n", pkg.URL)
		fmt.Fprintf(b, "    minVersion: %s\n", pkg.MinVersion)
	}
	b.WriteString("\n")
}

// writePackageDependenciesCfg writes `- package: ProductName` entries in a target's dependencies section.
func writePackageDependenciesCfg(b *strings.Builder, packages []PackageDep) {
	for _, pkg := range packages {
		products := pkg.Products
		if len(products) == 0 {
			products = []string{pkg.Name}
		}
		for _, product := range products {
			fmt.Fprintf(b, "      - package: %s\n", product)
		}
	}
}

func yamlQuote(s string) string {
	if strings.ContainsAny(s, ":{}[]|>&*!%#@,") || strings.Contains(s, "  ") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

func writeYAMLMap(b *strings.Builder, m map[string]any, indent int) {
	prefix := strings.Repeat(" ", indent)
	for k, v := range m {
		switch val := v.(type) {
		case bool:
			fmt.Fprintf(b, "%s%s: %t\n", prefix, k, val)
		case string:
			fmt.Fprintf(b, "%s%s: %s\n", prefix, k, yamlQuote(val))
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
			writeYAMLMap(b, val, indent+2)
		default:
			fmt.Fprintf(b, "%s%s: %v\n", prefix, k, val)
		}
	}
}
