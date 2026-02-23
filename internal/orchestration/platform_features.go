package orchestration

import (
	"fmt"
	"strings"
)

// Platform constants.
const (
	PlatformIOS     = "ios"
	PlatformWatchOS = "watchos"
)

// Watch project shape constants.
const (
	WatchShapeStandalone = "watch_only"
	WatchShapePaired     = "paired_ios_watch"
)

// watchOSUnsupportedRuleKeys lists rule_keys that are not supported on watchOS.
var watchOSUnsupportedRuleKeys = map[string]bool{
	"camera":             true,
	"foundation_models":  true,
	"apple_translation":  true,
	"adaptive_layout":    true,
	"liquid_glass":       true,
	"speech":             true,
	"app_review":         true,
}

// watchOSConditionalRuleKeys lists rule_keys that work differently on watchOS.
var watchOSConditionalRuleKeys = map[string]string{
	"haptics":    "watchOS uses WKInterfaceDevice.default().play(.click) instead of UIFeedbackGenerator/CoreHaptics",
	"biometrics": "watchOS uses wrist detection and optic ID instead of Face ID/Touch ID",
}

// watchOSUnsupportedExtensionKinds lists extension kinds not available on watchOS.
var watchOSUnsupportedExtensionKinds = map[string]bool{
	"live_activity":        true,
	"share":                true,
	"notification_service": true,
	"safari":               true,
	"app_clip":             true,
}

// ValidatePlatform checks that the platform string is a known value.
func ValidatePlatform(platform string) error {
	switch platform {
	case PlatformIOS, PlatformWatchOS, "":
		return nil
	default:
		return fmt.Errorf("unsupported platform %q: must be %q or %q", platform, PlatformIOS, PlatformWatchOS)
	}
}

// ValidateWatchShape checks that the watch project shape is valid.
func ValidateWatchShape(shape string) error {
	switch shape {
	case WatchShapeStandalone, WatchShapePaired, "":
		return nil
	default:
		return fmt.Errorf("unsupported watch_project_shape %q: must be %q or %q", shape, WatchShapeStandalone, WatchShapePaired)
	}
}

// IsWatchOS returns true if the platform is watchOS.
func IsWatchOS(platform string) bool {
	return platform == PlatformWatchOS
}

// FilterRuleKeysForPlatform filters rule_keys for a given platform.
// Returns the filtered keys and any validation warnings for unsupported keys.
func FilterRuleKeysForPlatform(platform string, keys []string) ([]string, []string) {
	if !IsWatchOS(platform) {
		return keys, nil
	}

	var filtered []string
	var warnings []string
	for _, key := range keys {
		if watchOSUnsupportedRuleKeys[key] {
			warnings = append(warnings, fmt.Sprintf("rule_key %q is not supported on watchOS and was removed", key))
			continue
		}
		if caveat, ok := watchOSConditionalRuleKeys[key]; ok {
			warnings = append(warnings, fmt.Sprintf("rule_key %q on watchOS: %s", key, caveat))
		}
		filtered = append(filtered, key)
	}
	return filtered, warnings
}

// ValidateExtensionsForPlatform validates that extension kinds are supported on the given platform.
func ValidateExtensionsForPlatform(platform string, extensions []ExtensionPlan) error {
	if !IsWatchOS(platform) {
		return nil
	}

	var unsupported []string
	for _, ext := range extensions {
		if watchOSUnsupportedExtensionKinds[ext.Kind] {
			unsupported = append(unsupported, ext.Kind)
		}
	}
	if len(unsupported) > 0 {
		return fmt.Errorf("watchOS does not support extension kinds: %s (only widget is supported)", strings.Join(unsupported, ", "))
	}
	return nil
}
