package orchestration

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

func loadPhaseSkillContent(skillName string) (string, error) {
	dirPath := fmt.Sprintf("skills/phases/%s", skillName)
	if _, err := fs.ReadDir(skillsFS, dirPath); err != nil {
		return "", fmt.Errorf("phase skill %q not found: %w", skillName, err)
	}

	var parts []string
	if body, found := readEmbeddedMarkdownBody(dirPath + "/SKILL.md"); found && strings.TrimSpace(body) != "" {
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
		if body, found := readEmbeddedMarkdownBody(p); found && strings.TrimSpace(body) != "" {
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
		if body, found := readEmbeddedMarkdownBody(p); found && strings.TrimSpace(body) != "" {
			parts = append(parts, strings.TrimSpace(body))
		}
	}

	content := strings.TrimSpace(strings.Join(parts, "\n\n"))
	if content == "" {
		return "", fmt.Errorf("phase skill %q has no loadable markdown content", skillName)
	}
	return content, nil
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
