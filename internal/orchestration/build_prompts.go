package orchestration

import (
	"fmt"
	"path/filepath"
	"strings"
)

func appendBuildPlanFileEntry(b *strings.Builder, f FilePlan) {
	platformTag := ""
	if f.Platform != "" {
		platformTag = fmt.Sprintf(" [%s]", f.Platform)
	}
	fmt.Fprintf(b, "- %s (%s)%s: %s\n  Components: %s\n  Data access: %s\n",
		f.Path, f.TypeName, platformTag, f.Purpose, f.Components, f.DataAccess)
}

// buildPrompts constructs the system and user prompts for the build phase.
func (p *Pipeline) buildPrompts(_ string, appName string, _ string, analysis *AnalysisResult, plan *PlannerResult) (string, string, error) {
	destination := canonicalBuildDestinationForShape(plan.GetPlatform(), plan.GetWatchProjectShape())
	// Build the append system prompt with coder rules + plan context
	basePrompt, err := composeCoderAppendPrompt("builder", plan.GetPlatform())
	if err != nil {
		return "", "", err
	}
	var appendPrompt strings.Builder
	appendPrompt.WriteString(basePrompt)

	// Add plan context
	appendPrompt.WriteString("\n\n<build-plan>\n")

	appendPrompt.WriteString("## Design\n")
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

	appendPrompt.WriteString("\n## Files (build in this order)\n")
	filesByPath := make(map[string]FilePlan, len(plan.Files))
	for _, f := range plan.Files {
		filesByPath[f.Path] = f
	}

	inBuildOrder := make(map[string]bool, len(plan.BuildOrder))
	for _, path := range plan.BuildOrder {
		inBuildOrder[path] = true
		if f, ok := filesByPath[path]; ok {
			appendBuildPlanFileEntry(&appendPrompt, f)
		}
	}

	// Include any files not in build_order, preserving original plan order.
	for _, f := range plan.Files {
		if inBuildOrder[f.Path] {
			continue
		}
		appendBuildPlanFileEntry(&appendPrompt, f)
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

	appendPrompt.WriteString("\n</build-plan>\n")

	// Inject rule content for each rule_key from embedded skill files
	if len(plan.RuleKeys) > 0 {
		appendPrompt.WriteString("\n<feature-rules>\n")
		for _, key := range plan.RuleKeys {
			content := loadRuleContent(key)
			if content != "" {
				appendPrompt.WriteString("\n")
				appendPrompt.WriteString(content)
				appendPrompt.WriteString("\n")
			}
		}
		appendPrompt.WriteString("</feature-rules>\n")
	}

	// Build user message
	var featureList strings.Builder
	for _, f := range analysis.Features {
		featureList.WriteString(fmt.Sprintf("- %s: %s\n", f.Name, f.Description))
	}

	var userMsg string
	if plan.IsMultiPlatform() {
		buildCmds := multiPlatformBuildCommands(appName, plan.GetPlatforms())
		var buildCmdStr strings.Builder
		for i, cmd := range buildCmds {
			fmt.Fprintf(&buildCmdStr, "%d. %s\n", i+1, cmd)
		}

		var sourceDirsList strings.Builder
		for _, plat := range plan.GetPlatforms() {
			suffix := PlatformSourceDirSuffix(plat)
			dirName := appName + suffix
			fmt.Fprintf(&sourceDirsList, "- %s/ — %s source (files with platform:%q)\n", dirName, PlatformDisplayName(plat), plat)
		}
		sourceDirsList.WriteString("- Shared/ — cross-platform code (files with platform:\"\")\n")

		userMsg = fmt.Sprintf(`Build the %s app following the plan in the system prompt.

App description: %s

Features:
%s
Core flow: %s

BEFORE WRITING CODE:
1. Use Glob to list all files in the project directory to understand the existing structure
2. Read the CLAUDE.md file to understand design tokens and architecture
3. Read project_config.json to understand the project configuration
Then proceed with writing code.

MULTI-PLATFORM SOURCE DIRECTORIES:
%s
INSTRUCTIONS:
1. The Xcode project is already configured with targets for all platforms.
2. Write ALL Swift files under the correct platform source directory based on the file's platform tag.
3. Extension files go under Targets/{ExtensionName}/.
4. Shared cross-platform types go under Shared/.
5. If you need additional permissions, extensions, or entitlements beyond the plan, use the xcodegen MCP tools.
6. After writing ALL files for ALL platforms, build each scheme in sequence:
%s7. If any build fails, read the errors, fix the Swift code, and rebuild.
8. Repeat until all builds succeed.
9. If Xcode says a scheme is missing, run: xcodebuild -list -project %s.xcodeproj and use the listed schemes.

IMPORTANT:
- Write files in the build order specified in the plan
- Use the exact type names and file paths from the plan
- Every View must have a #Preview block
- Each platform has its own @main App entry point`,
			analysis.AppName, analysis.Description, featureList.String(), analysis.CoreFlow,
			sourceDirsList.String(), buildCmdStr.String(), appName)
	} else {
		userMsg = fmt.Sprintf(`Build the %s app following the plan in the system prompt.

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
8. If Xcode says the scheme is missing, run: xcodebuild -list -project %s.xcodeproj and use the listed app scheme.

IMPORTANT:
- Write files in the build order specified in the plan
- Use the exact type names and file paths from the plan
- Every View must have a #Preview block`,
			analysis.AppName, analysis.Description, featureList.String(), analysis.CoreFlow,
			appName, appName, appName, destination, appName)
	}

	return appendPrompt.String(), userMsg, nil
}

// completionPrompts builds targeted prompts for unresolved planned files.
func (p *Pipeline) completionPrompts(appName string, projectDir string, plan *PlannerResult, report *FileCompletionReport) (string, string, error) {
	destination := canonicalBuildDestinationForShape(plan.GetPlatform(), plan.GetWatchProjectShape())
	basePrompt, err := composeCoderAppendPrompt("completion-recovery", plan.GetPlatform())
	if err != nil {
		return "", "", err
	}
	var appendPrompt strings.Builder
	appendPrompt.WriteString(basePrompt)
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
		if filePlan.Platform != "" {
			fileList.WriteString(fmt.Sprintf("  Platform: %s\n", filePlan.Platform))
		}
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

	var userMsg string
	if plan.IsMultiPlatform() {
		buildCmds := multiPlatformBuildCommands(appName, plan.GetPlatforms())
		var buildCmdStr strings.Builder
		for i, cmd := range buildCmds {
			fmt.Fprintf(&buildCmdStr, "%d. %s\n", i+1, cmd)
		}
		userMsg = fmt.Sprintf(`Complete the unresolved files from the original build plan.

Unresolved files:
%s
Required process:
1. Create/fix ONLY the unresolved files listed above.
2. Ensure each file contains the expected type name exactly.
3. Place files in the correct platform source directory based on the Platform field.
4. Keep existing already-valid files unchanged unless required for imports/signatures.
5. Build each scheme in sequence:
%s6. If any build fails, fix issues and rebuild.
7. If a scheme is missing, run: xcodebuild -list -project %s.xcodeproj and use the listed schemes.
8. Stop only when every unresolved file is complete and all builds succeed.`, fileList.String(), buildCmdStr.String(), appName)
	} else {
		userMsg = fmt.Sprintf(`Complete the unresolved files from the original build plan.

Unresolved files:
%s
Required process:
1. Create/fix ONLY the unresolved files listed above.
2. Ensure each file contains the expected type name exactly.
3. Keep existing already-valid files unchanged unless required for imports/signatures.
4. Run: xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet build
5. If build fails, fix issues and rebuild.
6. If the scheme is missing, run: xcodebuild -list -project %s.xcodeproj and use the listed app scheme.
7. Stop only when every unresolved file is complete and the build succeeds.`, fileList.String(), appName, appName, destination, appName)
	}

	return appendPrompt.String(), userMsg, nil
}
