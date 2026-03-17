package orchestration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/moasq/nanowave/internal/integrations"
	"github.com/moasq/nanowave/internal/integrations/providers"
	"github.com/moasq/nanowave/internal/mcpregistry"
)

// setupIntegration runs the interactive setup flow for a single integration provider.
// Called by the nw_setup_integration tool when the agent decides an integration is needed.
func setupIntegration(ctx context.Context, providerID, appName, projectDir string) (map[string]any, error) {
	// Build a fresh manager with all registered providers
	reg := integrations.NewRegistry()
	providers.RegisterAll(reg)
	home, _ := os.UserHomeDir()
	storeRoot := filepath.Join(home, ".nanowave")
	store := integrations.NewIntegrationStore(storeRoot)
	_ = store.Load()
	mgr := integrations.NewManager(reg, store)

	// Resolve the specific provider — this triggers the setup UI if unconfigured
	ui := &pipelineSetupUI{}
	resolved, err := mgr.Resolve(ctx, appName, []string{providerID}, ui)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %s: %w", providerID, err)
	}
	if len(resolved) == 0 {
		return map[string]any{
			"success":  false,
			"provider": providerID,
			"message":  fmt.Sprintf("%s setup was skipped — the agent should use placeholder credentials or skip this integration", providerID),
		}, nil
	}

	// If project_dir is provided, update MCP config and settings to include the new integration
	if projectDir != "" {
		mcpReg := mcpregistry.New()
		mcpregistry.RegisterAll(mcpReg)
		mcpConfigs, _ := mgr.MCPConfigs(ctx, resolved)
		_ = writeMCPConfig(projectDir, mcpReg, mcpConfigs)
		mcpTools := mgr.MCPToolAllowlist(resolved)
		_ = writeSettingsShared(projectDir, mcpReg, mcpTools)
	}

	cfg := resolved[0].Config
	result := map[string]any{
		"success":     true,
		"provider":    providerID,
		"project_ref": cfg.ProjectRef,
		"project_url": cfg.ProjectURL,
		"has_api_key": cfg.AnonKey != "",
		"message":     fmt.Sprintf("%s is configured and ready to use", providerID),
	}
	return result, nil
}
