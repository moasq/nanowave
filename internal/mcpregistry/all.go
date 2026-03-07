package mcpregistry

// RegisterAll registers all internal MCP servers with the registry.
// This is the single registration point — adding a new server
// requires one line here and one new file.
// Pattern: providers/all.go — explicit, traceable registration.
//
// ASC is deliberately excluded — it runs as a separate /connect operation
// with its own pre-flight checks and HITL confirmations.
func RegisterAll(r *Registry) {
	r.Register(AppleDocs())
	r.Register(XcodeGen())
}
