package orchestration

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed skills
var skillsFS embed.FS

// setupWorkspace creates the project directory and .claude/ structure.
func setupWorkspace(projectDir string) error {
	dirs := []string{
		projectDir,
		filepath.Join(projectDir, ".claude", "rules"),
		filepath.Join(projectDir, ".claude", "skills"),
		filepath.Join(projectDir, ".claude", "memory"),
		filepath.Join(projectDir, ".claude", "commands"),
		filepath.Join(projectDir, ".claude", "agents"),
		filepath.Join(projectDir, "scripts", "claude"),
		filepath.Join(projectDir, "docs"),
		filepath.Join(projectDir, ".github", "workflows"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}
	return nil
}

// writeInitialCLAUDEMD writes the CLAUDE.md with project-specific info only (before plan exists).
// CLAUDE.md is a thin index that imports shared project memory modules and core rules.
func writeInitialCLAUDEMD(projectDir, appName, platform, deviceFamily string) error {
	if err := writeClaudeMemoryFiles(projectDir, appName, platform, deviceFamily, nil); err != nil {
		return err
	}
	return writeCLAUDEMDIndex(projectDir, appName)
}

// enrichCLAUDEMD updates memory modules with plan-specific details after Phase 3.
func enrichCLAUDEMD(projectDir string, plan *PlannerResult, appName string) error {
	if err := writeClaudeMemoryFiles(projectDir, appName, plan.GetPlatform(), plan.GetDeviceFamily(), plan); err != nil {
		return err
	}
	return writeCLAUDEMDIndex(projectDir, appName)
}

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
// Device builds are the default validation target — they catch entitlement, architecture,
// and API availability issues that simulator builds miss. Uses CODE_SIGNING_ALLOWED=NO
// so no provisioning profile is required.
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
// Used by Run() for launching in the simulator and as a secondary validation pass.
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
		// macOS has no separate device/simulator distinction
		return fmt.Sprintf("xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet build", appName, appName, destination)
	}
	return fmt.Sprintf("xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' CODE_SIGNING_ALLOWED=NO -quiet build", appName, appName, destination)
}

func canonicalBuildCommand(appName, platform string) string {
	return canonicalBuildCommandForShape(appName, platform, "")
}

// multiPlatformBuildCommands returns device build commands for each platform scheme.
// Device builds are the default — they catch entitlement and architecture issues.
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
			// In multi-platform, watchOS is built via the iOS scheme (paired)
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
// Used as a secondary validation pass and for Run() launching.
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
			continue // macOS has no separate simulator build
		case PlatformWatchOS:
			continue // watchOS is built via iOS scheme
		default:
			scheme = appName
			destination = PlatformSimulatorDestination(PlatformIOS)
		}
		cmds = append(cmds, fmt.Sprintf("xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet build", appName, scheme, destination))
	}
	return cmds
}

func writeTextFile(path, content string, mode fs.FileMode) error {
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
