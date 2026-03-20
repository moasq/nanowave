package orchestration

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/moasq/nanowave/internal/agentruntime"
	"github.com/moasq/nanowave/internal/instructions"
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
		// Unknown runtime — write Claude format as default
		return writeSkillsForClaude(projectDir, platform, packages)
	}
}

// writeSkillsForClaude writes rules to .claude/rules/ which Claude Code auto-loads.
// Writes all top-level rules/*.md plus platform-conditional rules/{platform}/*.md.
func writeSkillsForClaude(projectDir, platform string, packages []PackagePlan) error {
	rulesDir := filepath.Join(projectDir, ".claude", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		return fmt.Errorf("failed to create rules dir: %w", err)
	}

	// Write all top-level rules/*.md
	entries, err := fs.ReadDir(instructionsFS, "rules")
	if err != nil {
		return fmt.Errorf("failed to read embedded rules: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := instructionsFS.ReadFile("rules/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read embedded rule %s: %w", entry.Name(), err)
		}

		content = adaptCoreRule(entry.Name(), content, platform, packages)

		if err := os.WriteFile(filepath.Join(rulesDir, entry.Name()), content, 0o644); err != nil {
			return err
		}
	}

	// Write platform-conditional rules
	platDir := platformRuleDir(platform)
	if platDir != "" {
		platEntries, err := fs.ReadDir(instructionsFS, "rules/"+platDir)
		if err == nil {
			for _, entry := range platEntries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
					continue
				}
				content, err := instructionsFS.ReadFile("rules/" + platDir + "/" + entry.Name())
				if err != nil {
					continue
				}
				// Write as {platform}-{name}.md to avoid collisions with top-level rules
				outName := platDir + "-" + entry.Name()
				if err := os.WriteFile(filepath.Join(rulesDir, outName), content, 0o644); err != nil {
					return err
				}
			}
		}
	}

	return nil
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

	// Core rules (the 4 original core files)
	for _, key := range []string{"swift-conventions", "mvvm-architecture", "file-structure", "forbidden-patterns"} {
		content := loadCoreRuleAdapted(key, platform, packages)
		if content != "" {
			b.WriteString(content)
			b.WriteString("\n\n")
		}
	}

	// Always-on rules (flattened from always/)
	for _, key := range []string{"components", "design-system", "layout", "navigation"} {
		if body, found := instructions.LoadRule(key); found && body != "" {
			b.WriteString(body)
			b.WriteString("\n\n")
		}
	}

	// Platform-conditional rules
	platDir := platformRuleDir(platform)
	if platDir != "" {
		platRules := instructions.LoadPlatformRules(platDir)
		if len(platRules) > 0 {
			b.WriteString("---\n\n# Platform Rules\n\n")
			for _, body := range platRules {
				b.WriteString(body)
				b.WriteString("\n\n")
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
	data, err := instructionsFS.ReadFile("rules/" + key + ".md")
	if err != nil {
		return ""
	}
	adapted := adaptCoreRule(key+".md", data, platform, packages)
	_, body := instructions.ExtractFrontmatter(string(adapted))
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

// platformRuleDir maps platform constants to the rules/ subdirectory name.
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

// loadRuleContent loads skill content by key, delegating to instructions.LoadSkillContent.
func loadRuleContent(ruleKey string) string {
	return instructions.LoadSkillContent(ruleKey)
}

// readEmbeddedMarkdownDirBodies reads all markdown bodies from a skill directory.
func readEmbeddedMarkdownDirBodies(dirPath string) string {
	dirPath = strings.TrimPrefix(dirPath, "skills/")
	return instructions.ReadMarkdownDirBodies("skills/" + dirPath)
}

// listAvailableSkillKeys returns all discoverable skill keys from the embedded FS.
func listAvailableSkillKeys() []string {
	return instructions.ListSkillKeys()
}
