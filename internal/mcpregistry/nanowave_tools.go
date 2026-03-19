package mcpregistry

// NanowaveTools returns the nanowave tools MCP server definition.
func NanowaveTools() Server {
	return Server{
		Name:    "nanowave-tools",
		Command: "nanowave",
		Args:    []string{"mcp", "tools"},
		Tools: []string{
			"mcp__nanowave-tools__nw_setup_workspace",
			"mcp__nanowave-tools__nw_enrich_workspace",
			"mcp__nanowave-tools__nw_scaffold_project",
			"mcp__nanowave-tools__nw_verify_files",
			"mcp__nanowave-tools__nw_xcode_build",
			"mcp__nanowave-tools__nw_finalize_project",
			"mcp__nanowave-tools__nw_project_info",
			"mcp__nanowave-tools__nw_validate_platform",
		},
	}
}
