// Package mcpregistry provides a service layer for internal MCP servers.
// Each server (apple-docs, xcodegen) registers independently — no
// server knows about any other. The pipeline queries the registry for
// server configs (.mcp.json), tool allowlists (settings.json), and
// agent tool lists (build phase).
package mcpregistry

// Server describes an always-on internal MCP server.
type Server struct {
	Name    string   // MCP server name (e.g. "xcodegen", "asc")
	Command string   // executable (e.g. "nanowave", "npx")
	Args    []string // command arguments (e.g. ["mcp", "xcodegen"])
	Tools   []string // MCP tool names (e.g. "mcp__xcodegen__add_permission")
}

// Registry aggregates internal MCP servers.
// Pattern: providers/all.go — explicit registration, no init() magic.
type Registry struct {
	servers []Server
}

// New creates an empty registry.
func New() *Registry { return &Registry{} }

// Register adds a server to the registry.
func (r *Registry) Register(s Server) {
	r.servers = append(r.servers, s)
}

// Servers returns all registered server definitions.
func (r *Registry) Servers() []Server {
	return r.servers
}

// AllTools returns all MCP tool names across all registered servers.
func (r *Registry) AllTools() []string {
	var tools []string
	for _, s := range r.servers {
		tools = append(tools, s.Tools...)
	}
	return tools
}
