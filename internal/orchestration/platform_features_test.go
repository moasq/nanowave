package orchestration

import (
	"strings"
	"testing"
)

func TestValidatePlatform(t *testing.T) {
	tests := []struct {
		platform string
		wantErr  bool
	}{
		{"ios", false},
		{"watchos", false},
		{"", false},
		{"macos", false},
		{"tvos", false},
		{"visionos", false},
	}

	for _, tc := range tests {
		t.Run(tc.platform, func(t *testing.T) {
			err := ValidatePlatform(tc.platform)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidatePlatform(%q) error = %v, wantErr %v", tc.platform, err, tc.wantErr)
			}
		})
	}
}

func TestValidateWatchShape(t *testing.T) {
	tests := []struct {
		shape   string
		wantErr bool
	}{
		{"watch_only", false},
		{"paired_ios_watch", false},
		{"", false},
		{"invalid", true},
		{"standalone", true},
	}

	for _, tc := range tests {
		t.Run(tc.shape, func(t *testing.T) {
			err := ValidateWatchShape(tc.shape)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateWatchShape(%q) error = %v, wantErr %v", tc.shape, err, tc.wantErr)
			}
		})
	}
}

func TestIsWatchOS(t *testing.T) {
	if !IsWatchOS("watchos") {
		t.Error("IsWatchOS(watchos) should be true")
	}
	if IsWatchOS("ios") {
		t.Error("IsWatchOS(ios) should be false")
	}
	if IsWatchOS("") {
		t.Error("IsWatchOS('') should be false")
	}
}

func TestFilterRuleKeysForPlatformIOS(t *testing.T) {
	keys := []string{"camera", "haptics", "storage", "widgets"}
	filtered, warnings := FilterRuleKeysForPlatform("ios", keys)
	if len(filtered) != len(keys) {
		t.Errorf("iOS should not filter any keys, got %d instead of %d", len(filtered), len(keys))
	}
	if len(warnings) != 0 {
		t.Errorf("iOS should produce no warnings, got %d", len(warnings))
	}
}

func TestFilterRuleKeysForPlatformWatchOS(t *testing.T) {
	keys := []string{"camera", "haptics", "storage", "widgets", "foundation-models", "biometrics"}
	filtered, warnings := FilterRuleKeysForPlatform("watchos", keys)

	// camera and foundation-models should be removed
	for _, f := range filtered {
		if f == "camera" || f == "foundation-models" {
			t.Errorf("watchOS should remove %q from keys", f)
		}
	}

	// haptics and biometrics should remain (conditional)
	foundHaptics := false
	foundBiometrics := false
	for _, f := range filtered {
		if f == "haptics" {
			foundHaptics = true
		}
		if f == "biometrics" {
			foundBiometrics = true
		}
	}
	if !foundHaptics {
		t.Error("haptics should remain in filtered keys (conditional)")
	}
	if !foundBiometrics {
		t.Error("biometrics should remain in filtered keys (conditional)")
	}

	// storage and widgets should remain (supported)
	foundStorage := false
	foundWidgets := false
	for _, f := range filtered {
		if f == "storage" {
			foundStorage = true
		}
		if f == "widgets" {
			foundWidgets = true
		}
	}
	if !foundStorage {
		t.Error("storage should remain in filtered keys")
	}
	if !foundWidgets {
		t.Error("widgets should remain in filtered keys")
	}

	// Warnings for unsupported + conditional
	if len(warnings) < 2 {
		t.Errorf("expected at least 2 warnings (camera removed + haptics/biometrics conditional), got %d", len(warnings))
	}

	// Check warning content
	hasRemovedWarning := false
	hasConditionalWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "camera") && strings.Contains(w, "removed") {
			hasRemovedWarning = true
		}
		if strings.Contains(w, "haptics") || strings.Contains(w, "biometrics") {
			hasConditionalWarning = true
		}
	}
	if !hasRemovedWarning {
		t.Error("should have a warning about camera being removed")
	}
	if !hasConditionalWarning {
		t.Error("should have a conditional warning about haptics or biometrics")
	}
}

func TestFilterRuleKeysAllUnsupported(t *testing.T) {
	keys := []string{"camera", "foundation-models", "apple-translation", "adaptive-layout", "liquid-glass", "speech", "app-review"}
	filtered, warnings := FilterRuleKeysForPlatform("watchos", keys)

	if len(filtered) != 0 {
		t.Errorf("all keys should be filtered out, got %d remaining", len(filtered))
	}
	if len(warnings) != len(keys) {
		t.Errorf("expected %d warnings, got %d", len(keys), len(warnings))
	}
}

func TestValidateExtensionsForPlatformIOS(t *testing.T) {
	extensions := []ExtensionPlan{
		{Kind: "widget"},
		{Kind: "live_activity"},
		{Kind: "share"},
	}
	err := ValidateExtensionsForPlatform("ios", extensions)
	if err != nil {
		t.Errorf("iOS should support all extension kinds, got: %v", err)
	}
}

func TestIsTvOS(t *testing.T) {
	if !IsTvOS("tvos") {
		t.Error("IsTvOS(tvos) should be true")
	}
	if IsTvOS("ios") {
		t.Error("IsTvOS(ios) should be false")
	}
	if IsTvOS("") {
		t.Error("IsTvOS('') should be false")
	}
}

func TestFilterRuleKeysForPlatformTvOS(t *testing.T) {
	keys := []string{"camera", "biometrics", "healthkit", "haptics", "maps", "speech", "apple-translation", "gestures", "animations", "storage"}
	filtered, warnings := FilterRuleKeysForPlatform("tvos", keys)

	// camera, biometrics, healthkit, haptics, maps, speech, apple-translation should be removed
	unsupported := map[string]bool{
		"camera": true, "biometrics": true, "healthkit": true,
		"haptics": true, "maps": true, "speech": true, "apple-translation": true,
	}
	for _, f := range filtered {
		if unsupported[f] {
			t.Errorf("tvOS should remove %q from keys", f)
		}
	}

	// gestures and animations should remain (conditional)
	foundGestures := false
	foundAnimations := false
	for _, f := range filtered {
		if f == "gestures" {
			foundGestures = true
		}
		if f == "animations" {
			foundAnimations = true
		}
	}
	if !foundGestures {
		t.Error("gestures should remain in filtered keys (conditional)")
	}
	if !foundAnimations {
		t.Error("animations should remain in filtered keys (conditional)")
	}

	// storage should remain (supported)
	foundStorage := false
	for _, f := range filtered {
		if f == "storage" {
			foundStorage = true
		}
	}
	if !foundStorage {
		t.Error("storage should remain in filtered keys")
	}

	// Check warnings
	hasRemovedWarning := false
	hasConditionalWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "removed") {
			hasRemovedWarning = true
		}
		if strings.Contains(w, "gestures") || strings.Contains(w, "animations") {
			hasConditionalWarning = true
		}
	}
	if !hasRemovedWarning {
		t.Error("should have removal warnings for tvOS unsupported keys")
	}
	if !hasConditionalWarning {
		t.Error("should have conditional warnings for gestures or animations")
	}
}

func TestValidateExtensionsForPlatformTvOS(t *testing.T) {
	// All standard extension kinds should be rejected on tvOS
	unsupported := []string{"live_activity", "share", "notification_service", "safari", "app_clip", "widget"}
	for _, kind := range unsupported {
		err := ValidateExtensionsForPlatform("tvos", []ExtensionPlan{{Kind: kind}})
		if err == nil {
			t.Errorf("tvOS should reject extension kind %q", kind)
		}
	}
}

func TestValidatePlatforms(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"all valid", []string{"ios", "watchos", "tvos", "visionos", "macos"}, []string{"ios", "watchos", "tvos", "visionos", "macos"}},
		{"drops invalid", []string{"ios", "android", "tvos"}, []string{"ios", "tvos"}},
		{"empty input", nil, nil},
		{"all invalid", []string{"android", "linux"}, nil},
		{"single valid", []string{"watchos"}, []string{"watchos"}},
		{"macos valid", []string{"ios", "macos"}, []string{"ios", "macos"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidatePlatforms(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("ValidatePlatforms(%v) = %v, want %v", tc.input, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("ValidatePlatforms(%v)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestPlatformSourceDirSuffix(t *testing.T) {
	tests := []struct {
		platform string
		want     string
	}{
		{"ios", ""},
		{"watchos", "Watch"},
		{"tvos", "TV"},
		{"", ""},
	}
	for _, tc := range tests {
		t.Run(tc.platform, func(t *testing.T) {
			got := PlatformSourceDirSuffix(tc.platform)
			if got != tc.want {
				t.Fatalf("PlatformSourceDirSuffix(%q) = %q, want %q", tc.platform, got, tc.want)
			}
		})
	}
}

func TestHasPlatform(t *testing.T) {
	platforms := []string{"ios", "watchos", "tvos"}
	if !HasPlatform(platforms, "watchos") {
		t.Error("HasPlatform should find watchos")
	}
	if HasPlatform(platforms, "macos") {
		t.Error("HasPlatform should not find macos")
	}
	if HasPlatform(nil, "ios") {
		t.Error("HasPlatform(nil) should return false")
	}
}

func TestPlatformBuildDestination(t *testing.T) {
	tests := []struct {
		platform string
		wantSub  string
	}{
		{"ios", "iOS Simulator"},
		{"watchos", "watchOS Simulator"},
		{"tvos", "tvOS Simulator"},
	}
	for _, tc := range tests {
		t.Run(tc.platform, func(t *testing.T) {
			got := PlatformBuildDestination(tc.platform)
			if !strings.Contains(got, tc.wantSub) {
				t.Fatalf("PlatformBuildDestination(%q) = %q, want to contain %q", tc.platform, got, tc.wantSub)
			}
		})
	}
}

func TestValidateExtensionsForPlatformWatchOS(t *testing.T) {
	// Widget is supported
	err := ValidateExtensionsForPlatform("watchos", []ExtensionPlan{{Kind: "widget"}})
	if err != nil {
		t.Errorf("watchOS should support widget, got: %v", err)
	}

	// Other kinds are not
	unsupported := []string{"live_activity", "share", "notification_service", "safari", "app_clip"}
	for _, kind := range unsupported {
		err := ValidateExtensionsForPlatform("watchos", []ExtensionPlan{{Kind: kind}})
		if err == nil {
			t.Errorf("watchOS should reject extension kind %q", kind)
		}
	}
}

func TestIsVisionOS(t *testing.T) {
	if !IsVisionOS("visionos") {
		t.Error("IsVisionOS(visionos) should be true")
	}
	if IsVisionOS("ios") {
		t.Error("IsVisionOS(ios) should be false")
	}
	if IsVisionOS("") {
		t.Error("IsVisionOS('') should be false")
	}
}

func TestFilterRuleKeysForPlatformVisionOS(t *testing.T) {
	keys := []string{"camera", "healthkit", "haptics", "maps", "speech", "app-review", "dark-mode", "biometrics", "gestures", "storage"}
	filtered, warnings := FilterRuleKeysForPlatform("visionos", keys)

	// camera, healthkit, haptics, maps, speech, app-review, dark-mode should be removed
	unsupported := map[string]bool{
		"camera": true, "healthkit": true, "haptics": true,
		"maps": true, "speech": true, "app-review": true, "dark-mode": true,
	}
	for _, f := range filtered {
		if unsupported[f] {
			t.Errorf("visionOS should remove %q from keys", f)
		}
	}

	// biometrics and gestures should remain (conditional)
	foundBiometrics := false
	foundGestures := false
	for _, f := range filtered {
		if f == "biometrics" {
			foundBiometrics = true
		}
		if f == "gestures" {
			foundGestures = true
		}
	}
	if !foundBiometrics {
		t.Error("biometrics should remain in filtered keys (conditional)")
	}
	if !foundGestures {
		t.Error("gestures should remain in filtered keys (conditional)")
	}

	// storage should remain (supported)
	foundStorage := false
	for _, f := range filtered {
		if f == "storage" {
			foundStorage = true
		}
	}
	if !foundStorage {
		t.Error("storage should remain in filtered keys")
	}

	// Check warnings
	hasRemovedWarning := false
	hasConditionalWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "removed") {
			hasRemovedWarning = true
		}
		if strings.Contains(w, "biometrics") || strings.Contains(w, "gestures") {
			hasConditionalWarning = true
		}
	}
	if !hasRemovedWarning {
		t.Error("should have removal warnings for visionOS unsupported keys")
	}
	if !hasConditionalWarning {
		t.Error("should have conditional warnings for biometrics or gestures")
	}
}

func TestValidateExtensionsForPlatformVisionOS(t *testing.T) {
	// Widget is supported
	err := ValidateExtensionsForPlatform("visionos", []ExtensionPlan{{Kind: "widget"}})
	if err != nil {
		t.Errorf("visionOS should support widget, got: %v", err)
	}

	// Other kinds are not
	unsupported := []string{"live_activity", "share", "notification_service", "safari", "app_clip"}
	for _, kind := range unsupported {
		err := ValidateExtensionsForPlatform("visionos", []ExtensionPlan{{Kind: kind}})
		if err == nil {
			t.Errorf("visionOS should reject extension kind %q", kind)
		}
	}
}

func TestPlatformSourceDirSuffixVisionOS(t *testing.T) {
	got := PlatformSourceDirSuffix("visionos")
	if got != "Vision" {
		t.Fatalf("PlatformSourceDirSuffix(visionos) = %q, want %q", got, "Vision")
	}
}

func TestPlatformBuildDestinationVisionOS(t *testing.T) {
	got := PlatformBuildDestination("visionos")
	if !strings.Contains(got, "visionOS Simulator") {
		t.Fatalf("PlatformBuildDestination(visionos) = %q, want to contain %q", got, "visionOS Simulator")
	}
}

func TestIsMacOS(t *testing.T) {
	if !IsMacOS("macos") {
		t.Error("IsMacOS(macos) should be true")
	}
	if IsMacOS("ios") {
		t.Error("IsMacOS(ios) should be false")
	}
	if IsMacOS("") {
		t.Error("IsMacOS('') should be false")
	}
}

func TestFilterRuleKeysForPlatformMacOS(t *testing.T) {
	keys := []string{"healthkit", "haptics", "speech", "biometrics", "gestures", "camera", "storage", "widgets"}
	filtered, warnings := FilterRuleKeysForPlatform("macos", keys)

	// healthkit, haptics, speech should be removed
	unsupported := map[string]bool{
		"healthkit": true, "haptics": true, "speech": true,
	}
	for _, f := range filtered {
		if unsupported[f] {
			t.Errorf("macOS should remove %q from keys", f)
		}
	}

	// biometrics, gestures, camera should remain (conditional)
	conditional := map[string]bool{"biometrics": true, "gestures": true, "camera": true}
	for key := range conditional {
		found := false
		for _, f := range filtered {
			if f == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s should remain in filtered keys (conditional)", key)
		}
	}

	// storage and widgets should remain (supported)
	for _, key := range []string{"storage", "widgets"} {
		found := false
		for _, f := range filtered {
			if f == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s should remain in filtered keys", key)
		}
	}

	// Check warnings
	hasRemovedWarning := false
	hasConditionalWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "removed") {
			hasRemovedWarning = true
		}
		if strings.Contains(w, "biometrics") || strings.Contains(w, "gestures") || strings.Contains(w, "camera") {
			hasConditionalWarning = true
		}
	}
	if !hasRemovedWarning {
		t.Error("should have removal warnings for macOS unsupported keys")
	}
	if !hasConditionalWarning {
		t.Error("should have conditional warnings for biometrics, gestures, or camera")
	}
}

func TestValidateExtensionsForPlatformMacOS(t *testing.T) {
	// Widget, share, and notification_service are supported
	supported := []string{"widget", "share", "notification_service"}
	for _, kind := range supported {
		err := ValidateExtensionsForPlatform("macos", []ExtensionPlan{{Kind: kind}})
		if err != nil {
			t.Errorf("macOS should support extension kind %q, got: %v", kind, err)
		}
	}

	// live_activity, app_clip, safari are not
	unsupported := []string{"live_activity", "app_clip", "safari"}
	for _, kind := range unsupported {
		err := ValidateExtensionsForPlatform("macos", []ExtensionPlan{{Kind: kind}})
		if err == nil {
			t.Errorf("macOS should reject extension kind %q", kind)
		}
	}
}

func TestPlatformSourceDirSuffixMacOS(t *testing.T) {
	got := PlatformSourceDirSuffix("macos")
	if got != "Mac" {
		t.Fatalf("PlatformSourceDirSuffix(macos) = %q, want %q", got, "Mac")
	}
}

func TestPlatformBuildDestinationMacOS(t *testing.T) {
	got := PlatformBuildDestination("macos")
	if got != "generic/platform=macOS" {
		t.Fatalf("PlatformBuildDestination(macos) = %q, want %q", got, "generic/platform=macOS")
	}
}

func TestPlatformDisplayNameMacOS(t *testing.T) {
	got := PlatformDisplayName("macos")
	if got != "macOS" {
		t.Fatalf("PlatformDisplayName(macos) = %q, want %q", got, "macOS")
	}
}

func TestPlatformDeploymentTargetKeyMacOS(t *testing.T) {
	got := PlatformDeploymentTargetKey("macos")
	if got != "macOS" {
		t.Fatalf("PlatformDeploymentTargetKey(macos) = %q, want %q", got, "macOS")
	}
}

func TestPlatformXcodegenValueMacOS(t *testing.T) {
	got := PlatformXcodegenValue("macos")
	if got != "macOS" {
		t.Fatalf("PlatformXcodegenValue(macos) = %q, want %q", got, "macOS")
	}
}
