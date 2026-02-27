package orchestration

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// conditionalCategories lists embedded directories searched for conditional skill keys.
var conditionalCategories = []string{"features", "ui", "extensions"}

// writeCoreRules copies skills/core/*.md to projectDir/.claude/rules/ (always loaded eagerly).
// Platform-specific content in swift-conventions.md is adapted to the target platform.
// Planner-approved packages are injected into forbidden-patterns.md.
func writeCoreRules(projectDir, platform string, packages []PackagePlan) error {
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

		// Adapt swift-conventions.md for the target platform
		if entry.Name() == "swift-conventions.md" {
			text := string(content)
			displayName := PlatformDisplayName(platform)
			text = strings.Replace(text, "**iOS 26+** deployment target", "**"+displayName+" 26+** deployment target", 1)
			archDesc := platformArchDescription(platform)
			if archDesc != "" {
				text = strings.Replace(text, "**SwiftUI-first** architecture. UIKit is allowed only when no viable SwiftUI equivalent exists for a required feature.", archDesc, 1)
			}
			content = []byte(text)
		}

		// Inject planner-approved packages into forbidden-patterns.md
		if entry.Name() == "forbidden-patterns.md" {
			text := string(content)
			replacement := ""
			if len(packages) > 0 {
				var sb strings.Builder
				sb.WriteString("\n### Approved Packages for This Project\n\n")
				sb.WriteString("The planner approved the following packages. Integrate each one:\n\n")
				for _, pkg := range packages {
					// Enrich with registry details when available
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

		if err := os.WriteFile(filepath.Join(rulesDir, entry.Name()), content, 0o644); err != nil {
			return err
		}
	}
	return nil
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

// platformOverrideDir returns the embedded skills override directory for the given platform,
// or empty string if no overrides exist. e.g. "skills/always-watchos" for watchOS.
func platformOverrideDir(platform string) string {
	switch {
	case IsWatchOS(platform):
		return "skills/always-watchos"
	case IsTvOS(platform):
		return "skills/always-tvos"
	case IsVisionOS(platform):
		return "skills/always-visionos"
	case IsMacOS(platform):
		return "skills/always-macos"
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
	} else if IsVisionOS(platform) {
		categories = append([]string{"visionos"}, conditionalCategories...)
	} else if IsMacOS(platform) {
		categories = append([]string{"macos"}, conditionalCategories...)
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
