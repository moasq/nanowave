package orchestration

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/moasq/nanowave/internal/skills"
)

// conditionalCategories lists embedded directories searched for conditional skill keys.
var conditionalCategories = []string{"features", "ui", "extensions"}

// writeCoreRules copies skills/core/*.md to projectDir/.claude/rules/ (always loaded eagerly).
// Platform-specific content in swift-conventions.md is adapted to the target platform.
// Planner-approved packages are injected into forbidden-patterns.md.
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

// loadRuleContent delegates to skills.LoadRuleContent.
func loadRuleContent(ruleKey string) string {
	return skills.LoadRuleContent(ruleKey)
}

// readEmbeddedMarkdownDirBodies delegates to skills.ReadMarkdownDirBodies.
func readEmbeddedMarkdownDirBodies(dirPath string) string {
	dirPath = strings.TrimPrefix(dirPath, "data/")
	return skills.ReadMarkdownDirBodies(dirPath)
}
