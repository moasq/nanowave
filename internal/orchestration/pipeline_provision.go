package orchestration

import (
	"context"
	"fmt"
	"strings"

	"github.com/moasq/nanowave/internal/integrations"
	"github.com/moasq/nanowave/internal/terminal"
)

// provisionState captures the result of integration resolution and provisioning.
type provisionState struct {
	backendProvisioned bool
	needsAppleSignIn   bool
}

// provisionIntegrations resolves and provisions integrations declared in the plan,
// writes MCP configs and settings, and generates StoreKit configuration if needed.
func (p *Pipeline) provisionIntegrations(ctx context.Context, projectDir, appName string, plan *PlannerResult, analysis *AnalysisResult) (*provisionState, error) {
	state := &provisionState{}

	if p.manager == nil || len(plan.Integrations) == 0 {
		terminal.Detail("Integrations", "none in plan")
		return state, nil
	}

	terminal.Info(fmt.Sprintf("Resolving %d integration(s): %s", len(plan.Integrations), strings.Join(plan.Integrations, ", ")))
	ui := &pipelineSetupUI{}
	activeProviders, err := p.manager.Resolve(ctx, appName, plan.Integrations, ui)
	if err != nil {
		terminal.Warning(fmt.Sprintf("Integration resolution failed: %v", err))
	}
	p.activeProviders = activeProviders

	var activeIntegrationIDs []string
	for _, ap := range activeProviders {
		activeIntegrationIDs = append(activeIntegrationIDs, string(ap.Provider.ID()))
	}
	terminal.Detail("Active integrations", fmt.Sprintf("%d: %s", len(activeIntegrationIDs), strings.Join(activeIntegrationIDs, ", ")))

	// Provision via Manager
	if len(activeProviders) > 0 && (analysis.BackendNeeds != nil && analysis.BackendNeeds.NeedsBackend() || plan.MonetizationPlan != nil) {
		state.backendProvisioned, state.needsAppleSignIn = p.runProvisioning(ctx, appName, plan, analysis, activeProviders)
	}

	// Generate StoreKit configuration file for local testing
	if plan.MonetizationPlan != nil {
		if err := writeStoreKitConfig(projectDir, appName, plan.MonetizationPlan); err != nil {
			terminal.Warning(fmt.Sprintf("StoreKit config generation failed: %v", err))
		} else {
			terminal.Success("StoreKit configuration file generated")
		}
	}

	// Write MCP config via Manager
	mcpConfigs, err := p.manager.MCPConfigs(ctx, activeProviders)
	if err != nil {
		terminal.Warning(fmt.Sprintf("MCP config generation failed: %v", err))
	}
	if err := writeMCPConfig(projectDir, p.registry, mcpConfigs); err != nil {
		return nil, fmt.Errorf("failed to write MCP config: %w", err)
	}
	terminal.Detail("MCP config", fmt.Sprintf("written to %s/.mcp.json (%d integrations)", projectDir, len(mcpConfigs)))

	// Write settings with Manager tool allowlist
	mcpTools := p.manager.MCPToolAllowlist(activeProviders)
	if err := writeSettingsShared(projectDir, p.registry, mcpTools); err != nil {
		return nil, fmt.Errorf("failed to update settings with integration permissions: %w", err)
	}

	return state, nil
}

// runProvisioning executes the provisioning request and returns backendProvisioned and needsAppleSignIn flags.
func (p *Pipeline) runProvisioning(ctx context.Context, appName string, plan *PlannerResult, analysis *AnalysisResult, activeProviders []integrations.ActiveProvider) (backendProvisioned, needsAppleSignIn bool) {
	var authMethods []string
	if analysis.BackendNeeds != nil {
		authMethods = analysis.BackendNeeds.AuthMethods
		if analysis.BackendNeeds.Auth && len(authMethods) == 0 {
			authMethods = []string{"email", "anonymous"}
		}
	}
	provReq := integrations.ProvisionRequest{
		AppName:       appName,
		BundleID:      fmt.Sprintf("%s.%s", bundleIDPrefix(), strings.ToLower(appName)),
		Models:        modelsToModelRefs(plan.Models),
		AuthMethods:   authMethods,
		NeedsAuth:     analysis.BackendNeeds != nil && analysis.BackendNeeds.Auth,
		NeedsDB:       analysis.BackendNeeds != nil && (analysis.BackendNeeds.DB || len(plan.Models) > 0),
		NeedsStorage:  analysis.BackendNeeds != nil && analysis.BackendNeeds.Storage,
		NeedsRealtime: analysis.BackendNeeds != nil && analysis.BackendNeeds.Realtime,
	}
	if plan.MonetizationPlan != nil {
		provReq.NeedsMonetization = true
		provReq.MonetizationType = plan.MonetizationPlan.Model
		provReq.MonetizationPlan = monetizationPlanToRef(plan.MonetizationPlan)
	}
	provResult, err := p.manager.Provision(ctx, provReq, activeProviders)
	if err != nil {
		terminal.Warning(fmt.Sprintf("Provisioning failed: %v", err))
		return false, false
	}
	if provResult == nil {
		return false, false
	}
	for _, w := range provResult.Warnings {
		terminal.Warning(w)
	}
	if len(provResult.TablesCreated) > 0 {
		terminal.Success(fmt.Sprintf("Tables created: %s", strings.Join(provResult.TablesCreated, ", ")))
	}
	return provResult.BackendProvisioned, provResult.NeedsAppleSignIn
}
