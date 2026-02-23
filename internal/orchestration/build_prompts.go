package orchestration

import (
	"fmt"
	"path/filepath"
	"strings"
)

// buildPrompts constructs the system and user prompts for the build phase.
func (p *Pipeline) buildPrompts(_ string, appName string, _ string, analysis *AnalysisResult, plan *PlannerResult) (string, string) {
	destination := canonicalBuildDestination(plan.GetPlatform())
	// Build the append system prompt with coder rules + plan context
	var appendPrompt strings.Builder
	appendPrompt.WriteString(coderPrompt)
	appendPrompt.WriteString("\n\n")
	appendPrompt.WriteString(sharedConstraints)

	// Add plan context
	appendPrompt.WriteString("\n\n## Build Plan\n\n")

	appendPrompt.WriteString("### Design\n")
	appendPrompt.WriteString(fmt.Sprintf("Navigation: %s\n", plan.Design.Navigation))
	appendPrompt.WriteString(fmt.Sprintf("Palette: primary=%s, secondary=%s, accent=%s, background=%s, surface=%s\n",
		plan.Design.Palette.Primary, plan.Design.Palette.Secondary, plan.Design.Palette.Accent,
		plan.Design.Palette.Background, plan.Design.Palette.Surface))
	appendPrompt.WriteString(fmt.Sprintf("Font: %s, Corner radius: %d, Density: %s, Surfaces: %s, Mood: %s\n",
		plan.Design.FontDesign, plan.Design.CornerRadius, plan.Design.Density, plan.Design.Surfaces, plan.Design.AppMood))

	appendPrompt.WriteString("\n### Models\n")
	for _, m := range plan.Models {
		appendPrompt.WriteString(fmt.Sprintf("- %s (%s):\n", m.Name, m.Storage))
		for _, prop := range m.Properties {
			if prop.DefaultValue != "" {
				appendPrompt.WriteString(fmt.Sprintf("  - %s: %s = %s\n", prop.Name, prop.Type, prop.DefaultValue))
			} else {
				appendPrompt.WriteString(fmt.Sprintf("  - %s: %s\n", prop.Name, prop.Type))
			}
		}
	}

	appendPrompt.WriteString("\n### Files (build in this order)\n")
	for _, path := range plan.BuildOrder {
		for _, f := range plan.Files {
			if f.Path == path {
				appendPrompt.WriteString(fmt.Sprintf("- %s (%s): %s\n  Components: %s\n  Data access: %s\n",
					f.Path, f.TypeName, f.Purpose, f.Components, f.DataAccess))
				break
			}
		}
	}
	// Include any files not in build_order
	for _, f := range plan.Files {
		found := false
		for _, path := range plan.BuildOrder {
			if f.Path == path {
				found = true
				break
			}
		}
		if !found {
			appendPrompt.WriteString(fmt.Sprintf("- %s (%s): %s\n  Components: %s\n  Data access: %s\n",
				f.Path, f.TypeName, f.Purpose, f.Components, f.DataAccess))
		}
	}

	if len(plan.Permissions) > 0 {
		appendPrompt.WriteString("\n### Permissions\n")
		for _, perm := range plan.Permissions {
			appendPrompt.WriteString(fmt.Sprintf("- %s: \"%s\" (framework: %s)\n", perm.Key, perm.Description, perm.Framework))
		}
	}

	if len(plan.Extensions) > 0 {
		appendPrompt.WriteString("\n### Extensions\n")
		for _, ext := range plan.Extensions {
			name := extensionTargetName(ext, appName)
			appendPrompt.WriteString(fmt.Sprintf("- %s (kind: %s): %s\n  Source path: Targets/%s/\n", name, ext.Kind, ext.Purpose, name))
		}
	}

	if len(plan.Localizations) > 0 {
		appendPrompt.WriteString(fmt.Sprintf("\n### Localizations: %s\n", strings.Join(plan.Localizations, ", ")))
	}

	// Inject rule content for each rule_key from embedded skill files
	if len(plan.RuleKeys) > 0 {
		appendPrompt.WriteString("\n## Feature Implementation Rules\n")
		for _, key := range plan.RuleKeys {
			content := loadRuleContent(key)
			if content != "" {
				appendPrompt.WriteString("\n")
				appendPrompt.WriteString(content)
				appendPrompt.WriteString("\n")
			}
		}
	}

	// Build user message
	var featureList strings.Builder
	for _, f := range analysis.Features {
		featureList.WriteString(fmt.Sprintf("- %s: %s\n", f.Name, f.Description))
	}

	userMsg := fmt.Sprintf(`Build the %s app following the plan in the system prompt.

App description: %s

Features:
%s
Core flow: %s

BEFORE WRITING CODE:
1. Use Glob to list all files in the project directory to understand the existing structure
2. Read the CLAUDE.md file to understand design tokens and architecture
3. Read project_config.json to understand the project configuration
Then proceed with writing code.

INSTRUCTIONS:
1. The Xcode project is already configured. Write ALL Swift files under %s/ following the plan file paths exactly.
2. Extension files go under Targets/{ExtensionName}/.
3. Shared types (e.g. ActivityAttributes) go under Shared/.
4. If you need additional permissions, extensions, or entitlements beyond the plan, use the xcodegen MCP tools.
5. After writing ALL files, run: xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet build
6. If the build fails, read the errors, fix the Swift code, and rebuild.
7. Repeat until the build succeeds.

IMPORTANT:
- Write files in the build order specified in the plan
- Use the exact type names and file paths from the plan
- Reference AppTheme for all design tokens — never hardcode colors
- Every View must have a #Preview block`,
		analysis.AppName, analysis.Description, featureList.String(), analysis.CoreFlow,
		appName, appName, appName, destination)

	return appendPrompt.String(), userMsg
}

// completionPrompts builds targeted prompts for unresolved planned files.
func (p *Pipeline) completionPrompts(appName string, projectDir string, plan *PlannerResult, report *FileCompletionReport) (string, string) {
	destination := canonicalBuildDestination(plan.GetPlatform())
	var appendPrompt strings.Builder
	appendPrompt.WriteString(coderPrompt)
	appendPrompt.WriteString("\n\n")
	appendPrompt.WriteString(sharedConstraints)
	appendPrompt.WriteString("\n\n## Completion Recovery Mode\n")
	appendPrompt.WriteString("Only complete the unresolved planned files listed in the user message.\n")
	appendPrompt.WriteString("Do not mark work done until every listed file exists, contains its expected type, and the build succeeds.\n")

	plannedByPath := make(map[string]FilePlan, len(plan.Files))
	for _, f := range plan.Files {
		plannedByPath[f.Path] = f
	}

	unresolved := unresolvedStatuses(report)

	var fileList strings.Builder
	for _, status := range unresolved {
		filePlan := plannedByPath[status.PlannedPath]
		relPath := status.ResolvedPath
		if rel, err := filepath.Rel(projectDir, status.ResolvedPath); err == nil {
			relPath = filepath.ToSlash(rel)
		}
		fileList.WriteString(fmt.Sprintf("- Planned path: %s\n", status.PlannedPath))
		fileList.WriteString(fmt.Sprintf("  Disk path: %s\n", relPath))
		if filePlan.TypeName != "" {
			fileList.WriteString(fmt.Sprintf("  Expected type: %s\n", filePlan.TypeName))
		}
		if filePlan.Purpose != "" {
			fileList.WriteString(fmt.Sprintf("  Purpose: %s\n", filePlan.Purpose))
		}
		if status.Reason != "" {
			fileList.WriteString(fmt.Sprintf("  Current issue: %s\n", status.Reason))
		}
	}

	userMsg := fmt.Sprintf(`Complete the unresolved files from the original build plan.

Unresolved files:
%s
Required process:
1. Create/fix ONLY the unresolved files listed above.
2. Ensure each file contains the expected type name exactly.
3. Keep existing already-valid files unchanged unless required for imports/signatures.
4. Run: xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet build
5. If build fails, fix issues and rebuild.
6. Stop only when every unresolved file is complete and the build succeeds.`, fileList.String(), appName, appName, destination)

	return appendPrompt.String(), userMsg
}
