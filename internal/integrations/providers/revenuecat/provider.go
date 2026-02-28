// Package revenuecat implements the integration Provider for RevenueCat.
// Pattern: same as supabase — delegates to capability interfaces.
package revenuecat

import (
	"github.com/moasq/nanowave/internal/integrations"
)

// revenuecatProvider implements integrations.Provider and all capability interfaces.
type revenuecatProvider struct{}

// New creates a new RevenueCat provider.
func New() integrations.Provider {
	return &revenuecatProvider{}
}

func (r *revenuecatProvider) ID() integrations.ProviderID {
	return integrations.ProviderRevenueCat
}

func (r *revenuecatProvider) Meta() integrations.ProviderMeta {
	return integrations.ProviderMeta{
		Name:        "RevenueCat",
		Description: "In-app purchases, subscriptions, and paywalls",
		SPMPackage:  "purchases-ios",
		MCPCommand:  "nanowave",
		MCPArgs:     []string{"mcp", "revenuecat"},
	}
}

// Compile-time interface checks.
var (
	_ integrations.Provider         = (*revenuecatProvider)(nil)
	_ integrations.SetupCapable     = (*revenuecatProvider)(nil)
	_ integrations.PromptCapable    = (*revenuecatProvider)(nil)
	_ integrations.MCPCapable       = (*revenuecatProvider)(nil)
	_ integrations.ProvisionCapable = (*revenuecatProvider)(nil)
)
