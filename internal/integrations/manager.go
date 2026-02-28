package integrations

import (
	"context"
	"fmt"
)

// Manager is the single facade orchestration uses for all integration operations.
// Pattern: database/sql.DB wraps driver internals, Docker CLI's command.Cli interface.
// Pipeline calls Manager methods; Manager iterates providers and type-asserts capabilities.
type Manager struct {
	registry *Registry
	store    *IntegrationStore
}

// NewManager creates a Manager backed by the given registry and store.
func NewManager(registry *Registry, store *IntegrationStore) *Manager {
	return &Manager{
		registry: registry,
		store:    store,
	}
}

// Store returns the underlying IntegrationStore.
func (m *Manager) Store() *IntegrationStore {
	return m.store
}

// Resolve resolves planned integration IDs into active providers with their configs.
// For each planned ID, it looks up the config from the store. If no config exists,
// it calls the SetupUI callback to let the user configure the provider.
func (m *Manager) Resolve(ctx context.Context, appName string, planned []string, ui SetupUI) ([]ActiveProvider, error) {
	var active []ActiveProvider

	for _, idStr := range planned {
		id := ProviderID(idStr)
		p, ok := m.registry.Get(id)
		if !ok {
			ui.Warning(fmt.Sprintf("Unknown integration: %s — skipping", idStr))
			continue
		}

		cfg, _ := m.store.GetProvider(id, appName)

		if cfg == nil {
			// No config — try auto-setup if provider supports it
			if sc, ok := p.(SetupCapable); ok {
				cfg = ui.PromptSetup(ctx, sc, p, m.store, appName)
			}
			if cfg == nil {
				ui.Info("Continuing without backend — using placeholder config")
				continue
			}
		} else {
			// Config found — validate and refresh if needed
			if sc, ok := p.(SetupCapable); ok {
				cfg = ui.ValidateExisting(ctx, sc, p, m.store, appName, cfg)
			}
		}

		if cfg == nil {
			continue
		}

		active = append(active, ActiveProvider{Provider: p, Config: cfg})
	}

	return active, nil
}

// AgentTools returns the combined agentic tool allowlist for all active providers.
// Pattern: Caddy type-asserts guest modules to check capabilities.
func (m *Manager) AgentTools(active []ActiveProvider) []string {
	var tools []string
	for _, a := range active {
		if mc, ok := a.Provider.(MCPCapable); ok {
			tools = append(tools, mc.AgentTools()...)
		}
	}
	return tools
}

// MCPConfigs returns MCP server configurations for all active providers.
func (m *Manager) MCPConfigs(ctx context.Context, active []ActiveProvider) ([]MCPServerConfig, error) {
	var configs []MCPServerConfig
	for _, a := range active {
		mc, ok := a.Provider.(MCPCapable)
		if !ok {
			continue
		}
		req := MCPRequest{
			PAT:        a.Config.PAT,
			ProjectURL: a.Config.ProjectURL,
			ProjectRef: a.Config.ProjectRef,
		}
		cfg, err := mc.MCPServer(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("MCP config for %s: %w", a.Provider.ID(), err)
		}
		if cfg != nil {
			configs = append(configs, *cfg)
		}
	}
	return configs, nil
}

// MCPToolAllowlist returns the combined MCP tool allowlist for all active providers.
func (m *Manager) MCPToolAllowlist(active []ActiveProvider) []string {
	var tools []string
	for _, a := range active {
		if mc, ok := a.Provider.(MCPCapable); ok {
			tools = append(tools, mc.MCPTools()...)
		}
	}
	return tools
}

// PromptContributions collects prompt content from all active providers.
func (m *Manager) PromptContributions(ctx context.Context, req PromptRequest, active []ActiveProvider) ([]PromptContribution, error) {
	var contributions []PromptContribution
	for _, a := range active {
		pc, ok := a.Provider.(PromptCapable)
		if !ok {
			continue
		}
		// Override store in request with the config we already resolved
		r := req
		r.Store = m.store
		contrib, err := pc.PromptContribution(ctx, r)
		if err != nil {
			return nil, fmt.Errorf("prompt contribution for %s: %w", a.Provider.ID(), err)
		}
		if contrib != nil {
			contributions = append(contributions, *contrib)
		}
	}
	return contributions, nil
}

// Provision runs backend provisioning for all active providers that support it.
func (m *Manager) Provision(ctx context.Context, req ProvisionRequest, active []ActiveProvider) (*ProvisionResult, error) {
	combined := &ProvisionResult{}
	for _, a := range active {
		pc, ok := a.Provider.(ProvisionCapable)
		if !ok {
			continue
		}
		// Fill in per-provider credentials from resolved config
		r := req
		r.PAT = a.Config.PAT
		r.ProjectURL = a.Config.ProjectURL
		r.ProjectRef = a.Config.ProjectRef
		result, err := pc.Provision(ctx, r)
		if err != nil {
			return nil, fmt.Errorf("provision for %s: %w", a.Provider.ID(), err)
		}
		if result != nil {
			if result.BackendProvisioned {
				combined.BackendProvisioned = true
			}
			if result.NeedsAppleSignIn {
				combined.NeedsAppleSignIn = true
			}
			combined.TablesCreated = append(combined.TablesCreated, result.TablesCreated...)
			combined.Warnings = append(combined.Warnings, result.Warnings...)
		}
	}
	return combined, nil
}

// ResolveExisting returns active providers that already have stored configs for
// the given app. Unlike Resolve, this never prompts for setup — it silently
// skips providers with no config. Used by the Edit/Fix flows where the project
// was already built and integrations were configured during the original build.
func (m *Manager) ResolveExisting(appName string) []ActiveProvider {
	var active []ActiveProvider
	for _, p := range m.registry.All() {
		cfg, _ := m.store.GetProvider(p.ID(), appName)
		if cfg == nil {
			continue
		}
		active = append(active, ActiveProvider{Provider: p, Config: cfg})
	}
	return active
}

// AllProviders returns all registered providers.
func (m *Manager) AllProviders() []Provider {
	return m.registry.All()
}

// GetProvider returns a provider by ID from the registry.
func (m *Manager) GetProvider(id ProviderID) (Provider, bool) {
	return m.registry.Get(id)
}

// SetupUI abstracts the user interaction during integration resolution.
// The pipeline implements this to bridge terminal UI calls.
type SetupUI interface {
	// PromptSetup handles first-time setup for a provider. Returns config or nil if skipped.
	PromptSetup(ctx context.Context, sc SetupCapable, p Provider, store *IntegrationStore, appName string) *IntegrationConfig
	// ValidateExisting validates an existing config, refreshing if needed. Returns updated config or nil.
	ValidateExisting(ctx context.Context, sc SetupCapable, p Provider, store *IntegrationStore, appName string, cfg *IntegrationConfig) *IntegrationConfig
	// Info prints an informational message.
	Info(msg string)
	// Warning prints a warning message.
	Warning(msg string)
}
