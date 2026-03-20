package orchestration

import (
	"fmt"
)

// setupBuildWorkspace creates the project directory and writes skill files
// using the current writeSkillsForRuntime() system.
func (p *Pipeline) setupBuildWorkspace(projectDir, appName string, plan *PlannerResult) error {
	// Write skill files (core rules, always-on skills, platform rules)
	// For Claude: .claude/rules/*.md; for Codex: codex.md; for OpenCode: AGENTS.md
	if err := writeSkillsForRuntime(projectDir, plan.GetPlatform(), plan.RuleKeys, plan.Packages, p.runtimeKind); err != nil {
		return fmt.Errorf("failed to write skills: %w", err)
	}

	// Write MCP config and settings
	if err := writeMCPConfig(projectDir, p.registry, nil); err != nil {
		return fmt.Errorf("failed to write MCP config: %w", err)
	}
	if err := writeSettingsShared(projectDir, p.registry, nil); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}
	if err := writeSettingsLocal(projectDir); err != nil {
		return fmt.Errorf("failed to write local settings: %w", err)
	}

	return nil
}
