package revenuecat

import (
	"context"

	"github.com/moasq/nanowave/internal/integrations"
)

// revenuecatMCPTools is the set of RevenueCat MCP tools for settings allowlist.
var revenuecatMCPTools = []string{
	"mcp__revenuecat__list_products",
	"mcp__revenuecat__create_product",
	"mcp__revenuecat__list_entitlements",
	"mcp__revenuecat__create_entitlement",
	"mcp__revenuecat__attach_products_to_entitlement",
	"mcp__revenuecat__list_offerings",
	"mcp__revenuecat__create_offering",
	"mcp__revenuecat__create_package",
	"mcp__revenuecat__attach_product_to_package",
	"mcp__revenuecat__get_public_api_keys",
	"mcp__revenuecat__list_apps",
}

// revenuecatAgentTools are the agentic build tools.
var revenuecatAgentTools = []string{
	"mcp__revenuecat__list_products",
	"mcp__revenuecat__create_product",
	"mcp__revenuecat__list_entitlements",
	"mcp__revenuecat__create_entitlement",
	"mcp__revenuecat__attach_products_to_entitlement",
	"mcp__revenuecat__list_offerings",
	"mcp__revenuecat__create_offering",
	"mcp__revenuecat__create_package",
	"mcp__revenuecat__attach_product_to_package",
	"mcp__revenuecat__get_public_api_keys",
	"mcp__revenuecat__list_apps",
}

// MCPServer returns the MCP server configuration for RevenueCat.
func (r *revenuecatProvider) MCPServer(_ context.Context, req integrations.MCPRequest) (*integrations.MCPServerConfig, error) {
	cfg := &integrations.MCPServerConfig{
		Name:    "revenuecat",
		Command: "nanowave",
		Args:    []string{"mcp", "revenuecat"},
	}
	if req.PAT != "" && req.ProjectURL != "" {
		cfg.Env = map[string]string{
			"REVENUECAT_API_KEY":    req.PAT,
			"REVENUECAT_PROJECT_ID": req.ProjectURL,
		}
	}
	return cfg, nil
}

func (r *revenuecatProvider) MCPTools() []string  { return revenuecatMCPTools }
func (r *revenuecatProvider) AgentTools() []string { return revenuecatAgentTools }
