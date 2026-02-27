package orchestration

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func writeCLAUDEMDIndex(projectDir, appName string) error {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(appName)
	b.WriteString(" — Claude Code Memory Index\n\n")

	imports := []string{
		"@memory/project-overview.md",
		"@memory/architecture.md",
		"@memory/design-system.md",
		"@memory/xcodegen-policy.md",
		"@memory/build-fix-workflow.md",
		"@memory/review-playbook.md",
		"@memory/accessibility-policy.md",
		"@memory/quality-gates.md",
		"@memory/generated-plan.md",
	}

	coreRuleImports := make([]string, 0, 8)
	if entries, err := fs.ReadDir(skillsFS, "skills/core"); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			coreRuleImports = append(coreRuleImports, "@rules/"+entry.Name())
		}
		sort.Strings(coreRuleImports)
	}

	for _, line := range imports {
		b.WriteString(line)
		b.WriteString("\n")
	}
	if len(coreRuleImports) > 0 {
		b.WriteString("\n# Core Rules (explicit imports for deterministic loading)\n")
		for _, line := range coreRuleImports {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n# Skills\n")
	b.WriteString("Project skills live in `skills/`. Use `/preflight`, `/build-green`, `/quality-review`, `/accessibility-audit`, and `/xcodegen-change` for repeatable workflows.\n")

	b.WriteString("\n## Required Reading — Load These Before Writing Code\n")
	b.WriteString("Before writing ANY Swift code, you MUST read these skills:\n")
	requiredSkills := []string{
		"@skills/swiftui/SKILL.md",
		"@skills/components/SKILL.md",
		"@skills/layout/SKILL.md",
		"@skills/navigation/SKILL.md",
		"@skills/design-system/SKILL.md",
	}
	for _, s := range requiredSkills {
		b.WriteString(s)
		b.WriteString("\n")
	}

	return os.WriteFile(filepath.Join(projectDir, ".claude", "CLAUDE.md"), []byte(b.String()), 0o644)
}

func writeClaudeMemoryFiles(projectDir, appName, platform, deviceFamily string, plan *PlannerResult) error {
	memoryDir := filepath.Join(projectDir, ".claude", "memory")
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		return fmt.Errorf("failed to create memory dir: %w", err)
	}

	watchProjectShape := ""
	if plan != nil {
		watchProjectShape = plan.GetWatchProjectShape()
	}
	isMulti := plan != nil && plan.IsMultiPlatform()
	var files = map[string]string{}

	// project-overview.md
	var overview strings.Builder
	overview.WriteString("# Project Overview\n\n")
	overview.WriteString("- App name: `")
	overview.WriteString(appName)
	overview.WriteString("`\n")
	if isMulti {
		overview.WriteString("- Platforms: ")
		for i, p := range plan.GetPlatforms() {
			if i > 0 {
				overview.WriteString(", ")
			}
			overview.WriteString(PlatformDisplayName(p))
		}
		overview.WriteString("\n")
		overview.WriteString("- Stack: SwiftUI (SPM packages added as needed during build)\n")
	} else {
		overview.WriteString("- Platform: ")
		overview.WriteString(platformSummary(platform, deviceFamily))
		overview.WriteString("\n")
		if IsWatchOS(platform) {
			overview.WriteString("- Stack: SwiftUI (SPM packages added as needed, no UIKit)\n")
		} else if IsTvOS(platform) {
			overview.WriteString("- Stack: SwiftUI (SPM packages added as needed, no UIKit)\n")
		} else if IsVisionOS(platform) {
			overview.WriteString("- Stack: SwiftUI + RealityKit (SPM packages added as needed, no UIKit)\n")
		} else if IsMacOS(platform) {
			overview.WriteString("- Stack: SwiftUI native macOS, AppKit bridge when needed, no UIKit. Menu bar, keyboard shortcuts, Settings scene, multiple windows.\n")
		} else {
			overview.WriteString("- Stack: SwiftUI + SwiftData (SPM packages added as needed)\n")
		}
	}
	overview.WriteString("- Project config source of truth: `project_config.json`\n")
	overview.WriteString("- XcodeGen spec: `project.yml`\n")

	if isMulti {
		overview.WriteString("\n## Source Directories\n")
		for _, p := range plan.GetPlatforms() {
			suffix := PlatformSourceDirSuffix(p)
			dirName := appName + suffix
			overview.WriteString(fmt.Sprintf("- `%s/` — %s source\n", dirName, PlatformDisplayName(p)))
		}
		overview.WriteString(fmt.Sprintf("- `Shared/` — cross-platform code\n"))

		overview.WriteString("\n## Build Commands\n\n```sh\n")
		for _, cmd := range multiPlatformBuildCommands(appName, plan.GetPlatforms()) {
			overview.WriteString(cmd)
			overview.WriteString("\n")
		}
		overview.WriteString("```\n")
	} else {
		buildCmd := canonicalBuildCommandForShape(appName, platform, watchProjectShape)
		overview.WriteString("\n## Canonical Build Command\n\n```sh\n")
		overview.WriteString(buildCmd)
		overview.WriteString("\n```\n")
	}
	overview.WriteString("\n## Constraints\n")
	overview.WriteString("- Use xcodegen MCP tools for permissions/extensions/entitlements/localizations/build settings\n")
	overview.WriteString("- Never manually edit `.xcodeproj` (generated file)\n")
	overview.WriteString("- Prefer Apple docs MCP for API verification; fall back to WebFetch/WebSearch when needed\n")
	if plan != nil && len(plan.Localizations) > 0 {
		overview.WriteString("- Localizations: ")
		overview.WriteString(strings.Join(plan.Localizations, ", "))
		overview.WriteString("\n")
	}
	files["project-overview.md"] = overview.String()

	// architecture.md
	var arch strings.Builder
	arch.WriteString("# Architecture\n\n")
	arch.WriteString("## App Structure\n")
	arch.WriteString("- `@main App` -> `RootView` -> `MainView` -> feature content\n")
	arch.WriteString("- Models in `Models/`\n")
	arch.WriteString("- Theme in `Theme/` (`AppTheme` is the single source of design tokens)\n")
	arch.WriteString("- Features in `Features/<Name>/`\n")
	arch.WriteString("- Shared feature services/components in `Features/Common/`\n")
	arch.WriteString("\n## Project Scaffolding Rules\n")
	arch.WriteString("- Keep generated project config changes in xcodegen MCP tools\n")
	arch.WriteString("- Shared extension types must live in `Shared/`\n")
	if plan != nil && len(plan.Extensions) > 0 {
		arch.WriteString("\n## Extensions\n")
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			fmt.Fprintf(&arch, "- `%s` (%s): %s -> `Targets/%s/`\n", name, ext.Kind, ext.Purpose, name)
		}
	} else {
		arch.WriteString("\n## Extensions\n- No extension targets planned.\n")
	}
	files["architecture.md"] = arch.String()

	// design-system.md
	var design strings.Builder
	design.WriteString("# Design System\n\n")
	design.WriteString("## Strict Enforcement\n")
	design.WriteString("- `AppTheme` is the **ONLY** place for colors, fonts, spacing, and style tokens\n")
	design.WriteString("- See `.claude/rules/forbidden-patterns.md` \"Hardcoded Styling\" section for full banned patterns\n")
	design.WriteString("- Every view should use semantic theme tokens and include `#Preview`\n")
	if plan != nil {
		design.WriteString("\n## Current Palette\n")
		fmt.Fprintf(&design, "- Primary: `%s`\n", plan.Design.Palette.Primary)
		fmt.Fprintf(&design, "- Secondary: `%s`\n", plan.Design.Palette.Secondary)
		fmt.Fprintf(&design, "- Accent: `%s`\n", plan.Design.Palette.Accent)
		fmt.Fprintf(&design, "- Background: `%s`\n", plan.Design.Palette.Background)
		fmt.Fprintf(&design, "- Surface: `%s`\n", plan.Design.Palette.Surface)
		fmt.Fprintf(&design, "- Font design: `%s`\n", plan.Design.FontDesign)
		fmt.Fprintf(&design, "- Corner radius: `%d`\n", plan.Design.CornerRadius)
		fmt.Fprintf(&design, "- Density: `%s`\n", plan.Design.Density)
		fmt.Fprintf(&design, "- Surfaces: `%s`\n", plan.Design.Surfaces)
	}
	files["design-system.md"] = design.String()

	// xcodegen-policy.md
	var xg strings.Builder
	xg.WriteString("# XcodeGen Policy\n\n")
	xg.WriteString("## Required Workflow\n")
	xg.WriteString("- Use xcodegen MCP tools for project configuration changes\n")
	xg.WriteString("- Preferred tools: `add_permission`, `add_extension`, `add_entitlement`, `add_localization`, `add_package`, `set_build_setting`, `get_project_config`, `regenerate_project`\n")
	xg.WriteString("- Do not manually edit `.xcodeproj`\n")
	xg.WriteString("- Avoid manual `project.yml` edits unless explicitly doing emergency recovery, then run `regenerate_project`\n")
	xg.WriteString("\n## Files\n")
	xg.WriteString("- `project_config.json` = editable source of truth\n")
	xg.WriteString("- `project.yml` = generated XcodeGen spec\n")
	xg.WriteString("- `.xcodeproj` = generated output\n")
	files["xcodegen-policy.md"] = xg.String()

	// build-fix-workflow.md
	var workflow strings.Builder
	workflow.WriteString("# Build / Fix Workflow\n\n")
	workflow.WriteString("## Preflight\n")
	workflow.WriteString("1. Run `/preflight`\n")
	workflow.WriteString("2. Confirm MCP readiness and skill integrity\n")
	workflow.WriteString("\n## Build Green Loop\n")
	workflow.WriteString("1. Implement or edit code\n")
	workflow.WriteString("2. Run `/build-green` or `make claude-check`\n")
	workflow.WriteString("3. Fix errors\n")
	workflow.WriteString("4. Repeat until build passes\n")
	workflow.WriteString("\n## Research Policy\n")
	workflow.WriteString("- Use Apple docs MCP first for API signatures and platform compatibility\n")
	workflow.WriteString("- If docs MCP is unavailable or insufficient, use WebFetch/WebSearch official docs fallback\n")
	workflow.WriteString("- `context7` is optional and not required for this project scaffold\n")
	files["build-fix-workflow.md"] = workflow.String()

	// review-playbook.md
	reviewPlaybook := "# Review Playbook\n\n" +
		"## When To Use Each Command\n" +
		"- Use `/quality-review` for project quality gates, structure, previews, wiring, and configuration policy checks\n" +
		"- Use `/accessibility-audit` for focused a11y review (code-first, screenshots optional)\n" +
		"- For UI-heavy changes, run `/quality-review` first and then `/accessibility-audit`\n\n" +
		"## Reporting Format (Required)\n" +
		"- Findings first, ordered by severity\n" +
		"- Use structured Markdown sections exactly as requested by the command\n" +
		"- Include file references for code-based findings (`path:line`)\n" +
		"- Include remediation direction for each finding\n\n" +
		"## Severity Definitions\n" +
		"- Critical: build/runtime breakage, severe accessibility failure, or release-blocking regression\n" +
		"- High: user-facing quality issue likely to ship incorrectly without fix\n" +
		"- Medium: correctness/UX issue with limited scope or workaround\n" +
		"- Low: polish or maintainability issue\n\n" +
		"## Evidence Rules\n" +
		"- Prefer code evidence first\n" +
		"- Use screenshots/images only when provided or necessary for visual validation\n" +
		"- If a visual issue cannot be proven from code, mark it as `needs verification`\n\n" +
		"## Fix Planning\n" +
		"- Prioritize high-confidence, low-blast-radius fixes first\n" +
		"- Re-run relevant local checks after fixes (`check-previews`, `check-no-placeholders`, a11y checks)\n" +
		"- End with exact re-test steps\n\n" +
		"## Staged Enforcement Note\n" +
		"- Phase 1: new a11y static checks are advisory in hooks and fail only in `make claude-check`\n" +
		"- CI does not run the new a11y checks yet\n"
	files["review-playbook.md"] = reviewPlaybook

	// accessibility-policy.md
	accessibilityPolicy := "# Accessibility Policy (Generated SwiftUI Project)\n\n" +
		"- Use SwiftUI system text styles (`.body`, `.headline`, etc.) and avoid fixed font sizes unless explicitly justified\n" +
		"- Icon-only controls must include `.accessibilityLabel(...)`\n" +
		"- Respect Reduce Motion and Reduce Transparency when adding motion or materials\n" +
		"- Prefer semantic accessibility elements/traits over decorative-only UI\n" +
		"- Minimum touch target expectation: 44x44 points\n" +
		"- Do not rely on color alone for status/meaning; pair with text and/or symbols\n" +
		"- Previews should include representative states when feasible (empty/loading/error/content)\n" +
		"- For ambiguous issues, report `needs verification` rather than guessing\n"
	files["accessibility-policy.md"] = accessibilityPolicy

	// quality-gates.md
	quality := "# Quality Gates\n\n" +
		"## Definition of Done\n" +
		"- Canonical build command passes\n" +
		"- No placeholder-only Swift files remain\n" +
		"- No dead feature views (new UI is reachable from app flow)\n" +
		"- Global settings are wired at the root app when they affect app-wide behavior\n" +
		"- New View files include #Preview\n" +
		"- Project configuration changes are done via xcodegen MCP tools\n" +
		"- Extensions compile and include required @main entry points (when applicable)\n" +
		"- Shared app/extension types live in Shared/\n" +
		"- **AppTheme compliance**: no hardcoded colors (use AppTheme.Colors.*), no hardcoded fonts (use AppTheme.Fonts.*), no hardcoded spacing (use AppTheme.Spacing.*)\n" +
		"- **AppTheme completeness**: Colors enum includes textPrimary/textSecondary/textTertiary; Fonts enum exists with plan's fontDesign applied\n\n" +
		"## Local Checks\n" +
		"- make claude-check\n" +
		"- /quality-review\n" +
		"- /accessibility-audit (for UI-heavy changes)\n"
	files["quality-gates.md"] = quality

	// generated-plan.md
	var planDoc strings.Builder
	planDoc.WriteString("# Generated Plan Snapshot\n\n")
	if plan == nil {
		planDoc.WriteString("Planner output has not been written yet. This file will be populated after the planning phase.\n")
	} else {
		planDoc.WriteString("## Platform\n")
		fmt.Fprintf(&planDoc, "- `%s`\n", plan.GetPlatform())
		if IsWatchOS(plan.GetPlatform()) {
			planDoc.WriteString("\n## Watch Project Shape\n")
			fmt.Fprintf(&planDoc, "- `%s`\n", plan.GetWatchProjectShape())
		} else if IsTvOS(plan.GetPlatform()) || IsVisionOS(plan.GetPlatform()) || IsMacOS(plan.GetPlatform()) {
			// tvOS/visionOS/macOS have no device family or watch shape — just platform
		} else {
			planDoc.WriteString("\n## Device Family\n")
			fmt.Fprintf(&planDoc, "- `%s`\n", plan.GetDeviceFamily())
		}
		planDoc.WriteString("\n## Rule Keys\n")
		if len(plan.RuleKeys) == 0 {
			planDoc.WriteString("- None\n")
		} else {
			for _, key := range plan.RuleKeys {
				fmt.Fprintf(&planDoc, "- `%s`\n", key)
			}
		}
		planDoc.WriteString("\n## Models\n")
		if len(plan.Models) == 0 {
			planDoc.WriteString("- None\n")
		} else {
			for _, m := range plan.Models {
				var props []string
				for _, p := range m.Properties {
					props = append(props, fmt.Sprintf("%s: %s", p.Name, p.Type))
				}
				fmt.Fprintf(&planDoc, "- `%s` (%s): %s\n", m.Name, m.Storage, strings.Join(props, ", "))
			}
		}
		planDoc.WriteString("\n## Files (Build Order)\n")
		if len(plan.BuildOrder) == 0 {
			for _, f := range plan.Files {
				fmt.Fprintf(&planDoc, "- `%s` (%s)\n", f.Path, f.TypeName)
			}
		} else {
			for _, p := range plan.BuildOrder {
				fmt.Fprintf(&planDoc, "- `%s`\n", p)
			}
		}
		planDoc.WriteString("\n## Permissions\n")
		if len(plan.Permissions) == 0 {
			planDoc.WriteString("- None\n")
		} else {
			for _, p := range plan.Permissions {
				fmt.Fprintf(&planDoc, "- `%s` (%s): %s\n", p.Key, p.Framework, p.Description)
			}
		}
		planDoc.WriteString("\n## Extensions\n")
		if len(plan.Extensions) == 0 {
			planDoc.WriteString("- None\n")
		} else {
			for _, ext := range plan.Extensions {
				fmt.Fprintf(&planDoc, "- `%s` (%s): %s\n", extensionTargetName(ext, appName), ext.Kind, ext.Purpose)
			}
		}
		planDoc.WriteString("\n## Localizations\n")
		if len(plan.Localizations) == 0 {
			planDoc.WriteString("- None\n")
		} else {
			for _, lang := range plan.Localizations {
				fmt.Fprintf(&planDoc, "- `%s`\n", lang)
			}
		}
		planDoc.WriteString("\n## Packages\n")
		if len(plan.Packages) == 0 {
			planDoc.WriteString("- None\n")
		} else {
			for _, pkg := range plan.Packages {
				fmt.Fprintf(&planDoc, "- `%s`: %s\n", pkg.Name, pkg.Reason)
			}
		}
	}
	files["generated-plan.md"] = planDoc.String()

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(memoryDir, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write memory file %s: %w", name, err)
		}
	}

	return nil
}

// writeClaudeProjectScaffold writes shared Claude Code project files for generated apps.
func writeClaudeProjectScaffold(projectDir, appName, platform string) error {
	return writeClaudeProjectScaffoldWithShape(projectDir, appName, platform, "")
}

func writeClaudeProjectScaffoldWithShape(projectDir, appName, platform, watchProjectShape string) error {
	if err := writeSkillCatalog(projectDir); err != nil {
		return err
	}
	if err := writeClaudeCommandsWithShape(projectDir, appName, platform, watchProjectShape); err != nil {
		return err
	}
	if err := writeClaudeAgents(projectDir); err != nil {
		return err
	}
	if err := writeClaudeScriptsWithShape(projectDir, appName, platform, watchProjectShape); err != nil {
		return err
	}
	if err := writeClaudeWorkflowDocsWithShape(projectDir, appName, platform, watchProjectShape); err != nil {
		return err
	}
	if err := writeSettingsShared(projectDir, nil); err != nil {
		return err
	}
	if err := writeProjectMakefileWithShape(projectDir, appName, platform, watchProjectShape); err != nil {
		return err
	}
	if err := writeCIWorkflowWithShape(projectDir, appName, platform, watchProjectShape); err != nil {
		return err
	}
	return nil
}

func writeClaudeCommands(projectDir, appName, platform string) error {
	return writeClaudeCommandsWithShape(projectDir, appName, platform, "")
}

func writeClaudeCommandsWithShape(projectDir, appName, platform, watchProjectShape string) error {
	buildCmd := canonicalBuildCommandForShape(appName, platform, watchProjectShape)
	cmdDir := filepath.Join(projectDir, ".claude", "commands")
	files := map[string]string{
		"preflight.md": `---
description: Run generated-project preflight checks for Claude Code workflows.
---
Run the preflight workflow for this iOS project.

1. Read @memory/project-overview.md and @memory/build-fix-workflow.md.
2. Run ./scripts/claude/mcp-health.sh.
3. Run ./scripts/claude/validate-skills.sh.
4. Summarize actionable issues only.
`,
		"build-green.md": fmt.Sprintf(`---
description: Build-fix loop until the iOS project builds successfully.
---
Use the quality-first build loop for this project.

1. Read @memory/build-fix-workflow.md and @memory/quality-gates.md.
2. Run ./scripts/claude/check-no-placeholders.sh and ./scripts/claude/check-previews.sh.
3. Run this build command:
   %s
4. If the build fails, investigate and fix the errors.
5. Repeat until the build is green.
6. Finish with a short summary of fixes and residual risks.
`, buildCmd),
		"fix-build.md": fmt.Sprintf(`---
description: Focused build error fixing using project memory and xcodegen policy.
---
Fix current build issues only.

1. Read @memory/xcodegen-policy.md and @memory/build-fix-workflow.md.
2. Run this build command first:
   %s
3. Fix the errors with minimal changes.
4. If project configuration must change, use xcodegen MCP tools.
5. Rebuild until green.
`, buildCmd),
		"add-feature.md": `---
description: Add a feature using generated project memory, skills, and quality checks.
---
Add the requested feature while preserving project quality.

1. Read @memory/generated-plan.md, @memory/architecture.md, and @memory/design-system.md.
2. Load relevant skills from .claude/skills and consult .claude/skills/INDEX.md.
3. If permissions/extensions/entitlements are needed, use xcodegen MCP tools.
4. Implement the feature end-to-end with previews and reachable navigation wiring.
5. Run make claude-check.
6. Summarize what changed and any follow-up work.
`,
		"xcodegen-change.md": fmt.Sprintf(`---
description: Perform project configuration changes through xcodegen MCP tools only.
---
Handle project configuration changes via xcodegen MCP tools only.

1. Read @memory/xcodegen-policy.md.
2. Use get_project_config to inspect current state.
3. Apply changes using xcodegen MCP tools.
4. Run regenerate_project if needed.
5. Verify with:
   %s
6. Summarize the configuration diff and affected targets.
`, buildCmd),
		"quality-review.md": `---
description: Review generated iOS project quality gates and report findings.
---
Perform a quality review of the current project state (not a full accessibility audit unless requested).

Actions:
1. Read @memory/review-playbook.md, @memory/quality-gates.md, and @memory/design-system.md.
2. Load the review skill from .claude/skills (see .claude/skills/INDEX.md).
3. Run ./scripts/claude/check-no-placeholders.sh.
4. Run ./scripts/claude/check-previews.sh.
5. Run ./scripts/claude/check-swift-structure.sh.
6. If the user supplied screenshots/paths, include an optional visual pass (code evidence still takes priority).
7. Report using structured Markdown with these exact sections:
   - ## Scope
   - ## Findings (severity-ordered)
   - ## Fix Plan
   - ## Verification Steps
   - ## Escalation
8. In ## Escalation, explicitly state whether /accessibility-audit is recommended.

Findings requirements:
- One finding per bullet
- Include severity (Critical/High/Medium/Low)
- Include evidence with file path and line when code-based
- Include remediation direction (not just symptom)
`,
		"accessibility-audit.md": `---
description: Perform a focused accessibility audit for generated SwiftUI apps (code-first, screenshots optional).
---
Run a focused accessibility audit for the current project or the requested files/screens.

Actions:
1. Read @memory/accessibility-policy.md and @memory/review-playbook.md.
2. Load relevant skills from .claude/skills/INDEX.md (especially accessibility/review guidance).
3. Run ./scripts/claude/check-a11y-dynamic-type.sh.
4. Run ./scripts/claude/check-a11y-icon-buttons.sh.
5. Review code evidence first; use screenshots/images only if provided for visual cues (contrast/tap target/hierarchy hints).
6. Report using structured Markdown with these exact sections:
   - ## Scope
   - ## Checklist Coverage
   - ## Findings (severity-ordered)
   - ## Remediation Plan
   - ## Re-test Steps
   - ## Open Questions (only if required)

Checklist coverage categories (fixed):
- Dynamic Type / text scaling
- VoiceOver labels/hints/traits
- Reduce Motion / Reduce Transparency
- Touch targets and interaction clarity
- Color/contrast (code-evidence and visual hints)
- Focus/form navigation (if forms exist)
- Status/feedback semantics (if present)
`,
		"research-apple-api.md": `---
description: Research Apple APIs with Apple docs MCP first, then official web fallback.
---
Research the requested Apple API/framework carefully.

Process:
1. Use Apple docs MCP tools first.
2. If unavailable or insufficient, use WebFetch/WebSearch on official Apple sources.
3. Do not guess API signatures.
4. Return exact API names, platform notes, and integration guidance for this project.

Note: context7 is optional for this project scaffold and not required.
`,
	}
	for name, content := range files {
		if err := writeTextFile(filepath.Join(cmdDir, name), content, 0o644); err != nil {
			return fmt.Errorf("failed to write command %s: %w", name, err)
		}
	}
	return nil
}

func writeClaudeAgents(projectDir string) error {
	agentsDir := filepath.Join(projectDir, ".claude", "agents")
	files := map[string]string{
		"ios-api-researcher.md": `---
name: ios-api-researcher
description: Verifies Apple API signatures, platform support, and usage details before implementation.
tools:
  - Read
  - WebFetch
  - WebSearch
  - mcp__apple-docs__search_apple_docs
  - mcp__apple-docs__get_apple_doc_content
  - mcp__apple-docs__search_framework_symbols
  - mcp__apple-docs__get_platform_compatibility
model: sonnet
---
You are the Apple API research specialist for this project.

Prefer Apple docs MCP first. Fall back to official web docs when MCP is unavailable.
Do not edit project files unless explicitly asked.
`,
		"xcodegen-config-specialist.md": `---
name: xcodegen-config-specialist
description: Handles XcodeGen project configuration changes via MCP tools with minimal risk.
tools:
  - Read
  - mcp__xcodegen__get_project_config
  - mcp__xcodegen__add_permission
  - mcp__xcodegen__add_extension
  - mcp__xcodegen__add_entitlement
  - mcp__xcodegen__add_localization
  - mcp__xcodegen__set_build_setting
  - mcp__xcodegen__regenerate_project
model: sonnet
---
You are the XcodeGen configuration specialist.

Use xcodegen MCP tools only. Avoid manual .xcodeproj edits. Summarize target-level effects.
`,
		"swiftui-quality-reviewer.md": `---
name: swiftui-quality-reviewer
description: Reviews SwiftUI output for previews, theme usage, adaptive layout, and dead code risks.
tools:
  - Read
  - Grep
  - Glob
  - Bash
model: sonnet
---
You are the SwiftUI quality reviewer.

Focus on project quality gates: previews, theme token usage, adaptive layout, dead code, and root wiring for app-wide settings.
If the change is UI-heavy or needs deeper accessibility analysis, recommend or delegate to swiftui-accessibility-reviewer.
Return findings first, ordered by severity.
`,
		"swiftui-accessibility-reviewer.md": `---
name: swiftui-accessibility-reviewer
description: Performs code-first accessibility audits for generated SwiftUI apps, with optional screenshot review.
tools:
  - Read
  - Grep
  - Glob
  - Bash
  - ViewImage
model: sonnet
---
You are the SwiftUI accessibility reviewer.

Audit code first and prefer file-based evidence. Use screenshots only when provided or needed for visual validation.
Return findings first, ordered by severity. Mark visual-only uncertainty as "needs verification".
Do not edit files unless explicitly asked.
`,
		"test-scaffold-writer.md": `---
name: test-scaffold-writer
description: Creates minimal, low-risk tests or checks for generated projects.
tools:
  - Read
  - Write
  - Edit
  - Bash
model: sonnet
---
Create deterministic, low-risk checks and tests with small diffs.
Prefer fast checks first and avoid destabilizing app code.
`,
	}
	for name, content := range files {
		if err := writeTextFile(filepath.Join(agentsDir, name), content, 0o644); err != nil {
			return fmt.Errorf("failed to write agent %s: %w", name, err)
		}
	}
	return nil
}

func writeClaudeWorkflowDocs(projectDir, appName, platform string) error {
	return writeClaudeWorkflowDocsWithShape(projectDir, appName, platform, "")
}

func writeClaudeWorkflowDocsWithShape(projectDir, appName, platform, watchProjectShape string) error {
	buildCmd := canonicalBuildCommandForShape(appName, platform, watchProjectShape)
	content := fmt.Sprintf(`# Claude Workflow (Generated Project)

This project is generated with a quality-first Claude Code scaffold.

## Shared Files (Commit)
- .claude/CLAUDE.md
- .claude/memory/
- .claude/skills/
- .claude/commands/
- .claude/agents/
- .claude/settings.json
- .mcp.json
- scripts/claude/
- docs/claude-workflow.md

## Local-Only Files (Ignored)
- .claude/settings.local.json
- .claude/logs/
- .claude/tmp/
- .claude/transcripts/

## Canonical Build Command

%s

## Recommended Commands
- /preflight
- /build-green
- /fix-build
- /xcodegen-change
- /quality-review
- /accessibility-audit
- /research-apple-api

## Review & Accessibility Workflow
- Run /quality-review before major refactors or before handing off a feature for QA
- Run /accessibility-audit for UI-heavy changes, new forms, custom controls, or motion-heavy interfaces
- Accessibility audits are code-first; add screenshot/image paths when you want a visual pass
- Phase 1 enforcement: a11y hooks are advisory, but make claude-check fails on the new local a11y checks
- CI intentionally does not run the new a11y checks yet (staged rollout)

## Project Config Policy
- Use xcodegen MCP tools for project configuration changes
- Do not manually edit .xcodeproj
- Manual project.yml edits should be rare and followed by regenerate_project

## Research Policy
- Apple docs MCP first
- Official web docs fallback when MCP is unavailable
- context7 is optional (not required for this scaffold)

## Optional context7 (Later)
If your environment supports context7, add it to .mcp.json and verify with scripts/claude/mcp-health.sh.
Keep Apple docs MCP plus web fallback as the default recovery path.
`, buildCmd)
	return writeTextFile(filepath.Join(projectDir, "docs", "claude-workflow.md"), content, 0o644)
}

func writeProjectMakefile(projectDir, appName, platform string) error {
	return writeProjectMakefileWithShape(projectDir, appName, platform, "")
}

func writeProjectMakefileWithShape(projectDir, appName, platform, watchProjectShape string) error {
	buildCmd := canonicalBuildCommandForShape(appName, platform, watchProjectShape)
	destination := canonicalBuildDestinationForShape(platform, watchProjectShape)
	content := fmt.Sprintf(".PHONY: build test claude-check mcp-health skills-validate\n\nbuild:\n\t%s\n\nmcp-health:\n\t./scripts/claude/mcp-health.sh\n\nskills-validate:\n\t./scripts/claude/validate-skills.sh\n\nclaude-check:\n\t-./scripts/claude/mcp-health.sh\n\tplutil -lint .mcp.json >/dev/null\n\tplutil -lint .claude/settings.json >/dev/null\n\t./scripts/claude/validate-skills.sh\n\t./scripts/claude/check-no-placeholders.sh\n\t./scripts/claude/check-previews.sh\n\t./scripts/claude/check-swift-structure.sh\n\t./scripts/claude/check-a11y-dynamic-type.sh\n\t./scripts/claude/check-a11y-icon-buttons.sh\n\t./scripts/claude/check-project-config-edits.sh --scan || true\n\t./scripts/claude/run-build-check.sh\n\ntest:\n\t@if [ -d Tests ]; then \\\n\t\techo \"Tests directory found; running xcodebuild test\"; \\\n\t\txcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet test; \\\n\telse \\\n\t\techo \"No Tests directory present; skipping tests\"; \\\n\tfi\n", buildCmd, appName, appName, destination)
	return writeTextFile(filepath.Join(projectDir, "Makefile"), content, 0o644)
}

func writeCIWorkflow(projectDir, appName, platform string) error {
	return writeCIWorkflowWithShape(projectDir, appName, platform, "")
}

func writeCIWorkflowWithShape(projectDir, appName, platform, watchProjectShape string) error {
	destination := canonicalBuildDestinationForShape(platform, watchProjectShape)
	content := fmt.Sprintf(`name: Claude Quality

on:
  pull_request:
  push:
    branches: [ main ]

jobs:
  quality:
    runs-on: macos-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Install XcodeGen
        run: brew install xcodegen

      - name: Validate Claude JSON config files
        run: |
          plutil -lint .mcp.json
          plutil -lint .claude/settings.json

      - name: Ensure scripts are executable
        run: chmod +x scripts/claude/*.sh

      - name: Validate skills
        run: ./scripts/claude/validate-skills.sh

      - name: Placeholder check
        run: ./scripts/claude/check-no-placeholders.sh

      - name: Build check
        run: ./scripts/claude/run-build-check.sh

      - name: Optional tests (if present)
        run: |
          if [ -d Tests ]; then
            xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet test
          else
            echo "No Tests directory present; skipping tests"
          fi
`, appName, appName, destination)
	return writeTextFile(filepath.Join(projectDir, ".github", "workflows", "claude-quality.yml"), content, 0o644)
}

func writeClaudeScripts(projectDir, appName, platform string) error {
	return writeClaudeScriptsWithShape(projectDir, appName, platform, "")
}

func writeClaudeScriptsWithShape(projectDir, appName, platform, watchProjectShape string) error {
	scriptsDir := filepath.Join(projectDir, "scripts", "claude")
	buildCmd := canonicalBuildCommandForShape(appName, platform, watchProjectShape)
	files := map[string]string{
		"check-project-config-edits.sh": `#!/bin/sh
set -eu

MODE="${1:-hook}"
INPUT=""
if [ ! -t 0 ]; then
  INPUT=$(cat || true)
fi

warn() {
  printf '%s\n' "$1" >&2
}

if [ "$MODE" = "--scan" ]; then
  if ! command -v git >/dev/null 2>&1; then
    exit 0
  fi
  if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    exit 0
  fi
  CHANGED=$(git diff --name-only 2>/dev/null || true)
  printf '%s\n' "$CHANGED" | grep -Eq '(^|/)(project\.yml|.*\.xcodeproj/)' || exit 0
  warn "[claude-check] Advisory: project.yml or .xcodeproj changes detected. Prefer xcodegen MCP tools for configuration changes."
  exit 0
fi

printf '%s' "$INPUT" | grep -Eq '"tool_name"[[:space:]]*:[[:space:]]*"(Write|Edit|MultiEdit)"' || exit 0
printf '%s' "$INPUT" | grep -Eq '(project\.yml|\.xcodeproj/)' || exit 0
warn "[claude-hook] Advisory: direct edits to project.yml/.xcodeproj detected. Prefer xcodegen MCP tools (add_permission/add_extension/etc.)."
exit 0
`,
		"check-bash-safety.sh": `#!/bin/sh
set -eu

INPUT=""
if [ ! -t 0 ]; then
  INPUT=$(cat || true)
fi

warn() {
  printf '%s\n' "$1" >&2
}

printf '%s' "$INPUT" | grep -Eq '"tool_name"[[:space:]]*:[[:space:]]*"Bash"' || exit 0
if printf '%s' "$INPUT" | grep -Eq '(git reset --hard|git checkout --|rm -rf)'; then
  warn "[claude-hook] Advisory: potentially destructive shell command detected. Confirm intent before continuing."
fi
exit 0
`,
		"check-no-placeholders.sh": `#!/bin/sh
set -eu

HOOK_MODE=0
if [ "${1:-}" = "--hook" ]; then
  HOOK_MODE=1
fi

ROOT="${CLAUDE_PROJECT_DIR:-$(pwd)}"
cd "$ROOT"

MATCHES=$(find . -type f -name '*.swift' \
  -not -path './.git/*' \
  -not -path './.build/*' \
  -not -path './DerivedData/*' \
  -exec grep -nH 'Placeholder.*replaced by generated code' {} + 2>/dev/null || true)

if [ -n "$MATCHES" ]; then
  printf '%s\n' "$MATCHES" >&2
  printf '%s\n' "[claude-check] Placeholder Swift files still exist. Replace scaffolding placeholders before completion." >&2
  if [ "$HOOK_MODE" -eq 1 ]; then
    exit 0
  fi
  exit 1
fi
exit 0
`,
		"check-previews.sh": `#!/bin/sh
set -eu

HOOK_MODE=0
if [ "${1:-}" = "--hook" ]; then
  HOOK_MODE=1
fi

ROOT="${CLAUDE_PROJECT_DIR:-$(pwd)}"
cd "$ROOT"

if [ "$HOOK_MODE" -eq 1 ] && [ ! -t 0 ]; then
  INPUT=$(cat || true)
  FILE=$(printf '%s' "$INPUT" | tr '\n' ' ' | sed -n 's/.*"file_path"[[:space:]]*:[[:space:]]*"\([^"]*\.swift\)".*/\1/p')
  if [ -n "${FILE:-}" ] && [ -f "$FILE" ]; then
    CANDIDATES="$FILE"
  else
    CANDIDATES=""
  fi
else
  CANDIDATES=""
fi

if [ -z "$CANDIDATES" ]; then
  CANDIDATES=$(find App Features Targets Shared -type f -name '*.swift' 2>/dev/null || true)
fi

MISSING=""
for f in $CANDIDATES; do
  [ -f "$f" ] || continue
  if grep -Eq 'struct[[:space:]]+[A-Za-z0-9_]+[[:space:]]*:[[:space:]]*View' "$f"; then
    if ! grep -q '#Preview' "$f"; then
      MISSING="${MISSING}${f}\n"
    fi
  fi
done

if [ -n "$MISSING" ]; then
  printf '%b' "$MISSING" >&2
  printf '%s\n' "[claude-check] View files without #Preview detected (advisory in hook mode)." >&2
  if [ "$HOOK_MODE" -eq 1 ]; then
    exit 0
  fi
  exit 1
fi
exit 0
`,
		"check-swift-structure.sh": `#!/bin/sh
set -eu

ROOT="${CLAUDE_PROJECT_DIR:-$(pwd)}"
cd "$ROOT"

FILES=""
if [ ! -t 0 ]; then
  INPUT=$(cat || true)
  FILE=$(printf '%s' "$INPUT" | tr '\n' ' ' | sed -n 's/.*"file_path"[[:space:]]*:[[:space:]]*"\([^"]*\.swift\)".*/\1/p')
  if [ -n "${FILE:-}" ] && [ -f "$FILE" ]; then
    FILES="$FILE"
  fi
fi
if [ -z "$FILES" ]; then
  FILES=$(find App Features Targets Shared -type f -name '*.swift' 2>/dev/null || true)
fi

for f in $FILES; do
  [ -f "$f" ] || continue
  if [ ! -s "$f" ]; then
    printf '%s\n' "[claude-hook] Advisory: empty Swift file detected: $f" >&2
    continue
  fi
  if grep -Eq 'TODO|FIXME' "$f"; then
    printf '%s\n' "[claude-hook] Advisory: TODO/FIXME found in $f" >&2
  fi
done
exit 0
`,
		"check-a11y-dynamic-type.sh": `#!/bin/sh
set -eu

HOOK_MODE=0
if [ "${1:-}" = "--hook" ]; then
  HOOK_MODE=1
fi

ROOT="${CLAUDE_PROJECT_DIR:-$(pwd)}"
cd "$ROOT"

FILES=$(find App Features Targets Shared -type f -name '*.swift' 2>/dev/null || true)
MATCHES=""
for f in $FILES; do
  [ -f "$f" ] || continue
  HITS=$(grep -nH -E '\.font[[:space:]]*\([[:space:]]*\.system[[:space:]]*\([[:space:]]*size[[:space:]]*:' "$f" 2>/dev/null | grep -v 'claude-a11y:ignore fixed-font' || true)
  if [ -n "$HITS" ]; then
    MATCHES="${MATCHES}${HITS}\n"
  fi
done

if [ -n "$MATCHES" ]; then
  printf '%b' "$MATCHES" >&2
  if [ "$HOOK_MODE" -eq 1 ]; then
    printf '%s\n' "[claude-hook] Advisory: fixed font size usage detected. Prefer SwiftUI text styles (.body/.headline/etc.) or add 'claude-a11y:ignore fixed-font' with justification." >&2
    exit 0
  fi
  printf '%s\n' "[claude-check] Fixed font size usage detected. Prefer Dynamic Type-friendly SwiftUI text styles or add 'claude-a11y:ignore fixed-font' with justification." >&2
  exit 1
fi
exit 0
`,
		"check-a11y-icon-buttons.sh": `#!/bin/sh
set -eu

HOOK_MODE=0
if [ "${1:-}" = "--hook" ]; then
  HOOK_MODE=1
fi

ROOT="${CLAUDE_PROJECT_DIR:-$(pwd)}"
cd "$ROOT"

FILES=$(find App Features Targets Shared -type f -name '*.swift' 2>/dev/null || true)
TMP_OUT=$(mktemp)
trap 'rm -f "$TMP_OUT"' EXIT

for f in $FILES; do
  [ -f "$f" ] || continue
  awk '
    { lines[NR] = $0 }
    END {
      for (i = 1; i <= NR; i++) {
        if (lines[i] !~ /Image[[:space:]]*\([[:space:]]*systemName:[[:space:]]*"/) {
          continue
        }
        start = i - 4
        if (start < 1) start = 1
        stop = i + 12
        if (stop > NR) stop = NR

        hasButton = 0
        hasLabel = 0
        hasText = 0
        ignore = 0
        for (j = start; j <= stop; j++) {
          line = lines[j]
          if (line ~ /claude-a11y:ignore icon-button-label/) ignore = 1
          if (line ~ /Button[[:space:]]*(\(|\{)/) hasButton = 1
          if (line ~ /\.accessibilityLabel[[:space:]]*\(/) hasLabel = 1
          if (line ~ /Text[[:space:]]*\(|Label[[:space:]]*\(/) hasText = 1
        }

        if (hasButton && !hasLabel && !hasText && !ignore) {
          printf "%s:%d: icon-only button may be missing accessibilityLabel\n", FILENAME, i
        }
      }
    }
  ' "$f" >> "$TMP_OUT"
done

if [ -s "$TMP_OUT" ]; then
  cat "$TMP_OUT" >&2
  if [ "$HOOK_MODE" -eq 1 ]; then
    printf '%s\n' "[claude-hook] Advisory: icon-only button without accessibilityLabel detected. Add .accessibilityLabel(...) or annotate with 'claude-a11y:ignore icon-button-label' and justification." >&2
    exit 0
  fi
  printf '%s\n' "[claude-check] Icon-only button without accessibilityLabel detected. Add .accessibilityLabel(...) or annotate with 'claude-a11y:ignore icon-button-label' and justification." >&2
  exit 1
fi
exit 0
`,
		"run-build-check.sh": fmt.Sprintf(`#!/bin/sh
set -eu

HOOK_MODE=0
if [ "${1:-}" = "--hook" ]; then
  HOOK_MODE=1
fi

ROOT="${CLAUDE_PROJECT_DIR:-$(pwd)}"
cd "$ROOT"

APP_NAME="%s"
BUILD_CMD="%s"
LOGFILE="${TMPDIR:-/tmp}/claude-build-check.log"

warn() {
  printf '%%s\n' "$1" >&2
}

if ! command -v xcodebuild >/dev/null 2>&1; then
  warn "[claude-build] xcodebuild not found; skipping build check."
  [ "$HOOK_MODE" -eq 1 ] && exit 0
  exit 1
fi

if [ ! -d "$APP_NAME.xcodeproj" ]; then
  warn "[claude-build] $APP_NAME.xcodeproj not found; skipping build check."
  [ "$HOOK_MODE" -eq 1 ] && exit 0
  exit 1
fi

if [ "$HOOK_MODE" -eq 1 ]; then
  if ! sh -c "$BUILD_CMD" >"$LOGFILE" 2>&1; then
    warn "[claude-hook] Advisory: build check failed. Inspect $LOGFILE or run /build-green."
  fi
  exit 0
fi

exec sh -c "$BUILD_CMD"
`, appName, buildCmd),
		"mcp-health.sh": `#!/bin/sh
set -eu

ROOT="${CLAUDE_PROJECT_DIR:-$(pwd)}"
cd "$ROOT"

FAIL=0
warn() { printf '%s\n' "$1" >&2; }

if [ ! -f .mcp.json ]; then
  warn "[mcp-health] .mcp.json not found"
  exit 1
fi

if command -v plutil >/dev/null 2>&1; then
  if ! plutil -lint .mcp.json >/dev/null; then
    warn "[mcp-health] .mcp.json is not valid JSON"
    FAIL=1
  fi
fi

for cmd in xcodegen npx nanowave; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    warn "[mcp-health] Missing required command in PATH: $cmd"
    FAIL=1
  fi
done

if [ "$FAIL" -eq 0 ]; then
  printf '%s\n' "[mcp-health] OK (command availability + config syntax)"
  exit 0
fi
exit 1
`,
		"validate-skills.sh": `#!/bin/sh
set -eu

ROOT="${CLAUDE_PROJECT_DIR:-$(pwd)}"
cd "$ROOT"

SKILLS_DIR=".claude/skills"
if [ ! -d "$SKILLS_DIR" ]; then
  printf '%s\n' "[validate-skills] skills directory not found: $SKILLS_DIR" >&2
  exit 1
fi

TMP_NAMES=$(mktemp)
trap 'rm -f "$TMP_NAMES"' EXIT
FAIL=0

has_top_toc() {
  f="$1"
  awk '
    NR > 30 { exit }
    {
      line = tolower($0)
      if (line ~ /^[[:space:]]*#{1,6}[[:space:]]+contents[[:space:]]*$/ || line ~ /table of contents/) {
        found = 1
        exit
      }
    }
    END { exit found ? 0 : 1 }
  ' "$f"
}

for d in "$SKILLS_DIR"/*; do
  [ -d "$d" ] || continue
  SKILL_MD="$d/SKILL.md"
  if [ ! -f "$SKILL_MD" ]; then
    printf '%s\n' "[validate-skills] Missing SKILL.md in $d" >&2
    FAIL=1
    continue
  fi
  if [ ! -s "$SKILL_MD" ]; then
    printf '%s\n' "[validate-skills] Empty SKILL.md in $d" >&2
    FAIL=1
    continue
  fi
  if ! head -1 "$SKILL_MD" | grep -q '^---$'; then
    printf '%s\n' "[validate-skills] Missing YAML frontmatter in $SKILL_MD" >&2
    FAIL=1
  fi
  for field in name description; do
    if ! grep -Eq "^${field}:" "$SKILL_MD"; then
      printf '%s\n' "[validate-skills] Missing frontmatter field '${field}' in $SKILL_MD" >&2
      FAIL=1
    fi
  done

  FM_KEYS=$(awk '
    BEGIN { in_fm = 0 }
    NR == 1 && $0 == "---" { in_fm = 1; next }
    in_fm && $0 == "---" { exit }
    in_fm {
      line = $0
      sub(/^[[:space:]]+/, "", line)
      if (line == "" || line ~ /^#/) next
      if (line ~ /^[A-Za-z0-9_-]+:[[:space:]]*/) {
        key = line
        sub(/:.*/, "", key)
        print key
      }
    }
  ' "$SKILL_MD")
  for key in $FM_KEYS; do
    case "$key" in
      name|description) ;;
      *)
        printf '%s\n' "[validate-skills] Unsupported frontmatter field '$key' in $SKILL_MD (Anthropic skills use only name + description)" >&2
        FAIL=1
        ;;
    esac
  done

  NAME=$(sed -n 's/^name:[[:space:]]*//p' "$SKILL_MD" | head -1 | sed "s/^['\"]//; s/['\"]$//")
  if [ -n "$NAME" ]; then
    printf '%s\n' "$NAME" >> "$TMP_NAMES"
    if ! printf '%s' "$NAME" | grep -Eq '^[a-z0-9-]{1,64}$'; then
      printf '%s\n' "[validate-skills] Invalid skill name in $SKILL_MD (must match ^[a-z0-9-]{1,64}$): $NAME" >&2
      FAIL=1
    fi
    case "$NAME" in
      *anthropic*|*claude*)
        printf '%s\n' "[validate-skills] Reserved term used in skill name in $SKILL_MD: $NAME" >&2
        FAIL=1
        ;;
    esac
  fi

  DESCRIPTION=$(sed -n 's/^description:[[:space:]]*//p' "$SKILL_MD" | head -1 | sed "s/^['\"]//; s/['\"]$//")
  if [ -z "$DESCRIPTION" ]; then
    printf '%s\n' "[validate-skills] Empty description in $SKILL_MD" >&2
    FAIL=1
  else
    DESC_LEN=$(printf '%s' "$DESCRIPTION" | wc -c | tr -d ' ')
    if [ "$DESC_LEN" -gt 1024 ]; then
      printf '%s\n' "[validate-skills] Description too long in $SKILL_MD ($DESC_LEN > 1024 chars)" >&2
      FAIL=1
    fi
    if printf '%s' "$DESCRIPTION" | grep -Eq '<[^>]+>'; then
      printf '%s\n' "[validate-skills] Description contains XML/HTML-like markup in $SKILL_MD" >&2
      FAIL=1
    fi
    if ! printf '%s' "$DESCRIPTION" | grep -Eqi 'use when'; then
      printf '%s\n' "[validate-skills] Description should include a 'Use when ...' clause in $SKILL_MD" >&2
      FAIL=1
    fi
  fi

  BODY_HAS_CONTENT=$(awk '
    BEGIN { in_fm = 0; fm_done = 0; has = 0 }
    NR == 1 && $0 == "---" { in_fm = 1; next }
    in_fm && $0 == "---" { in_fm = 0; fm_done = 1; next }
    !in_fm && fm_done {
      line = $0
      gsub(/[[:space:]]/, "", line)
      if (length(line) > 0) { has = 1; exit }
    }
    END { if (has) print "1"; else print "0" }
  ' "$SKILL_MD")
  if [ "$BODY_HAS_CONTENT" != "1" ]; then
    printf '%s\n' "[validate-skills] Skill body is empty in $SKILL_MD" >&2
    FAIL=1
  fi
  BODY_LINES=$(awk '
    BEGIN { in_fm = 0; fm_done = 0; n = 0 }
    NR == 1 && $0 == "---" { in_fm = 1; next }
    in_fm && $0 == "---" { in_fm = 0; fm_done = 1; next }
    !in_fm && fm_done { n++ }
    END { print n }
  ' "$SKILL_MD")
  if [ "$BODY_LINES" -ge 500 ]; then
    printf '%s\n' "[validate-skills] SKILL.md body must be <500 lines in strict mode: $SKILL_MD (got $BODY_LINES)" >&2
    FAIL=1
  fi

  LINKS=$(grep -Eo '\[[^]]+\]\([^)]+\.md\)' "$SKILL_MD" | sed -E 's/.*\(([^)]+)\)/\1/' || true)
  for link in $LINKS; do
    case "$link" in
      http*|/*|*#*)
        continue
        ;;
    esac
    case "$link" in
      *\\*)
        printf '%s\n' "[validate-skills] Local markdown links must use forward slashes in $SKILL_MD: $link" >&2
        FAIL=1
        continue
        ;;
    esac
    if [ ! -f "$d/$link" ]; then
      printf '%s\n' "[validate-skills] Referenced companion file missing: $d/$link (from $SKILL_MD)" >&2
      FAIL=1
    fi
  done

  for ref_md in $(find "$d" -type f -name '*.md' ! -name 'SKILL.md' 2>/dev/null); do
    REF_LINES=$(wc -l < "$ref_md" | tr -d ' ')
    if [ "$REF_LINES" -gt 100 ] && ! has_top_toc "$ref_md"; then
      printf '%s\n' "[validate-skills] Reference file >100 lines must include a top-of-file TOC/Contents section: $ref_md" >&2
      FAIL=1
    fi

    REF_LINKS=$(grep -Eo '\[[^]]+\]\([^)]+\)' "$ref_md" | sed -E 's/.*\(([^)]+)\)/\1/' || true)
    for ref_link in $REF_LINKS; do
      case "$ref_link" in
        http*|/*|#*)
          continue
          ;;
      esac
      case "$ref_link" in
        *\\*)
          printf '%s\n' "[validate-skills] Local markdown links must use forward slashes in $ref_md: $ref_link" >&2
          FAIL=1
          continue
          ;;
      esac
      case "$ref_link" in
        *.md)
          printf '%s\n' "[validate-skills] Nested local markdown links are not allowed in reference files (one-level reference rule): $ref_md -> $ref_link" >&2
          FAIL=1
          ;;
      esac
    done
  done
done

DUPES=$(sort "$TMP_NAMES" | uniq -d || true)
if [ -n "$DUPES" ]; then
  printf '%s\n' "$DUPES" | while IFS= read -r n; do
    [ -n "$n" ] && printf '%s\n' "[validate-skills] Duplicate skill name in frontmatter: $n" >&2
  done
  FAIL=1
fi

if [ "$FAIL" -ne 0 ]; then
  exit 1
fi
printf '%s\n' "[validate-skills] OK"
exit 0
`,
	}

	for name, content := range files {
		if err := writeExecutableFile(filepath.Join(scriptsDir, name), content); err != nil {
			return fmt.Errorf("failed to write script %s: %w", name, err)
		}
	}
	return nil
}
