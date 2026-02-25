package xcodegenserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Run starts the XcodeGen MCP server over stdio.
// It blocks until the client disconnects or the context is cancelled.
func Run(ctx context.Context) error {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "xcodegen",
			Version: "v1.0.0",
		},
		nil,
	)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_permission",
		Description: "Add a permission to the Xcode project. Adds the INFOPLIST_KEY entry to project.yml and regenerates the .xcodeproj. Works for iOS and watchOS projects. Example: add_permission(key: \"NSCameraUsageDescription\", description: \"Take photos for your profile\", framework: \"AVFoundation\")",
	}, handleAddPermission)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_extension",
		Description: "Add an extension target (widget, live activity, share sheet, etc.) to the Xcode project. Creates the full target configuration, scaffolds Targets/{Name}/ and Shared/ directories, sets up entitlements and Info.plist, and regenerates .xcodeproj. Note: watchOS only supports widget extensions.",
	}, handleAddExtension)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_entitlement",
		Description: "Add an entitlement to a target in the Xcode project. For example, App Groups for data sharing between app and extensions, or HealthKit access. Regenerates .xcodeproj after adding.",
	}, handleAddEntitlement)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_localization",
		Description: "Add language support to the Xcode project. Sets knownRegions, creates .lproj directories, and configures the localization resource handling in project.yml. Regenerates .xcodeproj.",
	}, handleAddLocalization)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_build_setting",
		Description: "Set an arbitrary Xcode build setting on a target. Can target the main app or any extension target. Regenerates .xcodeproj after setting.",
	}, handleSetBuildSetting)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_package",
		Description: "Add an SPM (Swift Package Manager) dependency to the Xcode project. Adds the package to the top-level packages section and as a dependency of the main app target, then regenerates .xcodeproj. Example: add_package(name: \"Lottie\", url: \"https://github.com/airbnb/lottie-ios\", min_version: \"4.0.0\")",
	}, handleAddPackage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_project_config",
		Description: "Get the current Xcode project configuration. Returns all targets, permissions, extensions, entitlements, localizations, packages, and build settings. Read-only — does not run xcodegen.",
	}, handleGetProjectConfig)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "regenerate_project",
		Description: "Regenerate the .xcodeproj from project.yml by running xcodegen generate. Use this after manually editing project.yml or when the .xcodeproj gets out of sync.",
	}, handleRegenerateProject)

	return server.Run(ctx, &mcp.StdioTransport{})
}
