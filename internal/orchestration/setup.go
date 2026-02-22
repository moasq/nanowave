package orchestration

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}
	return nil
}

// writeInitialCLAUDEMD writes the CLAUDE.md with project-specific info only (before plan exists).
// Architecture, concurrency, UI, and design rules come from .claude/rules/ and .claude/skills/.
func writeInitialCLAUDEMD(projectDir, appName, deviceFamily string) error {
	var platformLine string
	switch deviceFamily {
	case "ipad":
		platformLine = "iPad only, iOS 26+, Swift 6"
	case "universal":
		platformLine = "iPhone and iPad, iOS 26+, Swift 6"
	default:
		platformLine = "iPhone only, iOS 26+, Swift 6"
	}
	content := fmt.Sprintf(`# %s — iOS Project Rules

## Platform
- %s
- SwiftUI + SwiftData only. No UIKit (except when required), no third-party packages.

## Architecture
- @main App -> RootView -> MainView -> content
- AppTheme for all design tokens (colors, spacing, typography)
- Models in Models/, Views in Features/{Name}/, Theme in Theme/

## @AppStorage Wiring (CRITICAL)
- Any @AppStorage value written in child views MUST be read in RootView
- A toggle without visible app-wide effect is a bug

## Build Command
`+"```"+`
xcodebuild -project %s.xcodeproj -scheme %s \
  -destination 'generic/platform=iOS Simulator' -quiet build
`+"```"+`

## Project Configuration
- Use the xcodegen MCP tools to manage project configuration
- add_permission, add_extension, add_entitlement, add_localization, set_build_setting, get_project_config, regenerate_project
- NEVER manually edit project.yml or .xcodeproj files
`, appName, platformLine, appName, appName)

	return os.WriteFile(filepath.Join(projectDir, ".claude", "CLAUDE.md"), []byte(content), 0o644)
}

// enrichCLAUDEMD updates CLAUDE.md with design tokens and plan details after Phase 3.
func enrichCLAUDEMD(projectDir string, plan *PlannerResult, appName string) error {
	claudeMDPath := filepath.Join(projectDir, ".claude", "CLAUDE.md")

	existing, err := os.ReadFile(claudeMDPath)
	if err != nil {
		return fmt.Errorf("failed to read CLAUDE.md: %w", err)
	}

	var enrichment strings.Builder
	enrichment.WriteString("\n\n## Design System\n")
	enrichment.WriteString(fmt.Sprintf("- Primary: %s, Secondary: %s, Accent: %s\n",
		plan.Design.Palette.Primary, plan.Design.Palette.Secondary, plan.Design.Palette.Accent))
	enrichment.WriteString(fmt.Sprintf("- Background: %s, Surface: %s\n",
		plan.Design.Palette.Background, plan.Design.Palette.Surface))
	enrichment.WriteString(fmt.Sprintf("- Font: %s, Corner radius: %d, Mood: %s\n",
		plan.Design.FontDesign, plan.Design.CornerRadius, plan.Design.AppMood))
	enrichment.WriteString(fmt.Sprintf("- Density: %s, Surfaces: %s\n",
		plan.Design.Density, plan.Design.Surfaces))

	if len(plan.Models) > 0 {
		enrichment.WriteString("\n## Models\n")
		for _, m := range plan.Models {
			var props []string
			for _, p := range m.Properties {
				props = append(props, fmt.Sprintf("%s: %s", p.Name, p.Type))
			}
			enrichment.WriteString(fmt.Sprintf("- %s (%s): %s\n", m.Name, m.Storage, strings.Join(props, ", ")))
		}
	}

	if len(plan.Files) > 0 {
		enrichment.WriteString("\n## File Architecture\n")
		for _, f := range plan.Files {
			enrichment.WriteString(fmt.Sprintf("- %s: %s\n", f.Path, f.Purpose))
		}
	}

	if len(plan.Permissions) > 0 {
		enrichment.WriteString("\n## Permissions\n")
		for _, p := range plan.Permissions {
			enrichment.WriteString(fmt.Sprintf("- %s: %s (%s)\n", p.Key, p.Description, p.Framework))
		}
	}

	if len(plan.Localizations) > 0 {
		enrichment.WriteString("\n## Localizations\n")
		enrichment.WriteString(fmt.Sprintf("- Languages: %s\n", strings.Join(plan.Localizations, ", ")))
	}

	if len(plan.Extensions) > 0 {
		enrichment.WriteString("\n## Extensions\n")
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			enrichment.WriteString(fmt.Sprintf("- %s (%s): %s → Targets/%s/\n", name, ext.Kind, ext.Purpose, name))
		}
		enrichment.WriteString("\nExtension source files go in Targets/{ExtensionName}/. Shared types (e.g. ActivityAttributes) go in the Shared/ directory — both targets compile it.\n")
	}

	enrichment.WriteString(`
## Extending Rules & Skills

This project uses a scalable skill system. Rules live in ` + "`.claude/`" + `:
- ` + "`.claude/rules/`" + ` — always loaded (core architecture rules)
- ` + "`.claude/skills/<name>/`" + ` — lazy loaded (SKILL.md with description + implementation rules)

### Adding a new feature domain (e.g., payments, auth, Firebase)
1. Create ` + "`.claude/skills/<name>/SKILL.md`" + ` with frontmatter and implementation rules
2. Claude will load it automatically when working on that feature

### Skill format
Each SKILL.md uses frontmatter:
  name: <skill-name>
  description: "<when to use this skill>"
  user-invocable: false
The body contains the implementation rules and code examples.
`)

	enriched := string(existing) + enrichment.String()
	return os.WriteFile(claudeMDPath, []byte(enriched), 0o644)
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

// writeAlwaysSkills copies all skills/always/* to .claude/skills/*/ (lazy, always present).
// Handles both flat .md files and multi-file directories (e.g., swiftui/).
func writeAlwaysSkills(projectDir string) error {
	skillsDir := filepath.Join(projectDir, ".claude", "skills")

	entries, err := fs.ReadDir(skillsFS, "skills/always")
	if err != nil {
		return fmt.Errorf("failed to read embedded always skills: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Multi-file skill (e.g., swiftui/) — copy directory
			srcPath := "skills/always/" + entry.Name()
			dstPath := filepath.Join(skillsDir, entry.Name())
			if err := writeSkillDir(srcPath, dstPath); err != nil {
				return err
			}
		} else if strings.HasSuffix(entry.Name(), ".md") {
			// Single-file skill — wrap in a directory with the file as SKILL.md
			skillName := strings.TrimSuffix(entry.Name(), ".md")
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
	return nil
}

// writeConditionalSkills copies matching skills from features/, ui/, extensions/
// to .claude/skills/<key>/ for each key in ruleKeys.
// Handles both directories and flat .md files.
func writeConditionalSkills(projectDir string, ruleKeys []string) error {
	skillsDir := filepath.Join(projectDir, ".claude", "skills")

	for _, key := range ruleKeys {
		for _, cat := range conditionalCategories {
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
	entries, err := fs.ReadDir(skillsFS, embeddedPath)
	if err != nil {
		return fmt.Errorf("failed to read embedded dir %s: %w", embeddedPath, err)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create dir %s: %w", outputDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		content, err := skillsFS.ReadFile(embeddedPath + "/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read %s/%s: %w", embeddedPath, entry.Name(), err)
		}

		if err := os.WriteFile(filepath.Join(outputDir, entry.Name()), content, 0o644); err != nil {
			return err
		}
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

// loadRuleContent reads content for a given rule_key from the embedded FS.
// It searches core/, always/, features/, ui/, extensions/ for the key.
// Handles both flat .md files and directories with content files.
// Returns content stripped of YAML frontmatter, or empty string if not found.
func loadRuleContent(ruleKey string) string {
	// Try core/ first (single file)
	corePath := fmt.Sprintf("skills/core/%s.md", ruleKey)
	if data, err := skillsFS.ReadFile(corePath); err == nil {
		_, body := extractFrontmatter(string(data))
		return body
	}

	// Search categorized: always/, features/, ui/, extensions/
	categories := []string{"always", "features", "ui", "extensions"}
	for _, cat := range categories {
		// Try as flat file first
		filePath := fmt.Sprintf("skills/%s/%s.md", cat, ruleKey)
		if data, err := skillsFS.ReadFile(filePath); err == nil {
			_, body := extractFrontmatter(string(data))
			if body != "" {
				return body
			}
		}

		// Try as directory
		dirPath := fmt.Sprintf("skills/%s/%s", cat, ruleKey)
		entries, err := fs.ReadDir(skillsFS, dirPath)
		if err != nil {
			continue
		}

		// Load all .md files except SKILL.md
		var combined strings.Builder
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || entry.Name() == "SKILL.md" {
				continue
			}
			data, err := skillsFS.ReadFile(dirPath + "/" + entry.Name())
			if err != nil {
				continue
			}
			_, body := extractFrontmatter(string(data))
			if body != "" {
				if combined.Len() > 0 {
					combined.WriteString("\n\n")
				}
				combined.WriteString(body)
			}
		}
		if combined.Len() > 0 {
			return combined.String()
		}
	}
	return ""
}

// writeMCPConfig writes .mcp.json at the project root to give Claude Code access to Apple docs and xcodegen tools.
func writeMCPConfig(projectDir string) error {
	nanowaveBin, err := os.Executable()
	if err != nil {
		// Fallback: try to find nanowave in PATH
		nanowaveBin = "nanowave"
	}
	mcpConfig := fmt.Sprintf(`{
  "mcpServers": {
    "apple-docs": {
      "command": "npx",
      "args": ["-y", "@kimsungwhee/apple-docs-mcp"]
    },
    "xcodegen": {
      "command": %q,
      "args": ["mcp", "xcodegen"]
    }
  }
}
`, nanowaveBin)
	return os.WriteFile(filepath.Join(projectDir, ".mcp.json"), []byte(mcpConfig), 0o644)
}

// writeSettingsLocal writes .claude/settings.local.json to auto-allow MCP tools.
func writeSettingsLocal(projectDir string) error {
	settings := `{
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
      "WebFetch",
      "WebSearch"
    ]
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
.claude/
`
	return os.WriteFile(filepath.Join(projectDir, ".gitignore"), []byte(content), 0o644)
}

// writeAssetCatalog writes the minimal Assets.xcassets structure with AppIcon and AccentColor.
func writeAssetCatalog(projectDir, appName string) error {
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

	appIconContents := `{
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
		AppName       string            `json:"app_name"`
		BundleID      string            `json:"bundle_id"`
		DeviceFamily  string            `json:"device_family,omitempty"`
		Permissions   []permission      `json:"permissions,omitempty"`
		Extensions    []extensionPlan   `json:"extensions,omitempty"`
		Localizations []string          `json:"localizations,omitempty"`
		BuildSettings map[string]string `json:"build_settings,omitempty"`
	}

	cfg := projectConfig{
		AppName:      appName,
		BundleID:     bundleID,
		DeviceFamily: plan.GetDeviceFamily(),
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

	if plan != nil {
		// Shared/ directory when extensions exist
		if len(plan.Extensions) > 0 {
			dirs = append(dirs, filepath.Join(projectDir, "Shared"))
		}

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
	placeholders := []string{
		filepath.Join(projectDir, appName, "Placeholder.swift"),
	}
	if plan != nil && len(plan.Extensions) > 0 {
		placeholders = append(placeholders, filepath.Join(projectDir, "Shared", "Placeholder.swift"))
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			placeholders = append(placeholders, filepath.Join(projectDir, "Targets", name, "Placeholder.swift"))
		}
	}

	placeholderContent := []byte("// Placeholder — replaced by generated code\nimport Foundation\n")
	for _, p := range placeholders {
		if err := os.WriteFile(p, placeholderContent, 0o644); err != nil {
			return fmt.Errorf("failed to write placeholder %s: %w", p, err)
		}
	}

	return nil
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
