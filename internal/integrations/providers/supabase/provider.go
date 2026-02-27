// Package supabase implements the integration Provider for Supabase.
// Pattern: Adapter — delegates to existing integrations.SetupSupabase(), etc.
package supabase

import (
	"github.com/moasq/nanowave/internal/integrations"
)

// supabaseProvider implements integrations.Provider and all capability interfaces.
type supabaseProvider struct{}

// New creates a new Supabase provider.
func New() integrations.Provider {
	return &supabaseProvider{}
}

func (s *supabaseProvider) ID() integrations.ProviderID {
	return integrations.ProviderSupabase
}

func (s *supabaseProvider) Meta() integrations.ProviderMeta {
	return integrations.ProviderMeta{
		Name:        "Supabase",
		Description: "Open-source backend with auth, PostgreSQL, and storage",
		SPMPackage:  "supabase-swift",
		MCPCommand:  "nanowave",
		MCPArgs:     []string{"mcp", "supabase"},
		DocsMCPPkg:  "@anthropic-ai/supabase-docs-mcp",
	}
}

// Compile-time interface checks (like Grafana's plugin SDK pattern).
var (
	_ integrations.Provider         = (*supabaseProvider)(nil)
	_ integrations.SetupCapable     = (*supabaseProvider)(nil)
	_ integrations.MCPCapable       = (*supabaseProvider)(nil)
	_ integrations.PromptCapable    = (*supabaseProvider)(nil)
	_ integrations.ProvisionCapable = (*supabaseProvider)(nil)
)
