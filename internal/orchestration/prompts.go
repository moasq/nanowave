package orchestration

// coderPromptForPlatform returns the base prompt for build/edit/fix/completion phases.
func coderPromptForPlatform(platform string) string {
	target := "iOS 26+ (SwiftUI native)"
	switch {
	case IsMacOS(platform):
		target = "macOS 26+ (SwiftUI native, no UIKit)"
	case IsWatchOS(platform):
		target = "watchOS 26+ (SwiftUI native)"
	case IsTvOS(platform):
		target = "tvOS 26+ (SwiftUI native)"
	case IsVisionOS(platform):
		target = "visionOS 26+ (SwiftUI native with RealityKit)"
	}
	return "You are an expert Apple platform developer writing Swift 6 targeting " + target + `.
You have access to ALL tools — write files, edit files, run terminal commands, search Apple docs, and configure the Xcode project.
Do not manually edit project.yml — use xcodegen MCP tools instead.`
}
