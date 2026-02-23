package orchestration

import (
	"strings"
	"testing"
)

func TestParsePlanAcceptsEmptyExtensionKindGracefully(t *testing.T) {
	// The AI should never return empty kind (skill docs enforce this),
	// but if it does, parsePlan should not crash — the extension passes through
	// and the XcodeGen layer handles it with a safe fallback bundle ID.
	input := `{
  "platform": "ios",
  "device_family": "universal",
  "files": [
    {"path": "App/MyApp.swift", "type_name": "MyApp", "purpose": "entry", "components": "entry", "data_access": "none", "depends_on": []}
  ],
  "models": [],
  "permissions": [],
  "extensions": [
    {"kind": "", "name": "MyAppWidget", "purpose": "home screen widget"}
  ],
  "localizations": [],
  "rule_keys": [],
  "build_order": ["App/MyApp.swift"],
  "design": {"navigation":"tab","palette":{"primary":"#000","secondary":"#111","accent":"#222","background":"#FFF","surface":"#EEE"},"font_design":"default","corner_radius":12,"density":"standard","surfaces":"flat","app_mood":"calm"}
}`
	plan, err := parsePlan(input)
	if err != nil {
		t.Fatalf("parsePlan() should not crash on empty extension kind: %v", err)
	}
	if len(plan.Extensions) != 1 {
		t.Fatalf("Extensions len = %d, want 1", len(plan.Extensions))
	}
	// Kind stays empty — the AI should have set it, but we don't crash
	if plan.Extensions[0].Kind != "" {
		t.Fatalf("Extension kind = %q, want empty (no string-matching inference)", plan.Extensions[0].Kind)
	}
}

func TestParsePlan_RejectsInvalidWatchShape(t *testing.T) {
	// The AI must use exact canonical values. Variants like "watch_with_companion"
	// are rejected — the skill docs specify exactly "watch_only" or "paired_ios_watch".
	input := `{
  "platform": "watchos",
  "device_family": "",
  "watch_project_shape": "watch_with_companion",
  "files": [
    {
      "path": "TapCounterWatch/App/TapCounterWatchApp.swift",
      "type_name": "TapCounterWatchApp",
      "purpose": "Watch app entry",
      "components": "struct TapCounterWatchApp; body: some Scene",
      "data_access": "none",
      "depends_on": ["TapCounterWatch/Features/Counter/CounterView.swift"]
    }
  ],
  "models": [],
  "permissions": [],
  "extensions": [],
  "localizations": [],
  "rule_keys": [],
  "build_order": ["TapCounterWatch/App/TapCounterWatchApp.swift"]
}`

	_, err := parsePlan(input)
	if err == nil {
		t.Fatal("parsePlan() should reject invalid watch_project_shape 'watch_with_companion'")
	}
}

func TestParsePlan_AcceptsCanonicalWatchShape(t *testing.T) {
	input := `{
  "platform": "watchos",
  "device_family": "",
  "watch_project_shape": "paired_ios_watch",
  "files": [
    {
      "path": "TapCounterWatch/App/TapCounterWatchApp.swift",
      "type_name": "TapCounterWatchApp",
      "purpose": "Watch app entry",
      "components": ["struct TapCounterWatchApp", "body: some Scene"],
      "data_access": "none",
      "depends_on": "TapCounterWatch/Features/Counter/CounterView.swift"
    }
  ],
  "models": [],
  "permissions": [],
  "extensions": [],
  "localizations": [],
  "rule_keys": [],
  "build_order": ["TapCounterWatch/App/TapCounterWatchApp.swift"]
}`

	plan, err := parsePlan(input)
	if err != nil {
		t.Fatalf("parsePlan() error: %v", err)
	}
	if plan.Platform != PlatformWatchOS {
		t.Fatalf("Platform = %q, want %q", plan.Platform, PlatformWatchOS)
	}
	if plan.WatchProjectShape != WatchShapePaired {
		t.Fatalf("WatchProjectShape = %q, want %q", plan.WatchProjectShape, WatchShapePaired)
	}
	if plan.DeviceFamily != "" {
		t.Fatalf("DeviceFamily = %q, want empty for watchOS", plan.DeviceFamily)
	}
	if len(plan.Files) != 1 {
		t.Fatalf("Files len = %d, want 1", len(plan.Files))
	}
	if !strings.Contains(plan.Files[0].Components, "TapCounterWatchApp") {
		t.Fatalf("components = %q, expected joined string content", plan.Files[0].Components)
	}
	if len(plan.Files[0].DependsOn) != 1 {
		t.Fatalf("depends_on len = %d, want 1", len(plan.Files[0].DependsOn))
	}
}
