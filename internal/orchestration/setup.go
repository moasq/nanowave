package orchestration

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed skills
var skillsFS embed.FS

// setupWorkspace creates the project directory and .claude/ structure.
func setupWorkspace(projectDir string) error {
	dirs := []string{
		projectDir,
		filepath.Join(projectDir, ".claude", "rules"),
		filepath.Join(projectDir, ".claude", "skills"),
		filepath.Join(projectDir, ".claude", "memory"),
		filepath.Join(projectDir, ".claude", "commands"),
		filepath.Join(projectDir, ".claude", "agents"),
		filepath.Join(projectDir, "scripts", "claude"),
		filepath.Join(projectDir, "docs"),
		filepath.Join(projectDir, ".github", "workflows"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}
	return nil
}

// writeInitialCLAUDEMD writes the CLAUDE.md with project-specific info only (before plan exists).
// CLAUDE.md is a thin index that imports shared project memory modules and core rules.
func writeInitialCLAUDEMD(projectDir, appName, platform, deviceFamily string) error {
	if err := writeClaudeMemoryFiles(projectDir, appName, platform, deviceFamily, nil); err != nil {
		return err
	}
	return writeCLAUDEMDIndex(projectDir, appName)
}

// enrichCLAUDEMD updates memory modules with plan-specific details after Phase 3.
func enrichCLAUDEMD(projectDir string, plan *PlannerResult, appName string) error {
	if err := writeClaudeMemoryFiles(projectDir, appName, plan.GetPlatform(), plan.GetDeviceFamily(), plan); err != nil {
		return err
	}
	return writeCLAUDEMDIndex(projectDir, appName)
}

func platformSummary(platform, deviceFamily string) string {
	if IsWatchOS(platform) {
		return "Apple Watch, watchOS 26+, Swift 6"
	}
	if IsTvOS(platform) {
		return "Apple TV, tvOS 26+, Swift 6"
	}
	switch deviceFamily {
	case "ipad":
		return "iPad only, iOS 26+, Swift 6"
	case "universal":
		return "iPhone and iPad, iOS 26+, Swift 6"
	default:
		return "iPhone only, iOS 26+, Swift 6"
	}
}

func canonicalBuildDestinationForShape(platform, watchProjectShape string) string {
	if IsWatchOS(platform) {
		// Paired iPhone+Watch projects use an iOS app scheme as the primary executable.
		// Building against an iOS simulator destination avoids watch-only destination errors.
		if watchProjectShape == WatchShapePaired {
			return "generic/platform=iOS Simulator"
		}
		return "generic/platform=watchOS Simulator"
	}
	if IsTvOS(platform) {
		return "generic/platform=tvOS Simulator"
	}
	return "generic/platform=iOS Simulator"
}

func canonicalBuildCommandForShape(appName, platform, watchProjectShape string) string {
	destination := canonicalBuildDestinationForShape(platform, watchProjectShape)
	return fmt.Sprintf("xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet build", appName, appName, destination)
}

func canonicalBuildCommand(appName, platform string) string {
	return canonicalBuildCommandForShape(appName, platform, "")
}

// multiPlatformBuildCommands returns build commands for each platform scheme.
func multiPlatformBuildCommands(appName string, platforms []string) []string {
	var cmds []string
	for _, plat := range platforms {
		var scheme, destination string
		switch plat {
		case PlatformTvOS:
			scheme = appName + "TV"
			destination = PlatformBuildDestination(PlatformTvOS)
		case PlatformWatchOS:
			// In multi-platform, watchOS is built via the iOS scheme (paired)
			continue
		default:
			scheme = appName
			destination = PlatformBuildDestination(PlatformIOS)
		}
		cmds = append(cmds, fmt.Sprintf("xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet build", appName, scheme, destination))
	}
	return cmds
}

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
		overview.WriteString("- Stack: SwiftUI (no third-party packages)\n")
	} else {
		overview.WriteString("- Platform: ")
		overview.WriteString(platformSummary(platform, deviceFamily))
		overview.WriteString("\n")
		if IsWatchOS(platform) {
			overview.WriteString("- Stack: SwiftUI (no third-party packages, no UIKit)\n")
		} else if IsTvOS(platform) {
			overview.WriteString("- Stack: SwiftUI (no third-party packages, no UIKit)\n")
		} else {
			overview.WriteString("- Stack: SwiftUI + SwiftData (no third-party packages)\n")
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
	design.WriteString("- **NEVER** use hardcoded colors in views (`.white`, `.black`, `Color.red`, `.opacity()` on raw colors)\n")
	design.WriteString("- **NEVER** use hardcoded fonts in views (`.font(.title2)`, `.font(.system(size:))`)\n")
	design.WriteString("- **NEVER** use hardcoded spacing in views (`.padding(20)`, `VStack(spacing: 10)`)\n")
	design.WriteString("- ALL colors → `AppTheme.Colors.*` (including `textPrimary`, `textSecondary`, `textTertiary`)\n")
	design.WriteString("- ALL fonts → `AppTheme.Fonts.*` (with plan's font design applied)\n")
	design.WriteString("- ALL spacing → `AppTheme.Spacing.*`\n")
	design.WriteString("- Every view should use semantic theme tokens and include `#Preview`\n")
	design.WriteString("- Keep adaptive layout and accessibility in mind for iPad/universal apps\n")
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
	xg.WriteString("- Preferred tools: `add_permission`, `add_extension`, `add_entitlement`, `add_localization`, `set_build_setting`, `get_project_config`, `regenerate_project`\n")
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
		} else if IsTvOS(plan.GetPlatform()) {
			// tvOS has no device family or watch shape — just platform
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
	}
	files["generated-plan.md"] = planDoc.String()

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(memoryDir, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write memory file %s: %w", name, err)
		}
	}

	return nil
}

// conditionalCategories lists embedded directories searched for conditional skill keys.
var conditionalCategories = []string{"features", "ui", "extensions"}

// writeCoreRules copies skills/core/*.md to projectDir/.claude/rules/ (always loaded eagerly).
func writeCoreRules(projectDir string) error {
	rulesDir := filepath.Join(projectDir, ".claude", "rules")

	entries, err := fs.ReadDir(skillsFS, "skills/core")
	if err != nil {
		return fmt.Errorf("failed to read embedded core rules: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := skillsFS.ReadFile("skills/core/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read embedded rule %s: %w", entry.Name(), err)
		}

		if err := os.WriteFile(filepath.Join(rulesDir, entry.Name()), content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// platformOverrideDir returns the embedded skills override directory for the given platform,
// or empty string if no overrides exist. e.g. "skills/always-watchos" for watchOS.
func platformOverrideDir(platform string) string {
	switch {
	case IsWatchOS(platform):
		return "skills/always-watchos"
	case IsTvOS(platform):
		return "skills/always-tvos"
	default:
		return ""
	}
}

// writeAlwaysSkills copies all skills/always/* to .claude/skills/*/ (lazy, always present).
// Handles both flat .md files and multi-file directories (e.g., swiftui/).
// When platform has overrides (watchOS, tvOS), entries from the override directory replace
// same-named entries from skills/always/, and platform-only entries are also loaded.
// For multi-platform, loads the union of all platform overrides.
func writeAlwaysSkills(projectDir, platform string, extraPlatforms ...string) error {
	skillsDir := filepath.Join(projectDir, ".claude", "skills")

	// Collect all override dirs for the given platform(s)
	var overrideDirs []string
	if d := platformOverrideDir(platform); d != "" {
		overrideDirs = append(overrideDirs, d)
	}
	for _, p := range extraPlatforms {
		if d := platformOverrideDir(p); d != "" {
			overrideDirs = append(overrideDirs, d)
		}
	}

	// Build set of platform overrides (by skill name).
	overrides := map[string]bool{}
	for _, overrideDir := range overrideDirs {
		if entries, err := fs.ReadDir(skillsFS, overrideDir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					overrides[e.Name()] = true
				} else if strings.HasSuffix(e.Name(), ".md") {
					overrides[strings.TrimSuffix(e.Name(), ".md")] = true
				}
			}
		}
	}

	// Load from skills/always/, skipping entries that have a platform override.
	entries, err := fs.ReadDir(skillsFS, "skills/always")
	if err != nil {
		return fmt.Errorf("failed to read embedded always skills: %w", err)
	}

	for _, entry := range entries {
		skillName := entry.Name()
		if !entry.IsDir() && strings.HasSuffix(skillName, ".md") {
			skillName = strings.TrimSuffix(skillName, ".md")
		}
		if overrides[skillName] {
			continue // will be loaded from platform override dir instead
		}

		if entry.IsDir() {
			srcPath := "skills/always/" + entry.Name()
			dstPath := filepath.Join(skillsDir, entry.Name())
			if err := writeSkillDir(srcPath, dstPath); err != nil {
				return err
			}
		} else if strings.HasSuffix(entry.Name(), ".md") {
			dstDir := filepath.Join(skillsDir, skillName)
			if err := os.MkdirAll(dstDir, 0o755); err != nil {
				return fmt.Errorf("failed to create dir %s: %w", dstDir, err)
			}
			content, err := skillsFS.ReadFile("skills/always/" + entry.Name())
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", entry.Name(), err)
			}
			if err := os.WriteFile(filepath.Join(dstDir, "SKILL.md"), content, 0o644); err != nil {
				return err
			}
		}
	}

	// Load platform overrides + platform-only skills.
	for _, overrideDir := range overrideDirs {
		oEntries, err := fs.ReadDir(skillsFS, overrideDir)
		if err != nil {
			return fmt.Errorf("failed to read embedded %s skills: %w", overrideDir, err)
		}
		for _, entry := range oEntries {
			if entry.IsDir() {
				srcPath := overrideDir + "/" + entry.Name()
				dstPath := filepath.Join(skillsDir, entry.Name())
				if err := writeSkillDir(srcPath, dstPath); err != nil {
					return err
				}
			} else if strings.HasSuffix(entry.Name(), ".md") {
				skillName := strings.TrimSuffix(entry.Name(), ".md")
				dstDir := filepath.Join(skillsDir, skillName)
				if err := os.MkdirAll(dstDir, 0o755); err != nil {
					return fmt.Errorf("failed to create dir %s: %w", dstDir, err)
				}
				content, err := skillsFS.ReadFile(overrideDir + "/" + entry.Name())
				if err != nil {
					return fmt.Errorf("failed to read %s: %w", entry.Name(), err)
				}
				if err := os.WriteFile(filepath.Join(dstDir, "SKILL.md"), content, 0o644); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// writeConditionalSkills copies matching skills from features/, ui/, extensions/
// to .claude/skills/<key>/ for each key in ruleKeys.
// Handles both directories and flat .md files.
// When platform is watchOS, the search order is ["watchos", "features", "ui", "extensions"]
// so watchOS-specific skills take precedence (first match wins).
func writeConditionalSkills(projectDir string, ruleKeys []string, platform string) error {
	skillsDir := filepath.Join(projectDir, ".claude", "skills")

	categories := conditionalCategories
	if IsWatchOS(platform) {
		categories = append([]string{"watchos"}, conditionalCategories...)
	} else if IsTvOS(platform) {
		categories = append([]string{"tvos"}, conditionalCategories...)
	}

	for _, key := range ruleKeys {
		for _, cat := range categories {
			// Try as directory first
			srcPath := fmt.Sprintf("skills/%s/%s", cat, key)
			if _, err := fs.ReadDir(skillsFS, srcPath); err == nil {
				dstPath := filepath.Join(skillsDir, key)
				if err := writeSkillDir(srcPath, dstPath); err != nil {
					return err
				}
				break // found and written
			}

			// Try as flat file
			filePath := fmt.Sprintf("skills/%s/%s.md", cat, key)
			if data, err := skillsFS.ReadFile(filePath); err == nil {
				dstDir := filepath.Join(skillsDir, key)
				if err := os.MkdirAll(dstDir, 0o755); err != nil {
					return fmt.Errorf("failed to create dir %s: %w", dstDir, err)
				}
				if err := os.WriteFile(filepath.Join(dstDir, "SKILL.md"), data, 0o644); err != nil {
					return err
				}
				break // found and written
			}
		}
	}
	return nil
}

// writeSkillDir copies all files from an embedded directory to an output directory.
func writeSkillDir(embeddedPath, outputDir string) error {
	if err := fs.WalkDir(skillsFS, embeddedPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("failed walking %s: %w", path, walkErr)
		}

		rel := strings.TrimPrefix(path, embeddedPath)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				return fmt.Errorf("failed to create dir %s: %w", outputDir, err)
			}
			return nil
		}

		dstPath := filepath.Join(outputDir, filepath.FromSlash(rel))
		if d.IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				return fmt.Errorf("failed to create dir %s: %w", dstPath, err)
			}
			return nil
		}

		content, err := skillsFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return fmt.Errorf("failed to create parent dir for %s: %w", dstPath, err)
		}
		if err := os.WriteFile(dstPath, content, 0o644); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// extractFrontmatter splits YAML frontmatter from markdown content.
// Returns the description value from frontmatter and the body after the closing ---.
func extractFrontmatter(content string) (description string, body string) {
	if !strings.HasPrefix(content, "---") {
		return "", content
	}
	end := strings.Index(content[3:], "---")
	if end < 0 {
		return "", content
	}

	frontmatter := content[3 : end+3]
	body = strings.TrimSpace(content[end+6:])

	// Extract description from frontmatter
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "description:") {
			desc := strings.TrimPrefix(line, "description:")
			desc = strings.TrimSpace(desc)
			desc = strings.Trim(desc, "\"'")
			return desc, body
		}
	}
	return "", body
}

func readEmbeddedMarkdownBody(path string) (body string, found bool) {
	data, err := skillsFS.ReadFile(path)
	if err != nil {
		return "", false
	}
	_, body = extractFrontmatter(string(data))
	return body, true
}

func readEmbeddedMarkdownDirBodies(dirPath string) string {
	var combined strings.Builder

	if body, found := readEmbeddedMarkdownBody(dirPath + "/SKILL.md"); found && body != "" {
		combined.WriteString(body)
	}

	_ = fs.WalkDir(skillsFS, dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") || d.Name() == "SKILL.md" {
			return nil
		}
		body, found := readEmbeddedMarkdownBody(path)
		if !found || body == "" {
			return nil
		}
		if combined.Len() > 0 {
			combined.WriteString("\n\n")
		}
		combined.WriteString(body)
		return nil
	})
	return combined.String()
}

// loadRuleContent reads content for a given rule_key from the embedded FS.
// It searches core/, always/, features/, ui/, extensions/ for the key.
// Handles both flat .md files and directories with content files.
// Returns content stripped of YAML frontmatter, or empty string if not found.
func loadRuleContent(ruleKey string) string {
	// Try core/ first (single file)
	corePath := fmt.Sprintf("skills/core/%s.md", ruleKey)
	if body, found := readEmbeddedMarkdownBody(corePath); found {
		return body
	}

	// Search categorized: always/, features/, ui/, extensions/
	categories := []string{"always", "features", "ui", "extensions"}
	for _, cat := range categories {
		// Try as flat file first
		filePath := fmt.Sprintf("skills/%s/%s.md", cat, ruleKey)
		if body, found := readEmbeddedMarkdownBody(filePath); found && body != "" {
			return body
		}

		// Try as directory
		dirPath := fmt.Sprintf("skills/%s/%s", cat, ruleKey)
		if combined := readEmbeddedMarkdownDirBodies(dirPath); combined != "" {
			return combined
		}
	}
	return ""
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
	if err := writeSettingsShared(projectDir); err != nil {
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

func writeTextFile(path, content string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), mode)
}

func writeExecutableFile(path, content string) error {
	return writeTextFile(path, content, 0o755)
}

func writeSkillCatalog(projectDir string) error {
	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return fmt.Errorf("failed to read generated skills dir: %w", err)
	}

	type skillInfo struct {
		Name        string
		Description string
		Dir         string
		Companions  []string
	}
	var skills []skillInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()
		skillDir := filepath.Join(skillsDir, dirName)
		skillPath := filepath.Join(skillDir, "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}
		desc, _ := extractFrontmatter(string(data))

		var companions []string
		_ = filepath.WalkDir(skillDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".md") || d.Name() == "SKILL.md" {
				return nil
			}
			rel, err := filepath.Rel(skillDir, path)
			if err != nil {
				return nil
			}
			companions = append(companions, filepath.ToSlash(rel))
			return nil
		})
		sort.Strings(companions)
		skills = append(skills, skillInfo{
			Name:        dirName,
			Description: desc,
			Dir:         dirName,
			Companions:  companions,
		})
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })

	var b strings.Builder
	b.WriteString("# Skill Catalog\n\n")
	b.WriteString("Generated project-local skills for Claude Code. Skills are lazy-loaded from `.claude/skills/` when relevant.\n\n")
	b.WriteString("## Usage\n")
	b.WriteString("- Let Claude discover skills automatically via descriptions\n")
	b.WriteString("- You can also invoke related workflows through slash commands in `.claude/commands/`\n")
	b.WriteString("- Run `./scripts/claude/validate-skills.sh` after editing skill files\n")

	if len(skills) == 0 {
		b.WriteString("\n_No skills generated yet._\n")
	} else {
		b.WriteString("\n## Skills\n")
		for _, s := range skills {
			fmt.Fprintf(&b, "\n### `%s`\n", s.Name)
			if s.Description != "" {
				fmt.Fprintf(&b, "- Purpose: %s\n", s.Description)
			} else {
				b.WriteString("- Purpose: (no description found in frontmatter)\n")
			}
			fmt.Fprintf(&b, "- Path: `.claude/skills/%s/`\n", s.Dir)
			fmt.Fprintf(&b, "- Trigger hint: tasks related to `%s`\n", strings.ReplaceAll(s.Name, "_", " "))
			if len(s.Companions) > 0 {
				b.WriteString("- Companion docs: ")
				for i, c := range s.Companions {
					if i > 0 {
						b.WriteString(", ")
					}
					fmt.Fprintf(&b, "`%s`", c)
				}
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "- Example command: `/quality-review` before large refactors touching `%s`\n", s.Name)
		}
	}

	return writeTextFile(filepath.Join(skillsDir, "INDEX.md"), b.String(), 0o644)
}

// writeMCPConfig writes .mcp.json at the project root to give Claude Code access to Apple docs and xcodegen tools.
func writeMCPConfig(projectDir string) error {
	mcpConfig := `{
  "mcpServers": {
    "apple-docs": {
      "command": "npx",
      "args": ["-y", "@kimsungwhee/apple-docs-mcp"]
    },
    "xcodegen": {
      "command": "nanowave",
      "args": ["mcp", "xcodegen"]
    }
  }
}
`
	return os.WriteFile(filepath.Join(projectDir, ".mcp.json"), []byte(mcpConfig), 0o644)
}

// writeSettingsShared writes team-shared Claude Code settings.
func writeSettingsShared(projectDir string) error {
	settings := `{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "permissions": {
    "allow": [
      "mcp__apple-docs__search_apple_docs",
      "mcp__apple-docs__get_apple_doc_content",
      "mcp__apple-docs__search_framework_symbols",
      "mcp__apple-docs__get_sample_code",
      "mcp__apple-docs__get_related_apis",
      "mcp__apple-docs__find_similar_apis",
      "mcp__apple-docs__get_platform_compatibility",
      "mcp__xcodegen__add_permission",
      "mcp__xcodegen__add_extension",
      "mcp__xcodegen__add_entitlement",
      "mcp__xcodegen__add_localization",
      "mcp__xcodegen__set_build_setting",
      "mcp__xcodegen__get_project_config",
      "mcp__xcodegen__regenerate_project",
      "SlashCommand",
      "Task",
      "ViewImage",
      "WebFetch",
      "WebSearch"
    ],
    "deny": [
      "Read(./.env)",
      "Read(./.env.*)",
      "Read(./secrets/**)"
    ]
  },
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": "./scripts/claude/check-project-config-edits.sh"
          }
        ]
      },
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "./scripts/claude/check-bash-safety.sh"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": "./scripts/claude/check-swift-structure.sh"
          },
          {
            "type": "command",
            "command": "./scripts/claude/check-no-placeholders.sh --hook"
          },
          {
            "type": "command",
            "command": "./scripts/claude/check-previews.sh --hook"
          },
          {
            "type": "command",
            "command": "./scripts/claude/check-a11y-dynamic-type.sh --hook"
          },
          {
            "type": "command",
            "command": "./scripts/claude/check-a11y-icon-buttons.sh --hook"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "./scripts/claude/run-build-check.sh --hook"
          }
        ]
      }
    ]
  }
}
`
	return os.WriteFile(filepath.Join(projectDir, ".claude", "settings.json"), []byte(settings), 0o644)
}

// writeSettingsLocal writes local (non-committed) Claude Code settings for machine-specific overrides.
func writeSettingsLocal(projectDir string) error {
	settings := `{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "permissions": {
    "allow": []
  }
}
`
	return os.WriteFile(filepath.Join(projectDir, ".claude", "settings.local.json"), []byte(settings), 0o644)
}

// ensureMCPConfig writes .mcp.json and settings.local.json if they don't exist.
// Used by Edit and Fix flows on existing projects that may lack these files.
func ensureMCPConfig(projectDir string) {
	mcpPath := filepath.Join(projectDir, ".mcp.json")
	if _, err := os.Stat(mcpPath); os.IsNotExist(err) {
		_ = writeMCPConfig(projectDir)
	}
	sharedSettingsPath := filepath.Join(projectDir, ".claude", "settings.json")
	if _, err := os.Stat(sharedSettingsPath); os.IsNotExist(err) {
		_ = os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755)
		_ = writeSettingsShared(projectDir)
	}
	settingsPath := filepath.Join(projectDir, ".claude", "settings.local.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		_ = os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755)
		_ = writeSettingsLocal(projectDir)
	}
}

// writeGitignore writes a standard iOS .gitignore to the project directory.
func writeGitignore(projectDir string) error {
	content := `# Xcode
*.xcodeproj/project.xcworkspace/
*.xcodeproj/xcuserdata/
xcuserdata/
DerivedData/
build/
*.ipa
*.dSYM.zip
*.dSYM

# Swift Package Manager
.build/
Package.resolved

# OS
.DS_Store

# Claude Code
.claude/settings.local.json
.claude/logs/
.claude/tmp/
.claude/transcripts/
`
	return os.WriteFile(filepath.Join(projectDir, ".gitignore"), []byte(content), 0o644)
}

// writeAssetCatalog writes the minimal Assets.xcassets structure with AppIcon and AccentColor.
func writeAssetCatalog(projectDir, appName, platform string) error {
	assetsDir := filepath.Join(projectDir, appName, "Assets.xcassets")
	appIconDir := filepath.Join(assetsDir, "AppIcon.appiconset")
	accentColorDir := filepath.Join(assetsDir, "AccentColor.colorset")

	for _, d := range []string{appIconDir, accentColorDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create asset catalog: %w", err)
		}
	}

	assetsContents := `{"info":{"version":1,"author":"xcode"}}`
	if err := os.WriteFile(filepath.Join(assetsDir, "Contents.json"), []byte(assetsContents), 0o644); err != nil {
		return err
	}

	var appIconContents string
	if IsWatchOS(platform) {
		appIconContents = `{
  "images": [
    {
      "idiom": "universal",
      "platform": "watchos",
      "size": "1024x1024"
    }
  ],
  "info": {
    "version": 1,
    "author": "xcode"
  }
}`
	} else if IsTvOS(platform) {
		// tvOS uses layered image stacks for parallax effects on the home screen.
		// The App Store icon is 1280x768 and home screen icon is 400x240.
		appIconContents = `{
  "images": [
    {
      "idiom": "tv",
      "platform": "tvos",
      "size": "1280x768",
      "scale": "1x"
    },
    {
      "idiom": "tv",
      "platform": "tvos",
      "size": "400x240",
      "scale": "2x"
    }
  ],
  "info": {
    "version": 1,
    "author": "xcode"
  }
}`
	} else {
		appIconContents = `{
  "images": [
    {
      "idiom": "universal",
      "platform": "ios",
      "size": "1024x1024"
    }
  ],
  "info": {
    "version": 1,
    "author": "xcode"
  }
}`
	}
	if err := os.WriteFile(filepath.Join(appIconDir, "Contents.json"), []byte(appIconContents), 0o644); err != nil {
		return err
	}

	accentColorContents := `{
  "colors": [
    {
      "idiom": "universal"
    }
  ],
  "info": {
    "version": 1,
    "author": "xcode"
  }
}`
	return os.WriteFile(filepath.Join(accentColorDir, "Contents.json"), []byte(accentColorContents), 0o644)
}

// writeProjectConfig writes project_config.json from the PlannerResult.
// This is the source of truth that the xcodegen MCP server reads/writes.
func writeProjectConfig(projectDir string, plan *PlannerResult, appName string) error {
	bundleID := fmt.Sprintf("%s.%s", bundleIDPrefix(), strings.ToLower(appName))

	type permission struct {
		Key         string `json:"key"`
		Description string `json:"description"`
		Framework   string `json:"framework"`
	}
	type extensionPlan struct {
		Kind         string            `json:"kind"`
		Name         string            `json:"name"`
		Purpose      string            `json:"purpose"`
		InfoPlist    map[string]any    `json:"info_plist,omitempty"`
		Entitlements map[string]any    `json:"entitlements,omitempty"`
		Settings     map[string]string `json:"settings,omitempty"`
	}
	type projectConfig struct {
		AppName           string            `json:"app_name"`
		BundleID          string            `json:"bundle_id"`
		Platform          string            `json:"platform,omitempty"`
		Platforms         []string          `json:"platforms,omitempty"`
		WatchProjectShape string            `json:"watch_project_shape,omitempty"`
		DeviceFamily      string            `json:"device_family,omitempty"`
		Permissions       []permission      `json:"permissions,omitempty"`
		Extensions        []extensionPlan   `json:"extensions,omitempty"`
		Localizations     []string          `json:"localizations,omitempty"`
		BuildSettings     map[string]string `json:"build_settings,omitempty"`
	}

	cfg := projectConfig{
		AppName:           appName,
		BundleID:          bundleID,
		Platform:          plan.GetPlatform(),
		WatchProjectShape: plan.GetWatchProjectShape(),
		DeviceFamily:      plan.GetDeviceFamily(),
	}
	if plan.IsMultiPlatform() {
		cfg.Platforms = plan.GetPlatforms()
	}

	if plan != nil {
		for _, p := range plan.Permissions {
			cfg.Permissions = append(cfg.Permissions, permission{
				Key:         p.Key,
				Description: p.Description,
				Framework:   p.Framework,
			})
		}
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			cfg.Extensions = append(cfg.Extensions, extensionPlan{
				Kind:         ext.Kind,
				Name:         name,
				Purpose:      ext.Purpose,
				InfoPlist:    ext.InfoPlist,
				Entitlements: ext.Entitlements,
				Settings:     ext.Settings,
			})
		}
		cfg.Localizations = plan.Localizations
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project config: %w", err)
	}
	return os.WriteFile(filepath.Join(projectDir, "project_config.json"), data, 0o644)
}

// writeProjectYML writes the XcodeGen project.yml, including extension targets if present.
func writeProjectYML(projectDir string, plan *PlannerResult, appName string) error {
	yml := generateProjectYAML(appName, plan)
	return os.WriteFile(filepath.Join(projectDir, "project.yml"), []byte(yml), 0o644)
}

// scaffoldSourceDirs creates the directory structure that XcodeGen expects before generating
// the .xcodeproj. This ensures all source paths referenced in project.yml actually exist.
func scaffoldSourceDirs(projectDir, appName string, plan *PlannerResult) error {
	dirs := []string{
		filepath.Join(projectDir, appName),
	}

	if plan != nil && plan.IsMultiPlatform() {
		// Multi-platform: create source dirs for each platform
		for _, plat := range plan.GetPlatforms() {
			suffix := PlatformSourceDirSuffix(plat)
			if suffix != "" {
				dirs = append(dirs, filepath.Join(projectDir, appName+suffix))
			}
		}
		// Shared/ always created for multi-platform
		dirs = append(dirs, filepath.Join(projectDir, "Shared"))
	} else {
		// For paired watchOS apps, create the watch source directory
		if plan != nil && IsWatchOS(plan.GetPlatform()) && plan.GetWatchProjectShape() == WatchShapePaired {
			dirs = append(dirs, filepath.Join(projectDir, appName+"Watch"))
		}
		// Shared/ directory when extensions exist
		if plan != nil && len(plan.Extensions) > 0 {
			dirs = append(dirs, filepath.Join(projectDir, "Shared"))
		}
	}

	if plan != nil {
		// Targets/{ExtensionName}/ per extension
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			dirs = append(dirs, filepath.Join(projectDir, "Targets", name))
		}

		// .lproj directories for localizations
		for _, lang := range plan.Localizations {
			dirs = append(dirs, filepath.Join(projectDir, appName, lang+".lproj"))
		}
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	// Write a placeholder .swift file in each source directory so XcodeGen
	// doesn't complain about empty source groups.
	// Each placeholder uses a unique name to avoid "Multiple commands produce
	// *.stringsdata" collisions when multiple source dirs feed one target.
	type placeholderEntry struct {
		dir  string
		name string
	}
	placeholders := []placeholderEntry{
		{filepath.Join(projectDir, appName), "Placeholder.swift"},
	}

	if plan != nil && plan.IsMultiPlatform() {
		for _, plat := range plan.GetPlatforms() {
			suffix := PlatformSourceDirSuffix(plat)
			if suffix != "" {
				placeholders = append(placeholders, placeholderEntry{filepath.Join(projectDir, appName+suffix), "Placeholder.swift"})
			}
		}
		placeholders = append(placeholders, placeholderEntry{filepath.Join(projectDir, "Shared"), "SharedPlaceholder.swift"})
	} else if plan != nil && len(plan.Extensions) > 0 {
		placeholders = append(placeholders, placeholderEntry{filepath.Join(projectDir, "Shared"), "SharedPlaceholder.swift"})
	}

	if plan != nil {
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			placeholders = append(placeholders, placeholderEntry{filepath.Join(projectDir, "Targets", name), "Placeholder.swift"})
		}
	}

	placeholderContent := []byte("// Placeholder — replaced by generated code\nimport Foundation\n")
	for _, p := range placeholders {
		if err := os.WriteFile(filepath.Join(p.dir, p.name), placeholderContent, 0o644); err != nil {
			return fmt.Errorf("failed to write placeholder %s: %w", filepath.Join(p.dir, p.name), err)
		}
	}

	return nil
}

// cleanupScaffoldPlaceholders removes Placeholder.swift files from directories
// that now contain real generated Swift code. These scaffolding files are created
// by scaffoldSourceDirs to satisfy XcodeGen, but after the build phase writes
// real code they are no longer needed and trigger the quality-gate hook.
func cleanupScaffoldPlaceholders(projectDir, appName string, plan *PlannerResult) {
	candidates := []string{
		filepath.Join(projectDir, appName, "Placeholder.swift"),
	}
	if plan != nil && plan.IsMultiPlatform() {
		for _, plat := range plan.GetPlatforms() {
			suffix := PlatformSourceDirSuffix(plat)
			if suffix != "" {
				candidates = append(candidates, filepath.Join(projectDir, appName+suffix, "Placeholder.swift"))
			}
		}
		candidates = append(candidates, filepath.Join(projectDir, "Shared", "SharedPlaceholder.swift"))
	} else if plan != nil && len(plan.Extensions) > 0 {
		candidates = append(candidates, filepath.Join(projectDir, "Shared", "SharedPlaceholder.swift"))
	}
	if plan != nil {
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			candidates = append(candidates, filepath.Join(projectDir, "Targets", name, "Placeholder.swift"))
		}
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err != nil {
			continue // already gone
		}
		// Only remove if the parent directory contains at least one other .swift file
		dir := filepath.Dir(p)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		hasOtherSwift := false
		placeholderName := filepath.Base(p)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.HasSuffix(e.Name(), ".swift") && e.Name() != placeholderName {
				hasOtherSwift = true
				break
			}
		}
		// Also check subdirectories — the app source tree may have its Swift files in subdirs
		if !hasOtherSwift {
			_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				if strings.HasSuffix(d.Name(), ".swift") && d.Name() != placeholderName {
					hasOtherSwift = true
					return filepath.SkipAll
				}
				return nil
			})
		}
		if hasOtherSwift {
			os.Remove(p)
		}
	}
}

// runXcodeGen runs `xcodegen generate` in the project directory to create the .xcodeproj.
func runXcodeGen(projectDir string) error {
	cmd := exec.Command("xcodegen", "generate")
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("xcodegen generate failed: %w\n%s", err, string(output))
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
