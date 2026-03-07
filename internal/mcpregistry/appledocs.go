package mcpregistry

// AppleDocs returns the Apple Developer Documentation MCP server definition.
func AppleDocs() Server {
	return Server{
		Name:    "apple-docs",
		Command: "npx",
		Args:    []string{"-y", "@kimsungwhee/apple-docs-mcp"},
		Tools: []string{
			"mcp__apple-docs__search_apple_docs",
			"mcp__apple-docs__get_apple_doc_content",
			"mcp__apple-docs__search_framework_symbols",
			"mcp__apple-docs__get_sample_code",
			"mcp__apple-docs__get_related_apis",
			"mcp__apple-docs__find_similar_apis",
			"mcp__apple-docs__get_platform_compatibility",
		},
	}
}
