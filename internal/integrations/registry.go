package integrations

// CuratedIntegration describes a backend integration provider with its tooling.
type CuratedIntegration struct {
	ID          ProviderID
	Name        string
	Description string
	SPMPackage  string   // key in package_registry.go
	MCPCommand  string   // e.g. "nanowave"
	MCPArgs     []string // e.g. ["mcp", "supabase"]
	DocsMCPPkg  string   // NPM package for docs MCP (optional)
}

// integrationRegistry is the curated set of available backend providers.
var integrationRegistry = map[ProviderID]*CuratedIntegration{
	ProviderSupabase: {
		ID:          ProviderSupabase,
		Name:        "Supabase",
		Description: "Open-source backend with auth, PostgreSQL, and storage",
		SPMPackage:  "supabase-swift",
		MCPCommand:  "nanowave",
		MCPArgs:     []string{"mcp", "supabase"},
		DocsMCPPkg:  "@anthropic-ai/supabase-docs-mcp",
	},
	ProviderRevenueCat: {
		ID:          ProviderRevenueCat,
		Name:        "RevenueCat",
		Description: "In-app purchases, subscriptions, and paywalls",
		SPMPackage:  "purchases-ios",
		MCPCommand:  "nanowave",
		MCPArgs:     []string{"mcp", "revenuecat"},
	},
}

// LookupIntegration returns the curated integration for a provider ID, or nil.
func LookupIntegration(id ProviderID) *CuratedIntegration {
	return integrationRegistry[id]
}

// AllIntegrations returns all available integrations in a stable order.
func AllIntegrations() []*CuratedIntegration {
	return []*CuratedIntegration{
		integrationRegistry[ProviderSupabase],
		integrationRegistry[ProviderRevenueCat],
	}
}
