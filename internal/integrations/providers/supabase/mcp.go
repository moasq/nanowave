package supabase

import (
	"context"

	"github.com/moasq/nanowave/internal/integrations"
)

// supabaseMCPTools is the set of Supabase MCP tools for settings allowlist.
var supabaseMCPTools = []string{
	"mcp__supabase__execute_sql",
	"mcp__supabase__list_tables",
	"mcp__supabase__apply_migration",
	"mcp__supabase__list_storage_buckets",
	"mcp__supabase__get_project_url",
	"mcp__supabase__get_anon_key",
	"mcp__supabase__get_logs",
	"mcp__supabase__configure_auth_providers",
	"mcp__supabase__get_auth_config",
}

// supabaseAgentTools are the agentic build tools (same as MCP tools for Supabase).
var supabaseAgentTools = []string{
	"mcp__supabase__execute_sql",
	"mcp__supabase__list_tables",
	"mcp__supabase__apply_migration",
	"mcp__supabase__list_storage_buckets",
	"mcp__supabase__get_project_url",
	"mcp__supabase__get_anon_key",
	"mcp__supabase__get_logs",
	"mcp__supabase__configure_auth_providers",
	"mcp__supabase__get_auth_config",
}

// MCPServer returns the MCP server configuration for Supabase.
func (s *supabaseProvider) MCPServer(_ context.Context, req integrations.MCPRequest) (*integrations.MCPServerConfig, error) {
	cfg := &integrations.MCPServerConfig{
		Name:    "supabase",
		Command: "nanowave",
		Args:    []string{"mcp", "supabase"},
	}
	if req.PAT != "" && req.ProjectRef != "" {
		cfg.Env = map[string]string{
			"SUPABASE_ACCESS_TOKEN": req.PAT,
			"SUPABASE_PROJECT_REF":  req.ProjectRef,
		}
	}
	return cfg, nil
}

// MCPTools returns the Supabase MCP tool names for settings allowlist.
func (s *supabaseProvider) MCPTools() []string {
	return supabaseMCPTools
}

// AgentTools returns the Supabase tools for the agentic build allowlist.
func (s *supabaseProvider) AgentTools() []string {
	return supabaseAgentTools
}
