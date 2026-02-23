package orchestration

import (
	"fmt"
	"strings"
)

// Platform constants.
const (
	PlatformIOS     = "ios"
	PlatformWatchOS = "watchos"
	PlatformTvOS    = "tvos"
)

// Watch project shape constants.
const (
	WatchShapeStandalone = "watch_only"
	WatchShapePaired     = "paired_ios_watch"
)

// watchOSUnsupportedRuleKeys lists rule_keys that are not supported on watchOS.
var watchOSUnsupportedRuleKeys = map[string]bool{
	"camera":            true,
	"foundation-models": true,
	"apple-translation": true,
	"adaptive-layout":   true,
	"liquid-glass":      true,
	"speech":            true,
	"app-review":        true,
}

// watchOSConditionalRuleKeys lists rule_keys that work differently on watchOS.
var watchOSConditionalRuleKeys = map[string]string{
	"haptics":    "watchOS uses WKInterfaceDevice.default().play(.click) instead of UIFeedbackGenerator/CoreHaptics",
	"biometrics": "watchOS uses wrist detection and optic ID instead of Face ID/Touch ID",
}

// tvOSUnsupportedRuleKeys lists rule_keys that are not supported on tvOS.
var tvOSUnsupportedRuleKeys = map[string]bool{
	"camera":            true,
	"biometrics":        true,
	"healthkit":         true,
	"haptics":           true,
	"maps":              true,
	"speech":            true,
	"apple-translation": true,
}

// tvOSConditionalRuleKeys lists rule_keys that work differently on tvOS.
var tvOSConditionalRuleKeys = map[string]string{
	"gestures":   "tvOS uses Siri Remote input (onMoveCommand, onPlayPauseCommand, onExitCommand) instead of touch gestures",
	"animations": "tvOS animations should account for focus transitions and Siri Remote parallax effects",
}

// tvOSUnsupportedExtensionKinds lists extension kinds not available on tvOS.
var tvOSUnsupportedExtensionKinds = map[string]bool{
	"live_activity":        true,
	"share":                true,
	"notification_service": true,
	"safari":               true,
	"app_clip":             true,
	"widget":               true,
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
	case PlatformIOS, PlatformWatchOS, PlatformTvOS, "":
		return nil
	default:
		return fmt.Errorf("unsupported platform %q: must be %q, %q, or %q", platform, PlatformIOS, PlatformWatchOS, PlatformTvOS)
	}
}

// ValidatePlatforms validates a list of platform strings. Invalid entries are dropped.
// Returns only valid platforms. If all are invalid, returns nil (caller should fall back to defaults).
func ValidatePlatforms(platforms []string) []string {
	var valid []string
	seen := map[string]bool{}
	for _, p := range platforms {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if err := ValidatePlatform(p); err != nil {
			continue
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		valid = append(valid, p)
	}
	return valid
}

// PlatformSourceDirSuffix returns the source directory suffix for a platform in
// a multi-platform project. iOS uses "" (the app name dir), watchOS uses "Watch",
// tvOS uses "TV".
func PlatformSourceDirSuffix(platform string) string {
	switch platform {
	case PlatformWatchOS:
		return "Watch"
	case PlatformTvOS:
		return "TV"
	default:
		return ""
	}
}

// PlatformDisplayName returns a human-friendly name for a platform constant.
func PlatformDisplayName(platform string) string {
	switch platform {
	case PlatformWatchOS:
		return "watchOS"
	case PlatformTvOS:
		return "tvOS"
	default:
		return "iOS"
	}
}

// PlatformDeploymentTargetKey returns the XcodeGen deployment target key.
func PlatformDeploymentTargetKey(platform string) string {
	switch platform {
	case PlatformWatchOS:
		return "watchOS"
	case PlatformTvOS:
		return "tvOS"
	default:
		return "iOS"
	}
}

// PlatformXcodegenValue returns the XcodeGen platform value.
func PlatformXcodegenValue(platform string) string {
	switch platform {
	case PlatformWatchOS:
		return "watchOS"
	case PlatformTvOS:
		return "tvOS"
	default:
		return "iOS"
	}
}

// HasPlatform returns true if the given platform is in the list.
func HasPlatform(platforms []string, platform string) bool {
	for _, p := range platforms {
		if p == platform {
			return true
		}
	}
	return false
}

// PlatformBuildDestination returns the Xcode build destination for a single platform.
func PlatformBuildDestination(platform string) string {
	switch platform {
	case PlatformWatchOS:
		return "generic/platform=watchOS Simulator"
	case PlatformTvOS:
		return "generic/platform=tvOS Simulator"
	default:
		return "generic/platform=iOS Simulator"
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

// IsTvOS returns true if the platform is tvOS.
func IsTvOS(platform string) bool {
	return platform == PlatformTvOS
}

// FilterRuleKeysForPlatform filters rule_keys for a given platform.
// Returns the filtered keys and any validation warnings for unsupported keys.
func FilterRuleKeysForPlatform(platform string, keys []string) ([]string, []string) {
	var unsupported map[string]bool
	var conditional map[string]string
	var platformName string

	switch {
	case IsWatchOS(platform):
		unsupported = watchOSUnsupportedRuleKeys
		conditional = watchOSConditionalRuleKeys
		platformName = "watchOS"
	case IsTvOS(platform):
		unsupported = tvOSUnsupportedRuleKeys
		conditional = tvOSConditionalRuleKeys
		platformName = "tvOS"
	default:
		return keys, nil
	}

	var filtered []string
	var warnings []string
	for _, key := range keys {
		if unsupported[key] {
			warnings = append(warnings, fmt.Sprintf("rule_key %q is not supported on %s and was removed", key, platformName))
			continue
		}
		if caveat, ok := conditional[key]; ok {
			warnings = append(warnings, fmt.Sprintf("rule_key %q on %s: %s", key, platformName, caveat))
		}
		filtered = append(filtered, key)
	}
	return filtered, warnings
}

// ValidateExtensionsForPlatform validates that extension kinds are supported on the given platform.
func ValidateExtensionsForPlatform(platform string, extensions []ExtensionPlan) error {
	var unsupportedKinds map[string]bool
	var platformName, supportedNote string

	switch {
	case IsWatchOS(platform):
		unsupportedKinds = watchOSUnsupportedExtensionKinds
		platformName = "watchOS"
		supportedNote = "only widget is supported"
	case IsTvOS(platform):
		unsupportedKinds = tvOSUnsupportedExtensionKinds
		platformName = "tvOS"
		supportedNote = "only tv-top-shelf is supported"
	default:
		return nil
	}

	var unsupported []string
	for _, ext := range extensions {
		if unsupportedKinds[ext.Kind] {
			unsupported = append(unsupported, ext.Kind)
		}
	}
	if len(unsupported) > 0 {
		return fmt.Errorf("%s does not support extension kinds: %s (%s)", platformName, strings.Join(unsupported, ", "), supportedNote)
	}
	return nil
}
