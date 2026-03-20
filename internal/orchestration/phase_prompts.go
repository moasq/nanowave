package orchestration

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/moasq/nanowave/internal/skills"
)

// loadPhaseSkillContent reads and combines all markdown content from a phase skill directory.
// Phase skills live under data/phases/{skillName}/ in the embedded skills FS.
func loadPhaseSkillContent(skillName string) (string, error) {
	dirPath := "data/phases/" + skillName
	if _, err := fs.ReadDir(skillsFS, dirPath); err != nil {
		return "", fmt.Errorf("phase skill %q not found: %w", skillName, err)
	}

	var parts []string
	if body, found := skills.ReadMarkdownBody("phases/" + skillName + "/SKILL.md"); found && strings.TrimSpace(body) != "" {
		parts = append(parts, strings.TrimSpace(body))
	}

	seen := map[string]bool{
		dirPath + "/SKILL.md": true,
	}
	orderedRefs := []string{
		dirPath + "/references/workflow.md",
		dirPath + "/references/output-format.md",
		dirPath + "/references/common-mistakes.md",
		dirPath + "/references/examples.md",
	}
	for _, p := range orderedRefs {
		relPath := strings.TrimPrefix(p, "data/")
		if body, found := skills.ReadMarkdownBody(relPath); found && strings.TrimSpace(body) != "" {
			parts = append(parts, strings.TrimSpace(body))
			seen[p] = true
		}
	}

	var extras []string
	_ = fs.WalkDir(skillsFS, dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		if seen[path] {
			return nil
		}
		extras = append(extras, path)
		return nil
	})
	sort.Strings(extras)
	for _, p := range extras {
		relPath := strings.TrimPrefix(p, "data/")
		if body, found := skills.ReadMarkdownBody(relPath); found && strings.TrimSpace(body) != "" {
			parts = append(parts, strings.TrimSpace(body))
		}
	}

	content := strings.TrimSpace(strings.Join(parts, "\n\n"))
	if content == "" {
		return "", fmt.Errorf("phase skill %q has no loadable markdown content", skillName)
	}
	return content, nil
}

// formatIntentHintsForPrompt renders intent decision hints as a prompt section.
func formatIntentHintsForPrompt(intent *IntentDecision) string {
	if intent == nil {
		return ""
	}
	var lines []string
	if intent.PlatformHint != "" {
		lines = append(lines, fmt.Sprintf("- platform_hint: %s", intent.PlatformHint))
	}
	if len(intent.PlatformHints) > 1 {
		lines = append(lines, fmt.Sprintf("- platform_hints: [%s]", strings.Join(intent.PlatformHints, ", ")))
	}
	if intent.DeviceFamilyHint != "" {
		lines = append(lines, fmt.Sprintf("- device_family_hint: %s", intent.DeviceFamilyHint))
	}
	if intent.WatchProjectShapeHint != "" {
		lines = append(lines, fmt.Sprintf("- watch_project_shape_hint: %s", intent.WatchProjectShapeHint))
	}
	if intent.Operation != "" && intent.Operation != "unknown" {
		lines = append(lines, fmt.Sprintf("- operation: %s", intent.Operation))
	}
	if intent.Confidence > 0 {
		lines = append(lines, fmt.Sprintf("- confidence: %.2f", intent.Confidence))
	}
	if intent.Reason != "" {
		lines = append(lines, fmt.Sprintf("- reason: %s", intent.Reason))
	}
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Intent hints (advisory only; explicit user request wins):\n")
	for _, line := range lines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

// composeAnalyzerSystemPrompt builds the system prompt for the analyzer phase.
func composeAnalyzerSystemPrompt(intent *IntentDecision) (string, error) {
	phaseSkill, err := loadPhaseSkillContent("analyzer")
	if err != nil {
		return "", err
	}

	var b strings.Builder
	appendPromptSection(&b, "Analyzer Base", analyzerPrompt)
	appendXMLSection(&b, "constraints", planningConstraints)
	appendPromptSection(&b, "Phase Skill", phaseSkill)
	if hints := formatIntentHintsForPrompt(intent); hints != "" {
		appendPromptSection(&b, "Intent Hints", hints)
	}
	return b.String(), nil
}

// composePlannerSystemPrompt builds the system prompt for the planner phase.
func composePlannerSystemPrompt(intent *IntentDecision, platform string) (string, error) {
	phaseSkill, err := loadPhaseSkillContent("planner")
	if err != nil {
		return "", err
	}

	var b strings.Builder
	appendPromptSection(&b, "Planner Base", plannerPromptForPlatform(platform))
	appendXMLSection(&b, "constraints", planningConstraints)
	appendPromptSection(&b, "Phase Skill", phaseSkill)
	if hints := formatIntentHintsForPrompt(intent); hints != "" {
		appendPromptSection(&b, "Intent Hints", hints)
	}
	return b.String(), nil
}

// composeCoderAppendPrompt builds the system prompt append for build/edit/fix/completion.
func composeCoderAppendPrompt(phaseSkillName, platform string) (string, error) {
	phaseSkill, err := loadPhaseSkillContent(phaseSkillName)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	appendPromptSection(&b, "Coder Base", coderPromptForPlatform(platform))
	appendXMLSection(&b, "constraints", sharedConstraints)
	appendPromptSection(&b, "Phase Skill", phaseSkill)
	appendXMLSection(&b, "verification", composeSelfCheck(platform))

	return b.String(), nil
}

func appendPromptSection(b *strings.Builder, title, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	if title != "" {
		b.WriteString("## ")
		b.WriteString(title)
		b.WriteString("\n\n")
	}
	b.WriteString(content)
}

// appendXMLSection wraps content in XML tags for structured prompt injection.
func appendXMLSection(b *strings.Builder, tag, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	if b.Len() > 0 {
		b.WriteString("\n\n")
	}
	b.WriteString("<")
	b.WriteString(tag)
	b.WriteString(">\n")
	b.WriteString(content)
	b.WriteString("\n</")
	b.WriteString(tag)
	b.WriteString(">")
}

// ComposeAgenticSystemPrompt assembles a single system prompt for agentic mode.
func ComposeAgenticSystemPrompt(ac ActionContext, catalogRoot string) string {
	platform := ac.Platform
	if platform == "" {
		platform = PlatformIOS
	}

	var b strings.Builder

	appendPromptSection(&b, "Role", `You are an autonomous Apple app builder. You make all decisions yourself. Never ask clarifying questions.`)

	appendPromptSection(&b, "Coder", coderPromptForPlatform(platform))

	// Platform & scope constraints — critical for correct platform detection
	appendXMLSection(&b, "constraints", planningConstraints)

	// Architecture & AppTheme constraints — critical for code quality
	appendXMLSection(&b, "architecture-constraints", sharedConstraints)

	// Inject ALL core rules directly into the system prompt — same pattern as
	// the old build_prompts.go <feature-rules> injection from main branch.
	// This ensures rules are always in context even if the agent skips nw_scaffold_project.
	coreRules := loadCoreRulesForPrompt(platform)
	if coreRules != "" {
		appendPromptSection(&b, "Core Rules (MUST follow — violations are build failures)", coreRules)
	}

	if !ac.IsEdit() {
		buildWorkflow := `For NEW builds, you MUST follow this exact workflow. Skipping steps is a critical failure.

1. **Plan mentally** — decide app name, 1–2 core screens, models, and which skill keys you need. Set the correct platform in your plan JSON (ios, watchos, tvos, visionos, or macos).
2. **Call nw_get_skills** with list_available:true to see available skills, then load relevant feature skills (e.g. "media", "navigation-patterns", "charts") BEFORE writing any code.
3. **Call nw_scaffold_project** with your plan JSON — this creates the project directory, project.yml, asset catalogs, .claude/rules/, CLAUDE.md, and runs xcodegen. Do NOT create these manually.
4. **Write all Swift source files** following the rules loaded in .claude/rules/.
5. **Call nw_xcode_build** to compile. Use the correct platform parameter (ios, watchos, tvos, visionos, macos).
6. **For iOS apps:** Call nw_capture_screenshots to visually review the UI. Read the screenshot, load nw_get_skills with key "ui-review", and fix issues. **For macOS/watchOS/tvOS/visionOS:** Skip screenshots and carefully review your code for platform-specific correctness instead.
7. **Call nw_finalize_project** to git init and commit.

CRITICAL: You MUST call nw_scaffold_project for new builds. Do NOT manually create project.yml, asset catalogs, or .claude/ directories. The scaffold tool handles all of this correctly.
CRITICAL: You MUST call nw_get_skills to load feature-specific skills BEFORE implementing features. Do NOT rely on your training data alone.
CRITICAL: Set "platform" correctly in your plan_json. The platform determines build destinations, asset catalogs, project.yml structure, and which APIs are available.`

		// Add platform-specific build guidance
		switch {
		case IsMacOS(platform):
			buildWorkflow += `

MACOS BUILD NOTES:
- Use platform "macos" in nw_scaffold_project plan_json.
- macOS apps use NavigationSplitView (sidebar + detail), NOT TabView.
- Include a Settings scene for preferences (auto-wires Cmd+,).
- Add CommandMenu/CommandGroup for keyboard shortcuts.
- Use .frame(minWidth:minHeight:) for proper window sizing.
- No UIKit — macOS is SwiftUI + AppKit bridge when needed.
- Build destination: generic/platform=macOS (no CODE_SIGNING_ALLOWED=NO).`
		case IsWatchOS(platform):
			buildWorkflow += `

WATCHOS BUILD NOTES:
- Use platform "watchos" in nw_scaffold_project plan_json.
- watchOS apps are SwiftUI-only — NO UIKit imports at all.
- Use NavigationStack (not NavigationSplitView) for compact watch navigation.
- Keep UI minimal — 1-2 screens max, no large images.
- Digital Crown support via .digitalCrownRotation where appropriate.
- WKInterfaceDevice.default().play(.click) for haptics, not UIFeedbackGenerator.`
		case IsTvOS(platform):
			buildWorkflow += `

TVOS BUILD NOTES:
- Use platform "tvos" in nw_scaffold_project plan_json.
- tvOS uses focus-based navigation — add .focusable() on all interactive elements.
- Use onMoveCommand, onPlayPauseCommand, onExitCommand for Siri Remote input.
- Size text and images for 10-foot viewing distance.
- No touch gestures — tvOS has no touch screen.
- No camera, biometrics, healthkit, haptics, maps, or speech APIs.`
		case IsVisionOS(platform):
			buildWorkflow += `

VISIONOS BUILD NOTES:
- Use platform "visionos" in nw_scaffold_project plan_json.
- visionOS uses SwiftUI for 2D chrome, RealityKit/RealityView for 3D content.
- No UIKit imports — visionOS is SwiftUI + RealityKit.
- Use volumes (.windowStyle(.volumetric)) for 3D content.
- No dark mode concept — glass material auto-adapts.
- Spatial gestures via SpatialTapGesture, DragGesture.
- No camera, healthkit, haptics, maps, or speech APIs.`
		}

		appendPromptSection(&b, "Build Workflow — MANDATORY", buildWorkflow)
	}

	skillsHint := `Feature-specific skills (camera, authentication, media, charts, widgets, navigation-patterns, etc.) are available via the nw_get_skills tool. Call nw_get_skills with list_available:true to discover all available skills. Load relevant skills BEFORE implementing features.

When the user pastes images, determine whether each image is a design reference (visual guide) or an asset to embed in the app (icon, logo, background). For assets, call nw_get_skills with key "user-assets" for step-by-step integration instructions.`
	if platform != PlatformIOS {
		skillsHint += fmt.Sprintf("\n\nPlatform-specific rules for %s are also already loaded in your context.", PlatformDisplayName(platform))
	}
	appendPromptSection(&b, "Skills", skillsHint)

	// Platform-specific verification checklist
	appendXMLSection(&b, "verification", composeSelfCheck(platform))

	postBuildReview := `After a successful nw_xcode_build on a new build (not quick edits):

**For iOS apps:**
1. Call nw_capture_screenshots to capture the launch screen in the simulator.
2. Read the screenshot to visually evaluate the UI.
3. Load nw_get_skills with key "ui-review" for the evaluation checklist.
4. Collect all findings (layout, text, colors, sample data, components).
5. Fix issues ONE AT A TIME — rebuild and recapture after each fix to avoid cascading breakage.
6. After all fixes, do a final screenshot capture to verify.
7. Only then call nw_finalize_project.

**For macOS/watchOS/tvOS/visionOS apps:**
1. Review your code manually for platform-specific correctness.
2. Verify no UIKit imports on watchOS/visionOS. Verify macOS uses NavigationSplitView, Settings scene, and keyboard shortcuts. Verify tvOS uses focus-based navigation.
3. Call nw_finalize_project.`
	appendPromptSection(&b, "Post-Build Review", postBuildReview)

	appendPromptSection(&b, "Backend Integrations", composeIntegrationSection(ac.ActiveIntegrations))

	if ac.IsEdit() {
		editCtx := fmt.Sprintf("Operating on existing project:\n- Project dir: %s\n- App name: %s\n- Platform: %s", ac.ProjectDir, ac.AppName, ac.Platform)
		if len(ac.Platforms) > 1 {
			editCtx += fmt.Sprintf("\n- Platforms: %s", strings.Join(ac.Platforms, ", "))
		}
		appendPromptSection(&b, "Edit Context", editCtx)
	} else if catalogRoot != "" {
		appendPromptSection(&b, "Project Location", fmt.Sprintf(
			"CRITICAL: Create the project directory inside `%s`. For example, if the app is called MyApp, create it at `%s/MyApp/`. Do NOT create projects anywhere else.",
			catalogRoot, catalogRoot))
	}

	return b.String()
}

// composeIntegrationSection generates the Backend Integrations prompt section.
func composeIntegrationSection(activeIntegrations []string) string {
	active := make(map[string]bool, len(activeIntegrations))
	for _, id := range activeIntegrations {
		active[id] = true
	}

	var b strings.Builder

	// Describe what IS available
	if len(activeIntegrations) > 0 {
		b.WriteString("The following backend integrations are configured and available for this project:\n")
		for _, id := range activeIntegrations {
			switch id {
			case "supabase":
				b.WriteString("- **Supabase**: Configured. You MAY use Supabase for authentication, database, storage, and realtime. Use the `nw_get_skills` tool with key `repositories` to learn the repository pattern. MCP tools for Supabase are available.\n")
			case "revenuecat":
				b.WriteString("- **RevenueCat**: Configured. You MAY use RevenueCat for in-app purchases and subscriptions. Use the `nw_get_skills` tool with key `paywall` to learn the paywall pattern. MCP tools for RevenueCat are available.\n")
			default:
				b.WriteString(fmt.Sprintf("- **%s**: Configured and available.\n", id))
			}
		}
	}

	// Describe what is NOT yet configured
	var unconfigured []string
	if !active["supabase"] {
		unconfigured = append(unconfigured, "supabase")
	}
	if !active["revenuecat"] {
		unconfigured = append(unconfigured, "revenuecat")
	}

	if len(unconfigured) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("**The following backends are NOT yet configured:**\n")
		for _, id := range unconfigured {
			b.WriteString(fmt.Sprintf("- **%s** is not configured.\n", integrationDisplayName(id)))
		}
		b.WriteString("\nWhen the user asks for features that require an unconfigured backend (authentication, database, subscriptions, in-app purchases, paywalls, etc.), you MUST:\n")
		b.WriteString("1. Explain what you'll build and which backend it needs\n")
		b.WriteString("2. End your response by telling the user to run the setup command:\n")
		for _, id := range unconfigured {
			name := integrationDisplayName(id)
			b.WriteString(fmt.Sprintf("   - For %s: \"Run `/%s` to connect your %s account, then I'll wire everything up.\"\n", name, id, name))
		}
		b.WriteString("3. Do NOT generate any code, imports, or SPM packages for the unconfigured backend. Wait until the user confirms setup is complete.\n")
		if !active["supabase"] && !active["revenuecat"] {
			b.WriteString("\nCurrently this app has no backend. If the user does not request backend features, store all data on-device using SwiftData or UserDefaults.")
		}
	}

	return b.String()
}

func integrationDisplayName(id string) string {
	switch id {
	case "supabase":
		return "Supabase"
	case "revenuecat":
		return "RevenueCat"
	default:
		return id
	}
}

// loadCoreRulesForPrompt reads all core rules from the embedded FS and returns
// them as a single string for injection into the system prompt.
// This ensures the agent always has the rules in context, even if it skips
// nw_scaffold_project (which writes them to .claude/rules/ on disk).
// Follows the same pattern as the old build_prompts.go <feature-rules> injection.
func loadCoreRulesForPrompt(platform string) string {
	var b strings.Builder

	appendRule := func(content string) {
		if content == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(content)
	}

	// Core rules from data/core/ — always loaded
	coreKeys := []string{
		"scope",
		"swift-conventions",
		"mvvm-architecture",
		"file-structure",
		"forbidden-patterns",
	}
	for _, key := range coreKeys {
		appendRule(loadCoreRuleAdapted(key, platform, nil))
	}

	// Always-on rules from data/always/ (components, design-system, layout, navigation, swiftui, review)
	for _, key := range []string{"components", "design-system", "layout", "navigation", "swiftui", "review"} {
		appendRule(skills.LoadRuleContent(key))
	}

	// Platform-conditional always rules from data/always-{platform}/
	// Read both bare .md files AND SKILL.md from subdirectories.
	platDir := platformAlwaysDir(platform)
	if platDir != "" {
		entries, err := fs.ReadDir(skillsFS, "data/"+platDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					// Try loading SKILL.md from the subdirectory
					body, found := skills.ReadMarkdownBody(platDir + "/" + entry.Name() + "/SKILL.md")
					if found && body != "" {
						appendRule(body)
					}
				} else if strings.HasSuffix(entry.Name(), ".md") {
					body, found := skills.ReadMarkdownBody(platDir + "/" + entry.Name())
					if found && body != "" {
						appendRule(body)
					}
				}
			}
		}
	}

	return b.String()
}
