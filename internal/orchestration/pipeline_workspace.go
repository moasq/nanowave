package orchestration

import (
	"fmt"

	"github.com/moasq/nanowave/internal/terminal"
)

// setupBuildWorkspace creates the CLAUDE.md, core rules, conditional skills,
// and Claude project scaffold in the workspace directory.
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

	if plan.IsMultiPlatform() {
		platforms := plan.GetPlatforms()
		if err := writeAlwaysSkills(projectDir, platforms[0], platforms[1:]...); err != nil {
			return fmt.Errorf("failed to write always skills: %w", err)
		}
	} else {
		if err := writeAlwaysSkills(projectDir, plan.GetPlatform()); err != nil {
			return fmt.Errorf("failed to write always skills: %w", err)
		}
	}

	// Auto-inject adaptive-layout skill for iPad/universal apps (iOS only)
	if plan.GetPlatform() == PlatformIOS {
		if family := plan.GetDeviceFamily(); family == "ipad" || family == "universal" {
			hasAdaptive := false
			for _, k := range plan.RuleKeys {
				if k == "adaptive-layout" {
					hasAdaptive = true
					break
				}
			}
			if !hasAdaptive {
				plan.RuleKeys = append(plan.RuleKeys, "adaptive-layout")
			}
		}
	}

	if err := writeConditionalSkills(projectDir, plan.RuleKeys, plan.GetPlatform()); err != nil {
		return fmt.Errorf("failed to write conditional skills: %w", err)
	}

	scaffoldPlatform := plan.GetPlatform()
	scaffoldShape := plan.GetWatchProjectShape()
	if plan.IsMultiPlatform() {
		scaffoldPlatform = PlatformIOS
		scaffoldShape = ""
	}
	if err := writeClaudeProjectScaffoldWithShape(projectDir, appName, scaffoldPlatform, scaffoldShape, p.registry); err != nil {
		return fmt.Errorf("failed to write Claude project scaffold: %w", err)
	}

	if err := writeSettingsLocal(projectDir); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}

	terminal.Detail("Workspace", "CLAUDE.md, rules, skills, scaffold ready")
	return nil
}
