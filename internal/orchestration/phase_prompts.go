package orchestration

import (
	"fmt"
	"strings"
)

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

func composeAnalyzerSystemPrompt(intent *IntentDecision) string {
	var b strings.Builder
	appendPromptSection(&b, "Analyzer Base", analyzerPrompt)
	appendXMLSection(&b, "constraints", planningConstraints)
	if hints := formatIntentHintsForPrompt(intent); hints != "" {
		appendPromptSection(&b, "Intent Hints", hints)
	}
	return b.String()
}

func composePlannerSystemPrompt(intent *IntentDecision, platform string) string {
	var b strings.Builder
	appendPromptSection(&b, "Planner Base", plannerPromptForPlatform(platform))
	appendXMLSection(&b, "constraints", planningConstraints)
	if hints := formatIntentHintsForPrompt(intent); hints != "" {
		appendPromptSection(&b, "Intent Hints", hints)
	}
	return b.String()
}

func composeCoderAppendPrompt(platform string) string {
	var b strings.Builder
	appendPromptSection(&b, "Coder Base", coderPromptForPlatform(platform))
	appendXMLSection(&b, "constraints", sharedConstraints)
	appendXMLSection(&b, "verification", composeSelfCheck(platform))
	return b.String()
}

// ComposeAgenticSystemPrompt assembles a single system prompt for agentic mode.
// Provides domain knowledge (constraints, design rules, quality checks) — no
// rigid workflow steps. The LLM decides how to use the available tools.
func ComposeAgenticSystemPrompt(ac ActionContext) string {
	platform := ac.Platform
	if platform == "" {
		platform = PlatformIOS
	}

	var b strings.Builder

	appendPromptSection(&b, "Role", `You are an autonomous Apple app builder. You have tools to set up workspaces, scaffold Xcode projects, build, verify, and finalize. You make all decisions yourself. Never ask clarifying questions.`)

	appendXMLSection(&b, "constraints", planningConstraints)
	appendXMLSection(&b, "architecture", sharedConstraints)

	appendPromptSection(&b, "Coder", coderPromptForPlatform(platform))

	appendXMLSection(&b, "verification", composeSelfCheck(platform))

	if ac.IsEdit() {
		editCtx := fmt.Sprintf("Operating on existing project:\n- Project dir: %s\n- App name: %s\n- Platform: %s", ac.ProjectDir, ac.AppName, ac.Platform)
		if len(ac.Platforms) > 1 {
			editCtx += fmt.Sprintf("\n- Platforms: %s", strings.Join(ac.Platforms, ", "))
		}
		appendPromptSection(&b, "Edit Context", editCtx)
	}

	return b.String()
}

func composeSelfCheck(platform string) string {
	base := `Before completing each file, verify every item:
- [ ] No raw .font() — all fonts via AppTheme.Fonts.* (reason: centralized tokens enable theme changes)
- [ ] No raw .foregroundStyle(.white/.black/.red) — all colors via AppTheme.Colors.* (reason: consistency)
- [ ] No raw .padding(N) or VStack(spacing: N) — all spacing via AppTheme.Spacing.* (reason: consistency)
- [ ] @Observable used, NOT ObservableObject. @State with @Observable, NOT @StateObject.
- [ ] No type re-declarations — each type defined in exactly one file
- [ ] Every View file includes #Preview
- [ ] Every async view uses Loadable<T> switch with loading, empty, data, and error states
- [ ] Every mutation button disabled while in-progress with inline spinner
- [ ] Empty states use ContentUnavailableView with action button
- [ ] Error states show user-friendly message with retry button`

	switch {
	case IsMacOS(platform):
		base += `
- [ ] Settings scene present for preferences (auto-wires Cmd+,)
- [ ] CommandMenu actions wired via @FocusedValue — not empty closures
- [ ] .keyboardShortcut() on every primary action and menu item
- [ ] .disabled(value == nil) on every CommandMenu button`
	case IsWatchOS(platform):
		base += `
- [ ] No UIKit imports — watchOS is SwiftUI-only
- [ ] NavigationStack used (not NavigationSplitView) for watch navigation`
	case IsTvOS(platform):
		base += `
- [ ] Focus-based navigation with .focusable() on interactive elements
- [ ] No small tap targets — tvOS uses focus system, not touch`
	case IsVisionOS(platform):
		base += `
- [ ] RealityView used for 3D content, SwiftUI for 2D chrome
- [ ] No UIKit imports — visionOS is SwiftUI + RealityKit`
	}
	return base
}
