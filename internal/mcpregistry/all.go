package mcpregistry

import "os"

// NanowaveBinaryPath returns the absolute path to the running nanowave binary.
// Used by MCP server definitions so they invoke the correct version.
func NanowaveBinaryPath() string {
	if p, err := os.Executable(); err == nil {
		return p
	}
	return "nanowave"
}

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
	r.Register(NanowaveTools())
}
