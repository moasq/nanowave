package integrations

import (
	"context"
	"strings"
)

// Provider is the base interface every integration provider implements.
// Pattern: database/sql driver.Driver — minimal required contract.
type Provider interface {
	// ID returns the unique provider identifier (e.g. ProviderSupabase).
	ID() ProviderID
	// Meta returns display metadata for the provider.
	Meta() ProviderMeta
}

// ProviderMeta holds display and tooling metadata for a provider.
type ProviderMeta struct {
	Name        string   // Human-readable name (e.g. "Supabase")
	Description string   // Short description
	SPMPackage  string   // SPM package key (e.g. "supabase-swift")
	MCPCommand  string   // MCP server command (e.g. "nanowave")
	MCPArgs     []string // MCP server args (e.g. ["mcp", "supabase"])
	DocsMCPPkg  string   // NPM package for docs MCP (optional)
}

// --- Capability interfaces (optional) ---
// Pattern: Caddy's Provisioner/Validator/CleanerUpper, Grafana's QueryDataHandler/CheckHealthHandler.
// Providers implement only what they support. Type-assert at call site.

// SetupCapable providers can be configured, removed, and queried for status via CLI.
type SetupCapable interface {
	// Setup runs the interactive setup flow for a provider.
	Setup(ctx context.Context, req SetupRequest) error
	// Remove removes the provider config for an app.
	Remove(ctx context.Context, store *IntegrationStore, appName string) error
	// Status returns the current status for an app.
	Status(ctx context.Context, store *IntegrationStore, appName string) (ProviderStatus, error)
	// CLIAvailable returns true if the provider's CLI tool is installed.
	CLIAvailable() bool
}

// PromptCapable providers can contribute content to the build prompt.
type PromptCapable interface {
	// PromptContribution generates prompt content for the build phase.
	PromptContribution(ctx context.Context, req PromptRequest) (*PromptContribution, error)
}

// MCPCapable providers expose an MCP server with tools.
type MCPCapable interface {
	// MCPServer returns the MCP server configuration for this provider.
	MCPServer(ctx context.Context, req MCPRequest) (*MCPServerConfig, error)
	// MCPTools returns MCP tool names for settings allowlist (e.g. "mcp__supabase__execute_sql").
	MCPTools() []string
	// AgentTools returns tools for the agentic build allowlist.
	AgentTools() []string
}

// ProvisionCapable providers can auto-provision backend resources.
type ProvisionCapable interface {
	// Provision creates backend resources (tables, auth, storage, etc).
	Provision(ctx context.Context, req ProvisionRequest) (*ProvisionResult, error)
}

// --- Request/Response types ---

// SetupRequest holds parameters for the Setup flow.
type SetupRequest struct {
	Store      *IntegrationStore
	AppName    string
	Manual     bool
	ReadLineFn func(label string) string
	PrintFn    func(level, msg string)
	PickFn     func(title string, options []string) string
}

// ProviderStatus summarizes provider state for an app.
type ProviderStatus struct {
	Configured  bool
	ProjectURL  string
	HasAnonKey  bool
	HasPAT      bool
	ValidatedAt string
}

// PromptRequest holds parameters for generating prompt contributions.
type PromptRequest struct {
	AppName            string
	Models             []ModelRef
	AuthMethods        []string
	Store              *IntegrationStore
	BackendProvisioned bool
}

// PromptContribution is the output of a provider's prompt generation.
type PromptContribution struct {
	// SystemBlock is content appended to the system prompt (e.g. <integration-config>).
	SystemBlock string
	// UserBlock is content injected into the user message (e.g. backend-first instructions).
	UserBlock string
	// BackendProvisioned signals whether the backend was already provisioned.
	BackendProvisioned bool
}

// MCPRequest holds parameters for MCP server configuration.
type MCPRequest struct {
	PAT        string
	ProjectRef string
}

// MCPServerConfig describes an MCP server entry for .mcp.json.
type MCPServerConfig struct {
	Name    string            // server name key (e.g. "supabase")
	Command string            // executable (e.g. "nanowave")
	Args    []string          // arguments (e.g. ["mcp", "supabase"])
	Env     map[string]string // environment variables
}

// ProvisionRequest holds parameters for backend provisioning.
type ProvisionRequest struct {
	PAT         string
	ProjectRef  string
	AppName     string
	BundleID    string
	Models      []ModelRef
	AuthMethods []string
	NeedsAuth   bool
	NeedsDB     bool
	NeedsStorage bool
	NeedsRealtime bool
}

// ProvisionResult holds the outcome of backend provisioning.
type ProvisionResult struct {
	BackendProvisioned bool
	NeedsAppleSignIn   bool
	TablesCreated      []string
	Warnings           []string
}

// ModelRef is a bridge type mirroring orchestration.ModelPlan fields.
// Avoids circular import: orchestration → integrations → orchestration.
// Pipeline converts at the call boundary (like sql.DB → driver.Value).
type ModelRef struct {
	Name       string
	Storage    string
	Properties []PropertyRef
}

// PropertyRef mirrors orchestration.PropertyPlan fields.
type PropertyRef struct {
	Name         string
	Type         string
	DefaultValue string
}

// ActiveProvider pairs a resolved provider with its per-app config.
type ActiveProvider struct {
	Provider Provider
	Config   *IntegrationConfig
}

// --- Helper functions ---

// ModelRefTableName converts a PascalCase model name to a snake_case plural table name.
func ModelRefTableName(name string) string {
	snake := modelRefCamelToSnake(name)
	if strings.HasSuffix(snake, "s") {
		return snake
	}
	if strings.HasSuffix(snake, "y") {
		return snake[:len(snake)-1] + "ies"
	}
	return snake + "s"
}

// modelRefCamelToSnake converts PascalCase/camelCase to snake_case.
func modelRefCamelToSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + 32) // toLower
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
