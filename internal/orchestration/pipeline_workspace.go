package orchestration

import (
	"fmt"
)

// setupBuildWorkspace creates the CLAUDE.md, core rules, settings,
// and Makefile in the workspace directory.
func (p *Pipeline) setupBuildWorkspace(projectDir, appName string, plan *PlannerResult) error {
	if err := setupWorkspace(projectDir); err != nil {
		return fmt.Errorf("workspace setup failed: %w", err)
	}

	if err := writeInitialCLAUDEMD(projectDir, appName, plan.GetPlatform(), plan.GetDeviceFamily()); err != nil {
		return fmt.Errorf("failed to write CLAUDE.md: %w", err)
	}

	if err := enrichCLAUDEMD(projectDir, plan, appName); err != nil {
		return fmt.Errorf("failed to enrich CLAUDE.md: %w", err)
	}

	if err := writeCoreRules(projectDir, plan.GetPlatform(), plan.Packages); err != nil {
		return fmt.Errorf("failed to write core rules: %w", err)
	}

	scaffoldPlatform := plan.GetPlatform()
	scaffoldShape := plan.GetWatchProjectShape()
	if plan.IsMultiPlatform() {
		scaffoldPlatform = PlatformIOS
		scaffoldShape = ""
	}

	if err := writeSettingsShared(projectDir, p.registry, nil); err != nil {
		return fmt.Errorf("failed to write shared settings: %w", err)
	}

	if err := writeSettingsLocal(projectDir); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}

	if err := writeProjectMakefileWithShape(projectDir, appName, scaffoldPlatform, scaffoldShape); err != nil {
		return fmt.Errorf("failed to write Makefile: %w", err)
	}

	return nil
}
