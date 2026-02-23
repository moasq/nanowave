package orchestration

import "testing"

func TestNormalizeWatchShapeIntentHints(t *testing.T) {
	t.Run("watch defaults to standalone and clears device family", func(t *testing.T) {
		decision := &IntentDecision{
			PlatformHint:          PlatformWatchOS,
			DeviceFamilyHint:      "iphone",
			WatchProjectShapeHint: "",
		}
		normalizeWatchShapeIntentHints(decision)
		if decision.DeviceFamilyHint != "" {
			t.Fatalf("DeviceFamilyHint = %q, want empty", decision.DeviceFamilyHint)
		}
		if decision.WatchProjectShapeHint != WatchShapeStandalone {
			t.Fatalf("WatchProjectShapeHint = %q, want %q", decision.WatchProjectShapeHint, WatchShapeStandalone)
		}
	})

	t.Run("tvos clears device family and watch shape", func(t *testing.T) {
		decision := &IntentDecision{
			PlatformHint:          PlatformTvOS,
			DeviceFamilyHint:      "iphone",
			WatchProjectShapeHint: WatchShapePaired,
		}
		normalizeWatchShapeIntentHints(decision)
		if decision.DeviceFamilyHint != "" {
			t.Fatalf("DeviceFamilyHint = %q, want empty", decision.DeviceFamilyHint)
		}
		if decision.WatchProjectShapeHint != "" {
			t.Fatalf("WatchProjectShapeHint = %q, want empty", decision.WatchProjectShapeHint)
		}
	})

	t.Run("ios clears watch shape", func(t *testing.T) {
		decision := &IntentDecision{
			PlatformHint:          PlatformIOS,
			DeviceFamilyHint:      "ipad",
			WatchProjectShapeHint: WatchShapePaired,
		}
		normalizeWatchShapeIntentHints(decision)
		if decision.WatchProjectShapeHint != "" {
			t.Fatalf("WatchProjectShapeHint = %q, want empty", decision.WatchProjectShapeHint)
		}
		if decision.DeviceFamilyHint != "ipad" {
			t.Fatalf("DeviceFamilyHint = %q, want %q", decision.DeviceFamilyHint, "ipad")
		}
	})
}
