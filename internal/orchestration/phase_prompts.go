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
	appendPromptSection(&b, "Planning Constraints", planningConstraints)
	appendPromptSection(&b, "Phase Skill", phaseSkill)
	if hints := formatIntentHintsForPrompt(intent); hints != "" {
		appendPromptSection(&b, "Intent Hints", hints)
	}
	return b.String(), nil
}

func composePlannerSystemPrompt(intent *IntentDecision) (string, error) {
	phaseSkill, err := loadPhaseSkillContent("planner")
	if err != nil {
		return "", err
	}

	var b strings.Builder
	appendPromptSection(&b, "Planner Base", plannerPrompt)
	appendPromptSection(&b, "Planning Constraints", planningConstraints)
	appendPromptSection(&b, "Phase Skill", phaseSkill)
	if hints := formatIntentHintsForPrompt(intent); hints != "" {
		appendPromptSection(&b, "Intent Hints", hints)
	}
	return b.String(), nil
}

func composeCoderAppendPrompt(phaseSkillName string) (string, error) {
	phaseSkill, err := loadPhaseSkillContent(phaseSkillName)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	appendPromptSection(&b, "Coder Base", coderPrompt)
	appendPromptSection(&b, "Shared Constraints", sharedConstraints)
	appendPromptSection(&b, "Phase Skill", phaseSkill)
	return b.String(), nil
}
