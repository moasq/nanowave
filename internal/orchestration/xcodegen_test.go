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
