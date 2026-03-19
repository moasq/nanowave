package orchestration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/moasq/nanowave/internal/skills"
)

// skillsFS aliases the skills package FS for backward compatibility within orchestration.
var skillsFS = skills.FS

func platformSummary(platform, deviceFamily string) string {
	if IsWatchOS(platform) {
		return "Apple Watch, watchOS 26+, Swift 6"
	}
	if IsTvOS(platform) {
		return "Apple TV, tvOS 26+, Swift 6"
	}
	if IsVisionOS(platform) {
		return "Apple Vision Pro, visionOS 26+, Swift 6"
	}
	if IsMacOS(platform) {
		return "Mac, macOS 26+, Swift 6"
	}
	switch deviceFamily {
	case "ipad":
		return "iPad only, iOS 26+, Swift 6"
	case "universal":
		return "iPhone and iPad, iOS 26+, Swift 6"
	default:
		return "iPhone only, iOS 26+, Swift 6"
	}
}

// canonicalBuildDestinationForShape returns the generic device destination for a platform.
func canonicalBuildDestinationForShape(platform, watchProjectShape string) string {
	if IsWatchOS(platform) {
		if watchProjectShape == WatchShapePaired {
			return "generic/platform=iOS"
		}
		return "generic/platform=watchOS"
	}
	if IsTvOS(platform) {
		return "generic/platform=tvOS"
	}
	if IsVisionOS(platform) {
		return "generic/platform=visionOS"
	}
	if IsMacOS(platform) {
		return "generic/platform=macOS"
	}
	return "generic/platform=iOS"
}

// canonicalSimulatorBuildDestination returns the generic simulator destination for a platform.
func canonicalSimulatorBuildDestination(platform, watchProjectShape string) string {
	if IsWatchOS(platform) {
		if watchProjectShape == WatchShapePaired {
			return "generic/platform=iOS Simulator"
		}
		return "generic/platform=watchOS Simulator"
	}
	if IsTvOS(platform) {
		return "generic/platform=tvOS Simulator"
	}
	if IsVisionOS(platform) {
		return "generic/platform=visionOS Simulator"
	}
	if IsMacOS(platform) {
		return "generic/platform=macOS"
	}
	return "generic/platform=iOS Simulator"
}

func canonicalBuildCommandForShape(appName, platform, watchProjectShape string) string {
	destination := canonicalBuildDestinationForShape(platform, watchProjectShape)
	if IsMacOS(platform) {
		return fmt.Sprintf("xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet build", appName, appName, destination)
	}
	return fmt.Sprintf("xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' CODE_SIGNING_ALLOWED=NO -quiet build", appName, appName, destination)
}

func canonicalBuildCommand(appName, platform string) string {
	return canonicalBuildCommandForShape(appName, platform, "")
}

// multiPlatformBuildCommands returns device build commands for each platform scheme.
func multiPlatformBuildCommands(appName string, platforms []string) []string {
	var cmds []string
	for _, plat := range platforms {
		var scheme string
		switch plat {
		case PlatformTvOS:
			scheme = appName + "TV"
		case PlatformVisionOS:
			scheme = appName + "Vision"
		case PlatformMacOS:
			scheme = appName + "Mac"
		case PlatformWatchOS:
			continue
		default:
			scheme = appName
		}
		destination := PlatformBuildDestination(plat)
		if plat == PlatformMacOS {
			cmds = append(cmds, fmt.Sprintf("xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet build", appName, scheme, destination))
		} else {
			cmds = append(cmds, fmt.Sprintf("xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' CODE_SIGNING_ALLOWED=NO -quiet build", appName, scheme, destination))
		}
	}
	return cmds
}

// multiPlatformSimulatorBuildCommands returns simulator build commands for each platform scheme.
func multiPlatformSimulatorBuildCommands(appName string, platforms []string) []string {
	var cmds []string
	for _, plat := range platforms {
		var scheme, destination string
		switch plat {
		case PlatformTvOS:
			scheme = appName + "TV"
			destination = PlatformSimulatorDestination(PlatformTvOS)
		case PlatformVisionOS:
			scheme = appName + "Vision"
			destination = PlatformSimulatorDestination(PlatformVisionOS)
		case PlatformMacOS:
			continue
		case PlatformWatchOS:
			continue
		default:
			scheme = appName
			destination = PlatformSimulatorDestination(PlatformIOS)
		}
		cmds = append(cmds, fmt.Sprintf("xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet build", appName, scheme, destination))
	}
	return cmds
}

func writeTextFile(path, content string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), mode)
}

func writeExecutableFile(path, content string) error {
	return writeTextFile(path, content, 0o755)
}

// runXcodeGen runs `xcodegen generate` in the project directory to create the .xcodeproj.
func runXcodeGen(projectDir string) error {
	cmd := exec.Command("xcodegen", "generate")
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("xcodegen generate failed: %w\n%s", err, string(output))
	}
	return nil
}
