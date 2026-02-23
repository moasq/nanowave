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
		{"has watchOS platform", "platform: watchOS"},
		{"has WKWatchOnly info plist", "WKWatchOnly: true"},
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
		{"has iOS platform", "platform: iOS"},
		{"has watchOS platform", "platform: watchOS"},
		{"has watch target name", "PairedAppWatch:"},
		{"has watch bundle ID", ".watchkitapp"},
		{"has WKRunsIndependentlyOfCompanionApp", "WKRunsIndependentlyOfCompanionApp: true"},
		{"iOS depends on watch target", "target: PairedAppWatch"},
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

	// Count platform: watchOS occurrences (main app + extension)
	count := strings.Count(yml, "platform: watchOS")
	if count < 2 {
		t.Errorf("expected at least 2 'platform: watchOS' entries (app + extension), got %d", count)
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

	// Should have 3 targets: iOS app, watch app, widget extension
	if !strings.Contains(yml, "PairedWidgetApp:") {
		t.Error("should contain iOS app target")
	}
	if !strings.Contains(yml, "PairedWidgetAppWatch:") {
		t.Error("should contain watch app target")
	}
	if !strings.Contains(yml, "PairedWidgetAppWidget:") {
		t.Error("should contain widget extension target")
	}

	// Widget bundle ID should be under watchkitapp
	if !strings.Contains(yml, ".watchkitapp.widget") {
		t.Error("widget bundle ID should be under watchkitapp")
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
