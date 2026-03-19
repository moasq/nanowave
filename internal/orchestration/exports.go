package orchestration

import (
	"context"

	"github.com/moasq/nanowave/internal/agentruntime"
	"github.com/moasq/nanowave/internal/integrations"
	"github.com/moasq/nanowave/internal/mcpregistry"
	"github.com/moasq/nanowave/internal/terminal"
)

// Re-export agentruntime.Kind constants for external callers.
const (
	RuntimeClaude   = agentruntime.KindClaude
	RuntimeCodex    = agentruntime.KindCodex
	RuntimeOpenCode = agentruntime.KindOpenCode
)

// exports.go provides exported wrappers around internal orchestration functions
// so the nwtool and service packages can call them without duplicating logic.

// WriteSkillsForRuntimeExternal writes skill files in the native format for the given runtime.
func WriteSkillsForRuntimeExternal(projectDir, platform string, ruleKeys []string, packages []PackagePlan, runtimeKind agentruntime.Kind) error {
	return writeSkillsForRuntime(projectDir, platform, ruleKeys, packages, runtimeKind)
}

// LoadSkillContent loads skill/rule content by key for any runtime.
func LoadSkillContent(key string) string {
	return loadRuleContent(key)
}

// ListAvailableSkillKeys returns all available skill keys from the embedded FS.
func ListAvailableSkillKeys() []string {
	return listAvailableSkillKeys()
}

// ScaffoldProjectExternal scaffolds the Xcode project: config, yml, assets, dirs, xcodegen.
func ScaffoldProjectExternal(projectDir, appName string, plan *PlannerResult) error {
	if err := writeProjectConfig(projectDir, plan, appName); err != nil {
		return err
	}
	mainEntitlements := readConfigEntitlements(projectDir, "")
	if err := writeProjectYML(projectDir, plan, appName, mainEntitlements); err != nil {
		return err
	}
	if err := writeGitignore(projectDir); err != nil {
		return err
	}

	if plan.IsMultiPlatform() {
		for _, plat := range plan.GetPlatforms() {
			suffix := PlatformSourceDirSuffix(plat)
			if err := writeAssetCatalog(projectDir, appName+suffix, plat); err != nil {
				return err
			}
		}
	} else if IsWatchOS(plan.GetPlatform()) && plan.GetWatchProjectShape() == WatchShapePaired {
		if err := writeAssetCatalog(projectDir, appName, PlatformIOS); err != nil {
			return err
		}
		if err := writeAssetCatalog(projectDir, appName+"Watch", PlatformWatchOS); err != nil {
			return err
		}
	} else {
		if err := writeAssetCatalog(projectDir, appName, plan.GetPlatform()); err != nil {
			return err
		}
	}

	if err := scaffoldSourceDirs(projectDir, appName, plan); err != nil {
		return err
	}
	return runXcodeGen(projectDir)
}

// VerifyPlannedFilesExternal checks whether all planned files exist and are valid.
func VerifyPlannedFilesExternal(projectDir, appName string, plan *PlannerResult) (*FileCompletionReport, error) {
	return verifyPlannedFiles(projectDir, appName, plan)
}

// RunXcodeGenExternal runs xcodegen generate in the project directory.
func RunXcodeGenExternal(projectDir string) error {
	return runXcodeGen(projectDir)
}

// CoreAgenticToolsList returns the core non-MCP tools used by agent runtimes.
func CoreAgenticToolsList() []string {
	tools := make([]string, len(coreAgenticTools))
	copy(tools, coreAgenticTools)
	return tools
}

// NewProgressCallbackExported wraps newProgressCallback for use outside orchestration.
func NewProgressCallbackExported(progress *terminal.ProgressDisplay) func(agentruntime.StreamEvent) {
	return newProgressCallback(progress)
}

// WriteMCPConfigExternal writes .mcp.json for the project using the standard MCP registry.
func WriteMCPConfigExternal(projectDir string) error {
	reg := mcpregistry.New()
	mcpregistry.RegisterAll(reg)
	return writeMCPConfig(projectDir, reg, nil)
}

// WriteSettingsSharedExternal writes .claude/settings.json for the project using the standard MCP registry.
func WriteSettingsSharedExternal(projectDir string) error {
	reg := mcpregistry.New()
	mcpregistry.RegisterAll(reg)
	return writeSettingsShared(projectDir, reg, nil)
}

// SetupIntegrationExternal runs the interactive setup flow for a single integration provider.
// Returns a result map with provider config details on success.
func SetupIntegrationExternal(ctx context.Context, providerID, appName, projectDir string) (map[string]any, error) {
	return setupIntegration(ctx, providerID, appName, projectDir)
}

// EnsureProjectConfigsExternal writes .mcp.json, settings.json, and settings.local.json if missing.
func EnsureProjectConfigsExternal(projectDir string) {
	reg := mcpregistry.New()
	mcpregistry.RegisterAll(reg)
	p := &Pipeline{registry: reg}
	p.ensureProjectConfigs(projectDir)
}

// WriteMCPConfigWithIntegrationsExternal writes .mcp.json including integration MCP servers.
func WriteMCPConfigWithIntegrationsExternal(projectDir string, reg *mcpregistry.Registry, integrationConfigs []integrations.MCPServerConfig) error {
	return writeMCPConfig(projectDir, reg, integrationConfigs)
}

// WriteSettingsWithIntegrationsExternal writes .claude/settings.json including integration tool permissions.
func WriteSettingsWithIntegrationsExternal(projectDir string, reg *mcpregistry.Registry, integrationTools []string) error {
	return writeSettingsShared(projectDir, reg, integrationTools)
}
