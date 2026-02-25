package orchestration

import (
	"strings"
	"testing"
)

func TestGenerateProjectYAMLiOS(t *testing.T) {
	plan := &PlannerResult{
		Platform:     "ios",
		DeviceFamily: "iphone",
	}

	yml := generateProjectYAML("TestApp", plan)

	checks := []struct {
		desc string
		want string
	}{
		{"has iOS deployment target", "iOS: \"26.0\""},
		{"has application type", "type: application"},
		{"has iOS platform", "platform: iOS"},
		{"has supportedDestinations", "supportedDestinations:"},
		{"has iPhone destination filter", "device: iPhone"},
		{"has TARGETED_DEVICE_FAMILY", "TARGETED_DEVICE_FAMILY: \"1\""},
	}

	for _, c := range checks {
		if !strings.Contains(yml, c.want) {
			t.Errorf("iOS YAML: %s — expected to contain %q", c.desc, c.want)
		}
	}

	// Should NOT have watchOS markers
	if strings.Contains(yml, "watchOS") {
		t.Error("iOS YAML should not contain watchOS references")
	}
	if strings.Contains(yml, "WKWatchOnly") {
		t.Error("iOS YAML should not contain WKWatchOnly")
	}
}

func TestGenerateProjectYAMLiPad(t *testing.T) {
	plan := &PlannerResult{
		Platform:     "ios",
		DeviceFamily: "ipad",
	}

	yml := generateProjectYAML("TabletApp", plan)

	if !strings.Contains(yml, "TARGETED_DEVICE_FAMILY: \"2\"") {
		t.Error("iPad YAML should have TARGETED_DEVICE_FAMILY 2")
	}
	if !strings.Contains(yml, "device: iPad") {
		t.Error("iPad YAML should have iPad destination filter")
	}
}

func TestGenerateProjectYAMLUniversal(t *testing.T) {
	plan := &PlannerResult{
		Platform:     "ios",
		DeviceFamily: "universal",
	}

	yml := generateProjectYAML("UniApp", plan)

	if !strings.Contains(yml, "TARGETED_DEVICE_FAMILY: \"1,2\"") {
		t.Error("Universal YAML should have TARGETED_DEVICE_FAMILY 1,2")
	}
	if !strings.Contains(yml, "device: iPhone") || !strings.Contains(yml, "device: iPad") {
		t.Error("Universal YAML should have both iPhone and iPad destination filters")
	}
}

func TestGenerateProjectYAMLWatchOnly(t *testing.T) {
	plan := &PlannerResult{
		Platform:          "watchos",
		WatchProjectShape: "watch_only",
	}

	yml := generateProjectYAML("WatchApp", plan)

	checks := []struct {
		desc string
		want string
	}{
		{"has watchOS deployment target", "watchOS: \"26.0\""},
		{"has watchapp2-container type", "type: application.watchapp2-container"},
		{"has watchapp2 type", "type: application.watchapp2"},
		{"has watchkit2 extension type", "type: watchkit2-extension"},
		{"has watchOS platform", "platform: watchOS"},
		{"has WKWatchOnly info plist", "WKWatchOnly: true"},
		{"has watch app target", "WatchAppWatch:"},
		{"has intrinsic watch extension target", "WatchAppWatchExtension:"},
		{"container embeds watch app", "target: WatchAppWatch"},
		{"watch app embeds watch extension", "target: WatchAppWatchExtension"},
		{"has watchkit extension point", "NSExtensionPointIdentifier: com.apple.watchkit"},
	}

	for _, c := range checks {
		if !strings.Contains(yml, c.want) {
			t.Errorf("WatchOnly YAML: %s — expected to contain %q", c.desc, c.want)
		}
	}

	// Should NOT have iOS markers
	if strings.Contains(yml, "iOS: \"26.0\"") {
		t.Error("WatchOnly YAML should not contain iOS deployment target")
	}
	if strings.Contains(yml, "supportedDestinations:") {
		t.Error("WatchOnly YAML should not contain supportedDestinations")
	}
	if strings.Contains(yml, "destinationFilters:") {
		t.Error("WatchOnly YAML should not contain destinationFilters")
	}
	if strings.Contains(yml, "TARGETED_DEVICE_FAMILY") {
		t.Error("WatchOnly YAML should not contain TARGETED_DEVICE_FAMILY")
	}
}

func TestGenerateProjectYAMLPaired(t *testing.T) {
	plan := &PlannerResult{
		Platform:          "watchos",
		WatchProjectShape: "paired_ios_watch",
	}

	yml := generateProjectYAML("PairedApp", plan)

	checks := []struct {
		desc string
		want string
	}{
		{"has iOS deployment target", "iOS: \"26.0\""},
		{"has watchOS deployment target", "watchOS: \"26.0\""},
		{"has iOS app type", "type: application"},
		{"has watchapp2 type", "type: application.watchapp2"},
		{"has watchkit2 extension type", "type: watchkit2-extension"},
		{"has iOS platform", "platform: iOS"},
		{"has watchOS platform", "platform: watchOS"},
		{"has watch target name", "PairedAppWatch:"},
		{"has watch extension target name", "PairedAppWatchExtension:"},
		{"has watch bundle ID", ".watchkitapp"},
		{"has watch extension bundle ID", ".watchkitapp.watchkitextension"},
		{"has WKCompanionAppBundleIdentifier", "WKCompanionAppBundleIdentifier: " + bundleIDPrefix() + ".pairedapp"},
		{"has WKRunsIndependentlyOfCompanionApp", "WKRunsIndependentlyOfCompanionApp: true"},
		{"iOS depends on watch target", "target: PairedAppWatch"},
		{"watch app depends on watch extension", "target: PairedAppWatchExtension"},
		{"has watchkit extension point", "NSExtensionPointIdentifier: com.apple.watchkit"},
		{"has WKAppBundleIdentifier", "WKAppBundleIdentifier: " + bundleIDPrefix() + ".pairedapp.watchkitapp"},
		{"watch app excludes swift", "\"**/*.swift\""},
	}

	for _, c := range checks {
		if !strings.Contains(yml, c.want) {
			t.Errorf("Paired YAML: %s — expected to contain %q", c.desc, c.want)
		}
	}

	// Should NOT have WKWatchOnly (that's for standalone)
	if strings.Contains(yml, "WKWatchOnly") {
		t.Error("Paired YAML should not contain WKWatchOnly")
	}
}

func TestGenerateProjectYAMLWatchOnlyDoesNotRequireCompanionBundleIdentifier(t *testing.T) {
	plan := &PlannerResult{
		Platform:          "watchos",
		WatchProjectShape: "watch_only",
	}

	yml := generateProjectYAML("SoloWatch", plan)

	if strings.Contains(yml, "WKCompanionAppBundleIdentifier") {
		t.Error("watch_only YAML should not contain WKCompanionAppBundleIdentifier")
	}
	if !strings.Contains(yml, "WKWatchOnly: true") {
		t.Error("watch_only YAML should contain WKWatchOnly: true")
	}
	if !strings.Contains(yml, "type: watchkit2-extension") {
		t.Error("watch_only YAML should contain intrinsic watch extension target")
	}
}

func TestGenerateProjectYAMLWatchOnlyWithWidget(t *testing.T) {
	plan := &PlannerResult{
		Platform:          "watchos",
		WatchProjectShape: "watch_only",
		Extensions: []ExtensionPlan{
			{Kind: "widget", Name: "WatchWidgetAppWidget", Purpose: "Show summary"},
		},
	}

	yml := generateProjectYAML("WatchWidgetApp", plan)

	// Extension target should be present
	if !strings.Contains(yml, "WatchWidgetAppWidget:") {
		t.Error("should contain widget extension target")
	}

	// Count platform: watchOS occurrences (container + watch app + watch extension + widget extension)
	count := strings.Count(yml, "platform: watchOS")
	if count < 4 {
		t.Errorf("expected at least 4 'platform: watchOS' entries (container + watch app + watch extension + widget), got %d", count)
	}

	// Shared dir should be included
	if !strings.Contains(yml, "path: Shared") {
		t.Error("should include Shared directory for extensions")
	}
}

func TestGenerateProjectYAMLPairedWithWidget(t *testing.T) {
	plan := &PlannerResult{
		Platform:          "watchos",
		WatchProjectShape: "paired_ios_watch",
		Extensions: []ExtensionPlan{
			{Kind: "widget", Name: "PairedWidgetAppWidget", Purpose: "Show summary"},
		},
	}

	yml := generateProjectYAML("PairedWidgetApp", plan)

	// Should have 4 targets: iOS app, watch app, watch runtime extension, widget extension
	if !strings.Contains(yml, "PairedWidgetApp:") {
		t.Error("should contain iOS app target")
	}
	if !strings.Contains(yml, "PairedWidgetAppWatch:") {
		t.Error("should contain watch app target")
	}
	if !strings.Contains(yml, "PairedWidgetAppWatchExtension:") {
		t.Error("should contain intrinsic watch runtime extension target")
	}
	if !strings.Contains(yml, "PairedWidgetAppWidget:") {
		t.Error("should contain widget extension target")
	}

	// Widget bundle ID should be under watchkitapp
	if !strings.Contains(yml, ".watchkitapp.widget") {
		t.Error("widget bundle ID should be under watchkitapp")
	}
}

func TestGenerateProjectYAMLPairedWatchHasInstallableTargetGraph(t *testing.T) {
	plan := &PlannerResult{
		Platform:          "watchos",
		WatchProjectShape: "paired_ios_watch",
	}

	yml := generateProjectYAML("TapCounter", plan)

	markers := []string{
		"TapCounterWatch:",
		"TapCounterWatchExtension:",
		"type: application.watchapp2",
		"type: watchkit2-extension",
		"target: TapCounterWatch",
		"target: TapCounterWatchExtension",
		"NSExtensionPointIdentifier: com.apple.watchkit",
		"WKAppBundleIdentifier: " + bundleIDPrefix() + ".tapcounter.watchkitapp",
	}
	for _, m := range markers {
		if !strings.Contains(yml, m) {
			t.Fatalf("paired watch YAML missing required marker %q:\n%s", m, yml)
		}
	}
}

func TestGenerateProjectYAMLTvOS(t *testing.T) {
	plan := &PlannerResult{
		Platform: "tvos",
	}

	yml := generateProjectYAML("TVApp", plan)

	checks := []struct {
		desc string
		want string
	}{
		{"has tvOS deployment target", "tvOS: \"26.0\""},
		{"has application type", "type: application"},
		{"has tvOS platform", "platform: tvOS"},
		{"has supportedDestinations", "supportedDestinations:"},
		{"has tvOS destination", "- tvOS"},
		{"has TARGETED_DEVICE_FAMILY 3", "TARGETED_DEVICE_FAMILY: \"3\""},
	}

	for _, c := range checks {
		if !strings.Contains(yml, c.want) {
			t.Errorf("tvOS YAML: %s — expected to contain %q", c.desc, c.want)
		}
	}

	// Should NOT have iOS/watchOS markers
	if strings.Contains(yml, "iOS: \"26.0\"") {
		t.Error("tvOS YAML should not contain iOS deployment target")
	}
	if strings.Contains(yml, "watchOS") {
		t.Error("tvOS YAML should not contain watchOS references")
	}
	if strings.Contains(yml, "UILaunchScreen") {
		t.Error("tvOS YAML should not contain UILaunchScreen")
	}
	if strings.Contains(yml, "UISupportedInterfaceOrientations") {
		t.Error("tvOS YAML should not contain orientation settings")
	}
}

func TestGenerateMultiPlatformProjectYAML(t *testing.T) {
	plan := &PlannerResult{
		Platform:          "ios",
		Platforms:         []string{"ios", "watchos", "tvos"},
		DeviceFamily:      "universal",
		WatchProjectShape: "paired_ios_watch",
	}

	yml := generateProjectYAML("FocusFlow", plan)

	checks := []struct {
		desc string
		want string
	}{
		{"has iOS deployment target", "iOS: \"26.0\""},
		{"has watchOS deployment target", "watchOS: \"26.0\""},
		{"has tvOS deployment target", "tvOS: \"26.0\""},
		{"has iOS main target", "FocusFlow:"},
		{"has watch app target", "FocusFlowWatch:"},
		{"has watch extension target", "FocusFlowWatchExtension:"},
		{"has tvOS target", "FocusFlowTV:"},
		{"has iOS platform", "platform: iOS"},
		{"has watchOS platform", "platform: watchOS"},
		{"has tvOS platform", "platform: tvOS"},
		{"has Shared sources", "path: Shared"},
	}

	for _, c := range checks {
		if !strings.Contains(yml, c.want) {
			t.Errorf("Multi-platform YAML: %s — expected to contain %q", c.desc, c.want)
		}
	}
}

func TestGenerateMultiPlatformYAMLiOSWatchOnly(t *testing.T) {
	plan := &PlannerResult{
		Platform:          "ios",
		Platforms:         []string{"ios", "watchos"},
		DeviceFamily:      "iphone",
		WatchProjectShape: "paired_ios_watch",
	}

	yml := generateProjectYAML("MyApp", plan)

	// Should have iOS + watchOS but NOT tvOS
	if !strings.Contains(yml, "iOS: \"26.0\"") {
		t.Error("should have iOS deployment target")
	}
	if !strings.Contains(yml, "watchOS: \"26.0\"") {
		t.Error("should have watchOS deployment target")
	}
	if strings.Contains(yml, "tvOS: \"26.0\"") {
		t.Error("should NOT have tvOS deployment target when tvos not in platforms")
	}
	if strings.Contains(yml, "MyAppTV:") {
		t.Error("should NOT have tvOS target when tvos not in platforms")
	}
}

func TestGenerateProjectYAMLEmptyExtensionKindSafeBundleID(t *testing.T) {
	plan := &PlannerResult{
		Platform:     "ios",
		DeviceFamily: "universal",
		Extensions: []ExtensionPlan{
			{Kind: "", Name: "FocusFlowWidget", Purpose: "home screen widget"},
		},
		Files: []FilePlan{
			{Path: "App/MyApp.swift", TypeName: "MyApp"},
		},
	}

	yml := generateProjectYAML("FocusFlow", plan)

	// Bundle ID should use lowercase target name, NOT have a trailing dot
	if strings.Contains(yml, "PRODUCT_BUNDLE_IDENTIFIER: "+bundleIDPrefix()+".focusflow.\n") {
		t.Error("extension bundle ID should not end with a trailing dot when kind is empty")
	}
	if !strings.Contains(yml, "focusflowwidget") {
		t.Error("extension bundle ID should use lowercase target name as fallback")
	}
}

func TestGenerateProjectYAMLVisionOS(t *testing.T) {
	plan := &PlannerResult{
		Platform: "visionos",
	}

	yml := generateProjectYAML("VisionApp", plan)

	checks := []struct {
		desc string
		want string
	}{
		{"has visionOS deployment target", "visionOS: \"26.0\""},
		{"has application type", "type: application"},
		{"has visionOS platform", "platform: visionOS"},
		{"has supportedDestinations", "supportedDestinations:"},
		{"has visionOS destination", "- visionOS"},
		{"has TARGETED_DEVICE_FAMILY 7", "TARGETED_DEVICE_FAMILY: \"7\""},
	}

	for _, c := range checks {
		if !strings.Contains(yml, c.want) {
			t.Errorf("visionOS YAML: %s — expected to contain %q", c.desc, c.want)
		}
	}

	// Should NOT have iOS/watchOS markers
	if strings.Contains(yml, "iOS: \"26.0\"") {
		t.Error("visionOS YAML should not contain iOS deployment target")
	}
	if strings.Contains(yml, "watchOS") {
		t.Error("visionOS YAML should not contain watchOS references")
	}
	if strings.Contains(yml, "UILaunchScreen") {
		t.Error("visionOS YAML should not contain UILaunchScreen")
	}
	if strings.Contains(yml, "UIApplicationSceneManifest") {
		t.Error("visionOS YAML should not contain UIApplicationSceneManifest")
	}
}

func TestGenerateMultiPlatformProjectYAMLWithVisionOS(t *testing.T) {
	plan := &PlannerResult{
		Platform:     "ios",
		Platforms:    []string{"ios", "visionos"},
		DeviceFamily: "iphone",
	}

	yml := generateProjectYAML("SpatialApp", plan)

	checks := []struct {
		desc string
		want string
	}{
		{"has iOS deployment target", "iOS: \"26.0\""},
		{"has visionOS deployment target", "visionOS: \"26.0\""},
		{"has iOS main target", "SpatialApp:"},
		{"has visionOS target", "SpatialAppVision:"},
		{"has visionOS platform", "platform: visionOS"},
		{"has TARGETED_DEVICE_FAMILY 7", "TARGETED_DEVICE_FAMILY: \"7\""},
		{"has Shared sources", "path: Shared"},
		{"has visionOS source dir", "path: SpatialAppVision"},
		{"has visionOS bundle ID suffix", ".vision"},
	}

	for _, c := range checks {
		if !strings.Contains(yml, c.want) {
			t.Errorf("Multi-platform with visionOS YAML: %s — expected to contain %q", c.desc, c.want)
		}
	}
}

func TestGenerateProjectYAMLMacOS(t *testing.T) {
	plan := &PlannerResult{
		Platform: "macos",
	}

	yml := generateProjectYAML("MacApp", plan)

	checks := []struct {
		desc string
		want string
	}{
		{"has macOS deployment target", "macOS: \"26.0\""},
		{"has application type", "type: application"},
		{"has macOS platform", "platform: macOS"},
		{"has supportedDestinations", "supportedDestinations:"},
		{"has macOS destination", "- macOS"},
		{"has COMBINE_HIDPI_IMAGES", "COMBINE_HIDPI_IMAGES: YES"},
	}

	for _, c := range checks {
		if !strings.Contains(yml, c.want) {
			t.Errorf("macOS YAML: %s — expected to contain %q", c.desc, c.want)
		}
	}

	// macOS should NOT have TARGETED_DEVICE_FAMILY
	if strings.Contains(yml, "TARGETED_DEVICE_FAMILY") {
		t.Error("macOS YAML should not contain TARGETED_DEVICE_FAMILY")
	}
	// Should NOT have iOS/watchOS markers
	if strings.Contains(yml, "iOS: \"26.0\"") {
		t.Error("macOS YAML should not contain iOS deployment target")
	}
	if strings.Contains(yml, "watchOS") {
		t.Error("macOS YAML should not contain watchOS references")
	}
	if strings.Contains(yml, "UILaunchScreen") {
		t.Error("macOS YAML should not contain UILaunchScreen")
	}
	if strings.Contains(yml, "UIApplicationSceneManifest") {
		t.Error("macOS YAML should not contain UIApplicationSceneManifest")
	}
	if strings.Contains(yml, "UISupportedInterfaceOrientations") {
		t.Error("macOS YAML should not contain orientation settings")
	}
}

func TestGenerateMultiPlatformProjectYAMLWithMacOS(t *testing.T) {
	plan := &PlannerResult{
		Platform:     "ios",
		Platforms:    []string{"ios", "macos"},
		DeviceFamily: "iphone",
	}

	yml := generateProjectYAML("ProdApp", plan)

	checks := []struct {
		desc string
		want string
	}{
		{"has iOS deployment target", "iOS: \"26.0\""},
		{"has macOS deployment target", "macOS: \"26.0\""},
		{"has iOS main target", "ProdApp:"},
		{"has macOS target", "ProdAppMac:"},
		{"has macOS platform", "platform: macOS"},
		{"has Shared sources", "path: Shared"},
		{"has macOS source dir", "path: ProdAppMac"},
		{"has macOS bundle ID suffix", ".mac"},
	}

	for _, c := range checks {
		if !strings.Contains(yml, c.want) {
			t.Errorf("Multi-platform with macOS YAML: %s — expected to contain %q", c.desc, c.want)
		}
	}

	// macOS target should NOT have TARGETED_DEVICE_FAMILY
	// (iOS target will have it, but we check that macOS section doesn't)
	macIdx := strings.Index(yml, "ProdAppMac:")
	if macIdx < 0 {
		t.Fatal("ProdAppMac target not found")
	}
	macSection := yml[macIdx:]
	if strings.Contains(macSection, "TARGETED_DEVICE_FAMILY") {
		t.Error("macOS target in multi-platform YAML should not contain TARGETED_DEVICE_FAMILY")
	}
}

func TestGenerateMultiPlatformYAMLiOSMacOSNoDependencies(t *testing.T) {
	plan := &PlannerResult{
		Platform:     "ios",
		Platforms:    []string{"ios", "macos"},
		DeviceFamily: "iphone",
	}

	yml := generateProjectYAML("RecipeBook", plan)

	// iOS target without watch or extensions should NOT have an empty dependencies key
	iosIdx := strings.Index(yml, "  RecipeBook:")
	macIdx := strings.Index(yml, "  RecipeBookMac:")
	if iosIdx < 0 || macIdx < 0 {
		t.Fatalf("expected both targets; iOS=%d, macOS=%d", iosIdx, macIdx)
	}
	iosSection := yml[iosIdx:macIdx]
	if strings.Contains(iosSection, "dependencies:") {
		t.Error("iOS target should not have empty dependencies when no watch/extensions are present")
	}
}

func TestGenerateProjectYAMLDefaultsToIOS(t *testing.T) {
	plan := &PlannerResult{}

	yml := generateProjectYAML("DefaultApp", plan)

	if !strings.Contains(yml, "iOS: \"26.0\"") {
		t.Error("default should produce iOS YAML")
	}
	if strings.Contains(yml, "watchOS") {
		t.Error("default should not produce watchOS YAML")
	}
}

func TestAppearanceLockIOSLightByDefault(t *testing.T) {
	plan := &PlannerResult{Platform: "ios", DeviceFamily: "iphone"}
	yml := generateProjectYAML("App", plan)
	if !strings.Contains(yml, "INFOPLIST_KEY_UIUserInterfaceStyle: Light") {
		t.Error("iOS YAML without dark-mode rule should contain UIUserInterfaceStyle: Light")
	}
}

func TestAppearanceLockIOSOmittedWithDarkMode(t *testing.T) {
	plan := &PlannerResult{Platform: "ios", DeviceFamily: "iphone", RuleKeys: []string{"dark-mode"}}
	yml := generateProjectYAML("App", plan)
	if strings.Contains(yml, "UIUserInterfaceStyle") {
		t.Error("iOS YAML with dark-mode rule should NOT contain UIUserInterfaceStyle")
	}
}

func TestAppearanceLockMacOSNoLock(t *testing.T) {
	plan := &PlannerResult{Platform: "macos"}
	yml := generateProjectYAML("App", plan)
	if strings.Contains(yml, "NSRequiresAquaSystemAppearance") {
		t.Error("macOS YAML should NEVER contain NSRequiresAquaSystemAppearance — macOS follows system appearance")
	}
}

func TestAppearanceLockVisionOSNoLock(t *testing.T) {
	plan := &PlannerResult{Platform: "visionos"}
	yml := generateProjectYAML("App", plan)
	if strings.Contains(yml, "UIUserInterfaceStyle") {
		t.Error("visionOS YAML should NOT contain UIUserInterfaceStyle — glass auto-adapts")
	}
	if strings.Contains(yml, "NSRequiresAquaSystemAppearance") {
		t.Error("visionOS YAML should NOT contain NSRequiresAquaSystemAppearance")
	}
}

func TestAppearanceLockMultiPlatform(t *testing.T) {
	plan := &PlannerResult{
		Platform:     "ios",
		Platforms:    []string{"ios", "macos", "tvos"},
		DeviceFamily: "iphone",
	}
	yml := generateProjectYAML("App", plan)
	if !strings.Contains(yml, "INFOPLIST_KEY_UIUserInterfaceStyle: Light") {
		t.Error("multi-platform YAML should lock iOS appearance")
	}
	if strings.Contains(yml, "NSRequiresAquaSystemAppearance") {
		t.Error("multi-platform YAML should NOT lock macOS appearance — macOS follows system")
	}
}

func TestAppearanceLockMultiPlatformVisionOSExcluded(t *testing.T) {
	plan := &PlannerResult{
		Platform:  "ios",
		Platforms: []string{"ios", "visionos"},
	}
	yml := generateProjectYAML("App", plan)
	// Count occurrences of UIUserInterfaceStyle — should only appear for iOS, not visionOS
	count := strings.Count(yml, "INFOPLIST_KEY_UIUserInterfaceStyle: Light")
	if count != 1 {
		t.Errorf("multi-platform iOS+visionOS should have exactly 1 UIUserInterfaceStyle (iOS only), got %d", count)
	}
}
