package orchestration

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moasq/nanowave/internal/agentruntime"
	"github.com/moasq/nanowave/internal/skills"
)

// writeSkillsForRuntime writes skill files to disk in the native format
// for the given agent runtime. Each runtime auto-loads from its own convention:
//   - Claude Code: .claude/rules/*.md (auto-loaded into context)
//   - Codex: codex.md at project root (auto-loaded as instructions)
//   - OpenCode: AGENTS.md at project root (auto-loaded as instructions)
func writeSkillsForRuntime(projectDir, platform string, ruleKeys []string, packages []PackagePlan, runtimeKind agentruntime.Kind) error {
	switch runtimeKind {
	case agentruntime.KindClaude:
		return writeSkillsForClaude(projectDir, platform, packages)
	case agentruntime.KindCodex:
		return writeSkillsForCodex(projectDir, platform, ruleKeys, packages)
	case agentruntime.KindOpenCode:
		return writeSkillsForOpenCode(projectDir, platform, ruleKeys, packages)
	default:
		return writeSkillsForClaude(projectDir, platform, packages)
	}
}

// writeCoreRules copies skills/data/core/*.md to projectDir/.claude/rules/ (always loaded eagerly).
func writeCoreRules(projectDir, platform string, packages []PackagePlan) error {
	rulesDir := filepath.Join(projectDir, ".claude", "rules")

	entries, err := fs.ReadDir(skillsFS, "data/core")
	if err != nil {
		return fmt.Errorf("failed to read embedded core rules: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := skillsFS.ReadFile("data/core/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read embedded rule %s: %w", entry.Name(), err)
		}

		content = adaptCoreRule(entry.Name(), content, platform, packages)

		if err := os.WriteFile(filepath.Join(rulesDir, entry.Name()), content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// writeAlwaysRules copies skills/data/always/**/SKILL.md to projectDir/.claude/rules/.
func writeAlwaysRules(projectDir string) error {
	rulesDir := filepath.Join(projectDir, ".claude", "rules")

	entries, err := fs.ReadDir(skillsFS, "data/always")
	if err != nil {
		return nil // always/ may not exist
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillPath := "data/always/" + entry.Name() + "/SKILL.md"
		content, err := skillsFS.ReadFile(skillPath)
		if err != nil {
			continue
		}
		outName := entry.Name() + ".md"
		if err := os.WriteFile(filepath.Join(rulesDir, outName), content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// writePlatformRules copies skills/data/always-{platform}/ content to projectDir/.claude/rules/.
// Reads both bare .md files and SKILL.md from subdirectories.
func writePlatformRules(projectDir, platform string) error {
	rulesDir := filepath.Join(projectDir, ".claude", "rules")
	platDir := platformAlwaysDir(platform)
	if platDir == "" {
		return nil
	}

	entries, err := fs.ReadDir(skillsFS, "data/"+platDir)
	if err != nil {
		return nil
	}

	// Track which names we've written to avoid duplicates (bare .md vs subdir/SKILL.md)
	written := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() {
			// Try loading SKILL.md from subdirectory
			skillPath := "data/" + platDir + "/" + entry.Name() + "/SKILL.md"
			content, err := skillsFS.ReadFile(skillPath)
			if err != nil {
				continue
			}
			outName := platformRuleDir(platform) + "-" + entry.Name() + ".md"
			if written[outName] {
				continue
			}
			if err := os.WriteFile(filepath.Join(rulesDir, outName), content, 0o644); err != nil {
				return err
			}
			written[outName] = true
		} else if strings.HasSuffix(entry.Name(), ".md") {
			content, err := skillsFS.ReadFile("data/" + platDir + "/" + entry.Name())
			if err != nil {
				continue
			}
			outName := platformRuleDir(platform) + "-" + entry.Name()
			if written[outName] {
				continue
			}
			if err := os.WriteFile(filepath.Join(rulesDir, outName), content, 0o644); err != nil {
				return err
			}
			written[outName] = true
		}
	}
	return nil
}

// writeSkillsForClaude writes core + always + platform rules to .claude/rules/.
func writeSkillsForClaude(projectDir, platform string, packages []PackagePlan) error {
	rulesDir := filepath.Join(projectDir, ".claude", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		return fmt.Errorf("failed to create rules dir: %w", err)
	}

	if err := writeCoreRules(projectDir, platform, packages); err != nil {
		return err
	}
	if err := writeAlwaysRules(projectDir); err != nil {
		return err
	}
	return writePlatformRules(projectDir, platform)
}

// writeSkillsForCodex writes a single codex.md instructions file at the project root.
func writeSkillsForCodex(projectDir, platform string, ruleKeys []string, packages []PackagePlan) error {
	content := composeUnifiedSkillsDoc(platform, ruleKeys, packages)
	return os.WriteFile(filepath.Join(projectDir, "codex.md"), []byte(content), 0o644)
}

// writeSkillsForOpenCode writes AGENTS.md at the project root.
func writeSkillsForOpenCode(projectDir, platform string, ruleKeys []string, packages []PackagePlan) error {
	content := composeUnifiedSkillsDoc(platform, ruleKeys, packages)
	return os.WriteFile(filepath.Join(projectDir, "AGENTS.md"), []byte(content), 0o644)
}

// composeUnifiedSkillsDoc builds a single markdown document with all relevant rules and skills
// for runtimes that use a single instructions file (Codex, OpenCode, Gemini, etc.).
func composeUnifiedSkillsDoc(platform string, ruleKeys []string, packages []PackagePlan) string {
	var b strings.Builder

	b.WriteString("# Nanowave Project Rules\n\n")

	// Architecture constraints (AppTheme, Observable pattern, layout, animation safety)
	b.WriteString("---\n\n")
	b.WriteString(sharedConstraints)
	b.WriteString("\n\n")

	// Platform verification checklist
	b.WriteString("---\n\n")
	b.WriteString(composeSelfCheck(platform))
	b.WriteString("\n\n")

	// Core rules
	for _, key := range []string{"scope", "swift-conventions", "mvvm-architecture", "file-structure", "forbidden-patterns"} {
		content := loadCoreRuleAdapted(key, platform, packages)
		if content != "" {
			b.WriteString(content)
			b.WriteString("\n\n")
		}
	}

	// Always-on rules
	for _, key := range []string{"components", "design-system", "layout", "navigation", "swiftui", "review"} {
		content := skills.LoadRuleContent(key)
		if content != "" {
			b.WriteString(content)
			b.WriteString("\n\n")
		}
	}

	// Platform-conditional always rules
	platDir := platformAlwaysDir(platform)
	if platDir != "" {
		entries, err := fs.ReadDir(skillsFS, "data/"+platDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
					continue
				}
				body, found := skills.ReadMarkdownBody(platDir + "/" + entry.Name())
				if found && body != "" {
					b.WriteString("---\n\n")
					b.WriteString(body)
					b.WriteString("\n\n")
				}
			}
		}
	}

	// Feature-specific skills for plan's rule keys
	if len(ruleKeys) > 0 {
		b.WriteString("---\n\n# Feature Rules\n\n")
		for _, key := range ruleKeys {
			content := loadRuleContent(key)
			if content != "" {
				b.WriteString(content)
				b.WriteString("\n\n")
			}
		}
	}

	return b.String()
}

// loadCoreRuleAdapted loads a core rule and applies platform/package adaptations.
func loadCoreRuleAdapted(key, platform string, packages []PackagePlan) string {
	data, err := skillsFS.ReadFile("data/core/" + key + ".md")
	if err != nil {
		return ""
	}
	adapted := adaptCoreRule(key+".md", data, platform, packages)
	_, body := skills.ExtractFrontmatter(string(adapted))
	return body
}

// adaptCoreRule applies platform and package adaptations to core rule content.
func adaptCoreRule(filename string, content []byte, platform string, packages []PackagePlan) []byte {
	if filename == "swift-conventions.md" {
		text := string(content)
		displayName := PlatformDisplayName(platform)
		text = strings.Replace(text, "**iOS 26+** deployment target", "**"+displayName+" 26+** deployment target", 1)
		archDesc := platformArchDescription(platform)
		if archDesc != "" {
			text = strings.Replace(text, "**SwiftUI-first** architecture. UIKit is allowed only when no viable SwiftUI equivalent exists for a required feature.", archDesc, 1)
		}
		content = []byte(text)
	}

	if filename == "forbidden-patterns.md" {
		text := string(content)
		replacement := ""
		if len(packages) > 0 {
			var sb strings.Builder
			sb.WriteString("\n### Approved Packages for This Project\n\n")
			sb.WriteString("The planner approved the following packages. Integrate each one:\n\n")
			for _, pkg := range packages {
				if curated := LookupPackageByName(pkg.Name); curated != nil {
					sb.WriteString(fmt.Sprintf("- **%s** — %s\n", curated.Name, pkg.Reason))
					sb.WriteString(fmt.Sprintf("  - URL: %s\n", curated.RepoURL))
					sb.WriteString(fmt.Sprintf("  - XcodeGen key: `%s`\n", curated.RepoName))
					sb.WriteString(fmt.Sprintf("  - Version: `from: \"%s\"`\n", curated.MinVersion))
					sb.WriteString(fmt.Sprintf("  - Import: `%s`\n", strings.Join(curated.Products, "`, `")))
				} else {
					sb.WriteString(fmt.Sprintf("- **%s** — %s\n", pkg.Name, pkg.Reason))
				}
			}
			replacement = sb.String()
		}
		text = strings.Replace(text, "<!-- APPROVED_PACKAGES_PLACEHOLDER -->", replacement, 1)
		content = []byte(text)
	}

	return content
}

// platformArchDescription returns the architecture description for a platform.
func platformArchDescription(platform string) string {
	switch {
	case IsMacOS(platform):
		return "**SwiftUI-first** architecture. SwiftUI native, AppKit bridge when needed, no UIKit."
	case IsWatchOS(platform):
		return "**SwiftUI-first** architecture. SwiftUI native for watchOS, no UIKit."
	case IsTvOS(platform):
		return "**SwiftUI-first** architecture. SwiftUI native for tvOS, UIKit only when no viable SwiftUI equivalent exists."
	case IsVisionOS(platform):
		return "**SwiftUI-first** architecture. SwiftUI native with RealityKit for spatial features, no UIKit."
	default:
		return ""
	}
}

// platformRuleDir maps platform constants to subdirectory names.
func platformRuleDir(platform string) string {
	switch platform {
	case PlatformWatchOS:
		return "watchos"
	case PlatformTvOS:
		return "tvos"
	case PlatformVisionOS:
		return "visionos"
	case PlatformMacOS:
		return "macos"
	default:
		return ""
	}
}

// platformAlwaysDir returns the always-{platform} directory name.
func platformAlwaysDir(platform string) string {
	dir := platformRuleDir(platform)
	if dir == "" {
		return ""
	}
	return "always-" + dir
}

// loadRuleContent loads skill content by key, delegating to skills.LoadRuleContent.
func loadRuleContent(ruleKey string) string {
	return skills.LoadRuleContent(ruleKey)
}

// readEmbeddedMarkdownDirBodies reads all markdown bodies from a skill directory.
func readEmbeddedMarkdownDirBodies(dirPath string) string {
	return skills.ReadMarkdownDirBodies(dirPath)
}

// listAvailableSkillKeys returns all discoverable skill keys from the embedded FS.
// Platform-specific skills are returned with their platform prefix (e.g. "watchos-biometrics").
func listAvailableSkillKeys() []string {
	seen := make(map[string]bool)

	// Non-platform categories — keys returned as-is
	for _, cat := range []string{"core", "always", "features", "ui", "extensions"} {
		entries, err := fs.ReadDir(skillsFS, "data/"+cat)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				seen[name] = true
			} else if strings.HasSuffix(name, ".md") {
				seen[strings.TrimSuffix(name, ".md")] = true
			}
		}
	}

	// Platform categories — keys prefixed with platform name
	for _, plat := range []string{"watchos", "tvos", "visionos", "macos"} {
		entries, err := fs.ReadDir(skillsFS, "data/"+plat)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				seen[plat+"-"+name] = true
			} else if strings.HasSuffix(name, ".md") {
				seen[plat+"-"+strings.TrimSuffix(name, ".md")] = true
			}
		}
	}

	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
