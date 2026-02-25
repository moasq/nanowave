package orchestration

import (
	"fmt"
	"strings"
)

// Platform constants.
const (
	PlatformIOS      = "ios"
	PlatformWatchOS  = "watchos"
	PlatformTvOS     = "tvos"
	PlatformVisionOS = "visionos"
	PlatformMacOS    = "macos"
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

// visionOSUnsupportedRuleKeys lists rule_keys that are not supported on visionOS.
var visionOSUnsupportedRuleKeys = map[string]bool{
	"camera":     true,
	"healthkit":  true,
	"haptics":    true,
	"maps":       true,
	"speech":     true,
	"app-review": true,
	"dark-mode":  true, // visionOS has no dark mode — glass material auto-adapts
}

// visionOSConditionalRuleKeys lists rule_keys that work differently on visionOS.
var visionOSConditionalRuleKeys = map[string]string{
	"biometrics": "visionOS uses Optic ID instead of Face ID/Touch ID",
	"gestures":   "visionOS uses spatial gestures, eye tracking, and hand pinch instead of touch",
}

// visionOSUnsupportedExtensionKinds lists extension kinds not available on visionOS.
var visionOSUnsupportedExtensionKinds = map[string]bool{
	"live_activity":        true,
	"share":                true,
	"notification_service": true,
	"safari":               true,
	"app_clip":             true,
}

// watchOSUnsupportedExtensionKinds lists extension kinds not available on watchOS.
var watchOSUnsupportedExtensionKinds = map[string]bool{
	"live_activity":        true,
	"share":                true,
	"notification_service": true,
	"safari":               true,
	"app_clip":             true,
}

// macOSUnsupportedRuleKeys lists rule_keys that are not supported on macOS.
var macOSUnsupportedRuleKeys = map[string]bool{
	"healthkit": true,
	"haptics":   true,
	"speech":    true,
}

// macOSConditionalRuleKeys lists rule_keys that work differently on macOS.
var macOSConditionalRuleKeys = map[string]string{
	"biometrics": "macOS uses Touch ID (on compatible keyboards) instead of Face ID",
	"gestures":   "macOS uses trackpad, mouse, and keyboard input instead of touch",
	"camera":     "macOS has FaceTime camera only — no rear camera, no LiDAR, no portrait mode",
}

// macOSUnsupportedExtensionKinds lists extension kinds not available on macOS.
var macOSUnsupportedExtensionKinds = map[string]bool{
	"live_activity": true,
	"app_clip":      true,
	"safari":        true,
}

// ValidatePlatform checks that the platform string is a known value.
func ValidatePlatform(platform string) error {
	switch platform {
	case PlatformIOS, PlatformWatchOS, PlatformTvOS, PlatformVisionOS, PlatformMacOS, "":
		return nil
	default:
		return fmt.Errorf("unsupported platform %q: must be %q, %q, %q, %q, or %q", platform, PlatformIOS, PlatformWatchOS, PlatformTvOS, PlatformVisionOS, PlatformMacOS)
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
	case PlatformVisionOS:
		return "Vision"
	case PlatformMacOS:
		return "Mac"
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
	case PlatformVisionOS:
		return "visionOS"
	case PlatformMacOS:
		return "macOS"
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
	case PlatformVisionOS:
		return "visionOS"
	case PlatformMacOS:
		return "macOS"
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
	case PlatformVisionOS:
		return "visionOS"
	case PlatformMacOS:
		return "macOS"
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
	case PlatformVisionOS:
		return "generic/platform=visionOS Simulator"
	case PlatformMacOS:
		return "generic/platform=macOS"
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

// IsVisionOS returns true if the platform is visionOS.
func IsVisionOS(platform string) bool {
	return platform == PlatformVisionOS
}

// IsMacOS returns true if the platform is macOS.
func IsMacOS(platform string) bool {
	return platform == PlatformMacOS
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
	case IsVisionOS(platform):
		unsupported = visionOSUnsupportedRuleKeys
		conditional = visionOSConditionalRuleKeys
		platformName = "visionOS"
	case IsMacOS(platform):
		unsupported = macOSUnsupportedRuleKeys
		conditional = macOSConditionalRuleKeys
		platformName = "macOS"
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
	case IsVisionOS(platform):
		unsupportedKinds = visionOSUnsupportedExtensionKinds
		platformName = "visionOS"
		supportedNote = "only widget is supported"
	case IsMacOS(platform):
		unsupportedKinds = macOSUnsupportedExtensionKinds
		platformName = "macOS"
		supportedNote = "widget, share, and notification_service are supported"
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
