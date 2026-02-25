package orchestration

import (
	"strings"
	"testing"
)

func TestPlannerResultGetPlatforms(t *testing.T) {
	tests := []struct {
		name string
		plan *PlannerResult
		want []string
	}{
		{"nil plan", nil, []string{"ios"}},
		{"empty platforms falls back to Platform", &PlannerResult{Platform: "watchos"}, []string{"watchos"}},
		{"empty platform defaults to ios", &PlannerResult{}, []string{"ios"}},
		{"multi-platform returns Platforms", &PlannerResult{Platforms: []string{"ios", "watchos", "tvos"}, Platform: "ios"}, []string{"ios", "watchos", "tvos"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.plan.GetPlatforms()
			if len(got) != len(tc.want) {
				t.Fatalf("GetPlatforms() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("GetPlatforms()[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestPlannerResultIsMultiPlatform(t *testing.T) {
	single := &PlannerResult{Platform: "ios"}
	if single.IsMultiPlatform() {
		t.Error("single platform should not be multi-platform")
	}

	multi := &PlannerResult{Platforms: []string{"ios", "watchos"}}
	if !multi.IsMultiPlatform() {
		t.Error("two platforms should be multi-platform")
	}
}

func TestFinalizeBuildIntentDecision(t *testing.T) {
	tests := []struct {
		name         string
		parsed       *IntentDecision
		wantOp       string
		wantPlatform string
		wantDevice   string
		wantShape    string
	}{
		{
			name:         "nil parsed uses defaults",
			parsed:       nil,
			wantOp:       "build",
			wantPlatform: PlatformIOS,
			wantDevice:   "iphone",
			wantShape:    "",
		},
		{
			name:         "watch defaults to standalone shape",
			parsed:       &IntentDecision{Operation: "build", PlatformHint: PlatformWatchOS, Confidence: 0.8, Reason: "watch app"},
			wantOp:       "build",
			wantPlatform: PlatformWatchOS,
			wantDevice:   "",
			wantShape:    WatchShapeStandalone,
		},
		{
			name:         "watch preserves paired shape",
			parsed:       &IntentDecision{Operation: "build", PlatformHint: PlatformWatchOS, WatchProjectShapeHint: WatchShapePaired, Confidence: 0.9, Reason: "watch + companion"},
			wantOp:       "build",
			wantPlatform: PlatformWatchOS,
			wantDevice:   "",
			wantShape:    WatchShapePaired,
		},
		{
			name:         "ios clears watch shape and fills device default",
			parsed:       &IntentDecision{Operation: "build", PlatformHint: PlatformIOS, WatchProjectShapeHint: WatchShapePaired, Confidence: 0.7, Reason: "ios app"},
			wantOp:       "build",
			wantPlatform: PlatformIOS,
			wantDevice:   "iphone",
			wantShape:    "",
		},
		{
			name:         "unknown operation normalizes to build",
			parsed:       &IntentDecision{Operation: "unknown", PlatformHint: PlatformIOS},
			wantOp:       "build",
			wantPlatform: PlatformIOS,
			wantDevice:   "iphone",
			wantShape:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := finalizeBuildIntentDecision(tc.parsed, defaultBuildIntentDecision())
			if got == nil {
				t.Fatal("got nil decision")
			}
			if got.Operation != tc.wantOp {
				t.Fatalf("Operation = %q, want %q", got.Operation, tc.wantOp)
			}
			if got.PlatformHint != tc.wantPlatform {
				t.Fatalf("PlatformHint = %q, want %q", got.PlatformHint, tc.wantPlatform)
			}
			if got.DeviceFamilyHint != tc.wantDevice {
				t.Fatalf("DeviceFamilyHint = %q, want %q", got.DeviceFamilyHint, tc.wantDevice)
			}
			if got.WatchProjectShapeHint != tc.wantShape {
				t.Fatalf("WatchProjectShapeHint = %q, want %q", got.WatchProjectShapeHint, tc.wantShape)
			}
		})
	}
}

func TestParseIntentDecision(t *testing.T) {
	good := `{"operation":"build","platform_hint":"watchos","device_family_hint":"","watch_project_shape_hint":"watch_only","confidence":1.4,"reason":"explicit watch app","used_llm":true}`
	decision, err := parseIntentDecision(good)
	if err != nil {
		t.Fatalf("parseIntentDecision() error: %v", err)
	}
	if decision.Confidence != 1 {
		t.Fatalf("confidence clamp = %v, want 1", decision.Confidence)
	}
	if decision.PlatformHint != PlatformWatchOS {
		t.Fatalf("platform = %q", decision.PlatformHint)
	}

	// macOS is now a recognized platform
	macosJSON := `{"operation":"build","platform_hint":"macos"}`
	macosDecision, err := parseIntentDecision(macosJSON)
	if err != nil {
		t.Fatalf("parseIntentDecision() should not error for macos: %v", err)
	}
	if macosDecision.PlatformHint != PlatformMacOS {
		t.Fatalf("macos platform should be preserved, got %q", macosDecision.PlatformHint)
	}

	// Unrecognized platform falls back to iOS gracefully (no error)
	unknown := `{"operation":"build","platform_hint":"android"}`
	unknownDecision, err := parseIntentDecision(unknown)
	if err != nil {
		t.Fatalf("parseIntentDecision() should not error for unrecognized platform (graceful fallback): %v", err)
	}
	if unknownDecision.PlatformHint != PlatformIOS {
		t.Fatalf("unrecognized platform should fall back to iOS, got %q", unknownDecision.PlatformHint)
	}
}

func TestParseIntentDecisionUnrecognizedPlatformFallsBackToIOS(t *testing.T) {
	// When the AI returns an unrecognized platform_hint, we gracefully fall back to iOS
	// rather than crashing. The skill docs instruct the AI to return valid values,
	// but graceful degradation is important.
	raw := `{"operation":"build","platform_hint":"multiplatform","device_family_hint":"universal","confidence":0.8,"reason":"mentions all platforms"}`
	decision, err := parseIntentDecision(raw)
	if err != nil {
		t.Fatalf("parseIntentDecision() error: %v", err)
	}
	if decision.PlatformHint != PlatformIOS {
		t.Fatalf("PlatformHint = %q, want %q (unrecognized value should fall back to ios)", decision.PlatformHint, PlatformIOS)
	}
}

func TestFinalizeBuildIntentDecisionMultiPlatform(t *testing.T) {
	parsed := &IntentDecision{
		Operation:     "build",
		PlatformHints: []string{"ios", "watchos", "tvos"},
		Confidence:    0.9,
		Reason:        "multi-platform request",
	}
	got := finalizeBuildIntentDecision(parsed, defaultBuildIntentDecision())
	if got.PlatformHint != "ios" {
		t.Fatalf("PlatformHint = %q, want ios (first entry from PlatformHints)", got.PlatformHint)
	}
	if len(got.PlatformHints) != 3 {
		t.Fatalf("PlatformHints len = %d, want 3", len(got.PlatformHints))
	}
}

func TestParseIntentDecisionWithPlatformHints(t *testing.T) {
	raw := `{"operation":"build","platform_hints":["ios","watchos","tvos"],"confidence":0.9,"reason":"multi"}`
	decision, err := parseIntentDecision(raw)
	if err != nil {
		t.Fatalf("parseIntentDecision() error: %v", err)
	}
	if len(decision.PlatformHints) != 3 {
		t.Fatalf("PlatformHints len = %d, want 3", len(decision.PlatformHints))
	}
	if decision.PlatformHint != "ios" {
		t.Fatalf("PlatformHint = %q, want ios (set from first PlatformHints entry)", decision.PlatformHint)
	}
}

func TestParseIntentDecisionPlatformHintsDropsInvalid(t *testing.T) {
	raw := `{"operation":"build","platform_hints":["ios","android","tvos"],"confidence":0.8,"reason":"mixed"}`
	decision, err := parseIntentDecision(raw)
	if err != nil {
		t.Fatalf("parseIntentDecision() error: %v", err)
	}
	if len(decision.PlatformHints) != 2 {
		t.Fatalf("PlatformHints len = %d, want 2 (android dropped)", len(decision.PlatformHints))
	}
}

func TestParseIntentDecisionPlatformHintsKeepsVisionOS(t *testing.T) {
	raw := `{"operation":"build","platform_hints":["ios","visionos"],"confidence":0.9,"reason":"spatial"}`
	decision, err := parseIntentDecision(raw)
	if err != nil {
		t.Fatalf("parseIntentDecision() error: %v", err)
	}
	if len(decision.PlatformHints) != 2 {
		t.Fatalf("PlatformHints len = %d, want 2 (visionos should be kept)", len(decision.PlatformHints))
	}
	found := false
	for _, p := range decision.PlatformHints {
		if p == "visionos" {
			found = true
		}
	}
	if !found {
		t.Fatal("PlatformHints should contain visionos")
	}
}

func TestFormatIntentHintsMultiPlatform(t *testing.T) {
	intent := &IntentDecision{
		PlatformHint:  "ios",
		PlatformHints: []string{"ios", "watchos", "tvos"},
		Operation:     "build",
		Confidence:    0.9,
		Reason:        "multi-platform request",
	}
	hints := formatIntentHintsForPrompt(intent)
	if !strings.Contains(hints, "platform_hints: [ios, watchos, tvos]") {
		t.Fatalf("expected platform_hints line, got:\n%s", hints)
	}
}

func TestParseIntentDecisionPlatformHintsKeepsMacOS(t *testing.T) {
	raw := `{"operation":"build","platform_hints":["ios","macos"],"confidence":0.9,"reason":"cross-platform"}`
	decision, err := parseIntentDecision(raw)
	if err != nil {
		t.Fatalf("parseIntentDecision() error: %v", err)
	}
	if len(decision.PlatformHints) != 2 {
		t.Fatalf("PlatformHints len = %d, want 2 (macos should be kept)", len(decision.PlatformHints))
	}
	found := false
	for _, p := range decision.PlatformHints {
		if p == "macos" {
			found = true
		}
	}
	if !found {
		t.Fatal("PlatformHints should contain macos")
	}
}

func TestFinalizeBuildIntentDecisionMultiPlatformWithMacOS(t *testing.T) {
	parsed := &IntentDecision{
		Operation:     "build",
		PlatformHints: []string{"ios", "watchos", "macos"},
		Confidence:    0.9,
		Reason:        "multi-platform with mac",
	}
	got := finalizeBuildIntentDecision(parsed, defaultBuildIntentDecision())
	if got.PlatformHint != "ios" {
		t.Fatalf("PlatformHint = %q, want ios (first entry from PlatformHints)", got.PlatformHint)
	}
	if len(got.PlatformHints) != 3 {
		t.Fatalf("PlatformHints len = %d, want 3", len(got.PlatformHints))
	}
}

func TestParseIntentDecisionUnrecognizedOperationFallsBackToBuild(t *testing.T) {
	// The AI should return exact canonical values. If it returns something
	// unrecognized like "create_app", we gracefully fall back to "build".
	raw := `{"operation":"create_app","platform_hint":"watchos","watch_project_shape_hint":"paired_ios_watch","confidence":0.9,"reason":"paired watch app"}`
	decision, err := parseIntentDecision(raw)
	if err != nil {
		t.Fatalf("parseIntentDecision() error: %v", err)
	}
	if decision.Operation != "build" {
		t.Fatalf("Operation = %q, want build (unrecognized should fall back to build)", decision.Operation)
	}
	if decision.PlatformHint != PlatformWatchOS {
		t.Fatalf("PlatformHint = %q, want %q", decision.PlatformHint, PlatformWatchOS)
	}
	if decision.WatchProjectShapeHint != WatchShapePaired {
		t.Fatalf("WatchProjectShapeHint = %q, want %q", decision.WatchProjectShapeHint, WatchShapePaired)
	}
}
