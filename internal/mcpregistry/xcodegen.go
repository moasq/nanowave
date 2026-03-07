package mcpregistry

// XcodeGen returns the XcodeGen project configuration MCP server definition.
func XcodeGen() Server {
	return Server{
		Name:    "xcodegen",
		Command: "nanowave",
		Args:    []string{"mcp", "xcodegen"},
		Tools: []string{
			"mcp__xcodegen__add_permission",
			"mcp__xcodegen__add_extension",
			"mcp__xcodegen__add_entitlement",
			"mcp__xcodegen__add_localization",
			"mcp__xcodegen__set_build_setting",
			"mcp__xcodegen__get_project_config",
			"mcp__xcodegen__add_package",
			"mcp__xcodegen__regenerate_project",
		},
	}
}
