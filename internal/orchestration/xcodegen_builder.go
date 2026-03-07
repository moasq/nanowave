package orchestration

import (
	"fmt"
	"strings"
)

// targetBuilder captures common XcodeGen YAML generation patterns shared across
// all platform generators (iOS, tvOS, macOS, visionOS, watchOS).
// Each method writes exact YAML strings to match expected test output.
type targetBuilder struct {
	b        *strings.Builder
	appName  string
	bundleID string
	platform string
	plan     *PlannerResult
}

func newTargetBuilder(b *strings.Builder, appName, platform string, plan *PlannerResult) *targetBuilder {
	return &targetBuilder{
		b:        b,
		appName:  appName,
		bundleID: fmt.Sprintf("%s.%s", bundleIDPrefix(), strings.ToLower(appName)),
		platform: platform,
		plan:     plan,
	}
}

// writeHeader writes the project name.
func (t *targetBuilder) writeHeader(resolvedPkgs []*CuratedPackage) {
	fmt.Fprintf(t.b, "name: %s\n", t.appName)
	writePackagesSection(t.b, resolvedPkgs)
}

// writeOptions writes the options section with deployment target.
func (t *targetBuilder) writeOptions(deploymentTargets map[string]string) {
	t.b.WriteString("options:\n")
	fmt.Fprintf(t.b, "  bundleIdPrefix: %s\n", bundleIDPrefix())
	t.b.WriteString("  deploymentTarget:\n")
	for platform, version := range deploymentTargets {
		fmt.Fprintf(t.b, "    %s: \"%s\"\n", platform, version)
	}
	t.b.WriteString("  xcodeVersion: \"16.0\"\n")
	t.b.WriteString("  createIntermediateGroups: true\n")
	t.b.WriteString("  generateEmptyDirectories: true\n")
	t.b.WriteString("  useBaseInternationalization: false\n")
}

// writeLocalizations writes knownRegions if plan has localizations.
func (t *targetBuilder) writeLocalizations() {
	if t.plan != nil && len(t.plan.Localizations) > 0 {
		t.b.WriteString("  knownRegions:\n")
		for _, lang := range t.plan.Localizations {
			fmt.Fprintf(t.b, "    - %s\n", lang)
		}
	}
	t.b.WriteString("\n")
}

// writeCommonBuildSettings writes build settings shared by all single-platform targets.
func (t *targetBuilder) writeCommonBuildSettings() {
	t.b.WriteString("    settings:\n")
	t.b.WriteString("      base:\n")
	t.b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
	fmt.Fprintf(t.b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", t.bundleID)
	t.b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
	t.b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
	t.b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
	t.b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
}

// writeCommonPostBuildSettings writes build settings that appear after platform-specific ones.
func (t *targetBuilder) writeCommonPostBuildSettings() {
	t.b.WriteString("        ASSETCATALOG_COMPILER_APPICON_NAME: AppIcon\n")
	t.b.WriteString("        INFOPLIST_KEY_CFBundleIconName: AppIcon\n")
	t.b.WriteString("        ASSETCATALOG_COMPILER_GLOBAL_ACCENT_COLOR_NAME: AccentColor\n")
	t.b.WriteString("        ENABLE_PREVIEWS: YES\n")
	t.b.WriteString("        SWIFT_EMIT_LOC_STRINGS: YES\n")
	t.b.WriteString("        LD_RUNPATH_SEARCH_PATHS:\n")
	t.b.WriteString("          - \"$(inherited)\"\n")
	t.b.WriteString("          - \"@executable_path/Frameworks\"\n")
	t.b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
	t.b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")
}

// writePermissions writes permission build settings from the plan.
func (t *targetBuilder) writePermissions() {
	if t.plan != nil {
		for _, perm := range t.plan.Permissions {
			fmt.Fprintf(t.b, "        INFOPLIST_KEY_%s: %s\n", perm.Key, xcodeYAMLQuote(perm.Description))
		}
	}
}

// writeEntitlements writes the entitlements section for the main target.
func (t *targetBuilder) writeEntitlements(sourceDir string, entitlements map[string]any) {
	t.b.WriteString("    entitlements:\n")
	fmt.Fprintf(t.b, "      path: %s/%s.entitlements\n", sourceDir, t.appName)
	writeEntitlementProperties(t.b, entitlements)
}

// writeDependencies writes the dependencies section (packages + extensions).
func (t *targetBuilder) writeDependencies(resolvedPkgs []*CuratedPackage, hasExtensions bool) {
	hasPackages := len(resolvedPkgs) > 0
	if !hasExtensions && !hasPackages {
		return
	}
	t.b.WriteString("    dependencies:\n")
	writePackageDependencies(t.b, resolvedPkgs)
	if hasExtensions && t.plan != nil {
		for _, ext := range t.plan.Extensions {
			name := extensionTargetName(ext, t.appName)
			fmt.Fprintf(t.b, "      - target: %s\n", name)
			t.b.WriteString("        embed: true\n")
		}
	}
}

// writeExtensionTargets writes all extension targets for the given platform.
func (t *targetBuilder) writeExtensionTargets(platformStr string) {
	if t.plan == nil {
		return
	}
	for _, ext := range t.plan.Extensions {
		name := extensionTargetName(ext, t.appName)
		kind := ext.Kind
		kindForBundleID := strings.ReplaceAll(kind, "_", "")
		if kindForBundleID == "" {
			kindForBundleID = strings.ToLower(name)
		}
		extBundleID := fmt.Sprintf("%s.%s", t.bundleID, kindForBundleID)
		sourcePath := fmt.Sprintf("Targets/%s", name)

		t.b.WriteString("\n")
		fmt.Fprintf(t.b, "  %s:\n", name)
		fmt.Fprintf(t.b, "    type: %s\n", xcodegenTargetType(kind))
		fmt.Fprintf(t.b, "    platform: %s\n", platformStr)
		t.b.WriteString("    sources:\n")
		fmt.Fprintf(t.b, "      - path: %s\n", sourcePath)
		t.b.WriteString("        type: folder\n")
		t.b.WriteString("      - path: Shared\n")
		t.b.WriteString("        type: folder\n")
		t.b.WriteString("        optional: true\n")
		t.b.WriteString("    settings:\n")
		t.b.WriteString("      base:\n")
		fmt.Fprintf(t.b, "        PRODUCT_BUNDLE_IDENTIFIER: %s\n", extBundleID)
		t.b.WriteString("        CODE_SIGN_STYLE: Automatic\n")
		t.b.WriteString("        SWIFT_VERSION: \"6.0\"\n")
		t.b.WriteString("        GENERATE_INFOPLIST_FILE: YES\n")
		t.b.WriteString("        SKIP_INSTALL: YES\n")
		t.b.WriteString("        DEAD_CODE_STRIPPING: NO\n")
		t.b.WriteString("        CURRENT_PROJECT_VERSION: 1\n")
		t.b.WriteString("        MARKETING_VERSION: \"1.0\"\n")
		t.b.WriteString("        SWIFT_APPROACHABLE_CONCURRENCY: YES\n")
		t.b.WriteString("        SWIFT_DEFAULT_ACTOR_ISOLATION: MainActor\n")

		for k, v := range ext.Settings {
			fmt.Fprintf(t.b, "        %s: %s\n", k, xcodeYAMLQuote(v))
		}

		infoPlist := mergeInfoPlistDefaults(kind, ext.InfoPlist)
		if len(infoPlist) > 0 {
			t.b.WriteString("    info:\n")
			fmt.Fprintf(t.b, "      path: %s/Info.plist\n", sourcePath)
			t.b.WriteString("      properties:\n")
			writeXcodeYAMLMap(t.b, infoPlist, 8)
		}

		entitlements := mergeEntitlementDefaults(kind, ext.Entitlements, t.bundleID)
		if len(entitlements) > 0 {
			t.b.WriteString("    entitlements:\n")
			fmt.Fprintf(t.b, "      path: %s/%s.entitlements\n", sourcePath, name)
			t.b.WriteString("      properties:\n")
			writeXcodeYAMLMap(t.b, entitlements, 8)
		}
	}
}

// writeScheme writes an explicit scheme when extensions or StoreKit are present.
func (t *targetBuilder) writeScheme(hasExtensions, hasMonetization bool) {
	if !hasExtensions && !hasMonetization {
		return
	}
	t.b.WriteString("\nschemes:\n")
	fmt.Fprintf(t.b, "  %s:\n", t.appName)
	t.b.WriteString("    build:\n")
	t.b.WriteString("      targets:\n")
	fmt.Fprintf(t.b, "        %s: all\n", t.appName)
	if hasExtensions && t.plan != nil {
		for _, ext := range t.plan.Extensions {
			name := extensionTargetName(ext, t.appName)
			fmt.Fprintf(t.b, "        %s: all\n", name)
		}
	}
	t.b.WriteString("    run:\n")
	fmt.Fprintf(t.b, "      executable: %s\n", t.appName)
	if hasMonetization {
		fmt.Fprintf(t.b, "      storeKitConfiguration: %s/%s.storekit\n", t.appName, t.appName)
	}
}
