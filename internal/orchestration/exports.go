package orchestration

import (
	"github.com/moasq/nanowave/internal/agentruntime"
	"github.com/moasq/nanowave/internal/terminal"
)

// exports.go provides exported wrappers around internal orchestration functions
// so the nwtool and service packages can call them without duplicating logic.

// SetupWorkspaceExternal creates the project directory and .claude/ structure.
func SetupWorkspaceExternal(projectDir string) error {
	return setupWorkspace(projectDir)
}

// WriteInitialCLAUDEMDExternal writes a thin CLAUDE.md with project name and build command.
func WriteInitialCLAUDEMDExternal(projectDir, appName, platform, deviceFamily string) error {
	return writeInitialCLAUDEMD(projectDir, appName, platform, deviceFamily)
}

// EnrichCLAUDEMDExternal updates CLAUDE.md with plan-specific details.
func EnrichCLAUDEMDExternal(projectDir string, plan *PlannerResult, appName string) error {
	return enrichCLAUDEMD(projectDir, plan, appName)
}

// WriteCoreRulesExternal writes core rules to the project's .claude/rules/.
func WriteCoreRulesExternal(projectDir, platform string, packages []PackagePlan) error {
	return writeCoreRules(projectDir, platform, packages)
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
func NewProgressCallbackExported(progress *terminal.ProgressDisplay, runtimeLabel string) func(agentruntime.StreamEvent) {
	return newProgressCallback(progress, runtimeLabel)
}
