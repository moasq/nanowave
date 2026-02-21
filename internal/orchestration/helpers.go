package orchestration

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moasq/nanowave/internal/claude"
	"github.com/moasq/nanowave/internal/terminal"
)

// parseAnalysis parses the analyzer JSON response.
func parseAnalysis(result string) (*AnalysisResult, error) {
	cleaned := extractJSON(result)

	var analysis AnalysisResult
	if err := json.Unmarshal([]byte(cleaned), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse analysis: %w\nRaw output:\n%s", err, truncateStr(result, 500))
	}

	if analysis.AppName == "" {
		return nil, fmt.Errorf("analysis has no app name")
	}

	return &analysis, nil
}

// parsePlan parses the planner JSON response.
func parsePlan(result string) (*PlannerResult, error) {
	cleaned := extractJSON(result)

	var plan PlannerResult
	if err := json.Unmarshal([]byte(cleaned), &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w\nRaw output:\n%s", err, truncateStr(result, 500))
	}

	if len(plan.Files) == 0 {
		return nil, fmt.Errorf("plan has no files")
	}

	return &plan, nil
}

// extractJSON finds and extracts the first JSON object from a string.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)

	// Remove markdown code fences
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		var filtered []string
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				continue
			}
			filtered = append(filtered, line)
		}
		s = strings.Join(filtered, "\n")
	}

	// Find the first { and last }
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}

	return s
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func sanitizeToPascalCase(name string) string {
	var result strings.Builder
	capitalizeNext := true

	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			if capitalizeNext {
				if r >= 'a' && r <= 'z' {
					r = r - 32 // to upper
				}
				result.WriteRune(r)
				capitalizeNext = false
			} else {
				result.WriteRune(r)
			}
		} else {
			capitalizeNext = true
		}
	}
	return result.String()
}

// newProgressCallback returns a callback that updates a ProgressDisplay based on Claude Code streaming events.
func newProgressCallback(progress *terminal.ProgressDisplay) func(claude.StreamEvent) {
	return func(ev claude.StreamEvent) {
		switch ev.Type {
		case "tool_use":
			if ev.ToolName != "" {
				progress.OnToolUse(ev.ToolName, func(key string) string {
					return extractToolInputString(ev.ToolInput, key)
				})
			}
		case "assistant":
			if ev.Text != "" {
				progress.OnAssistantText(ev.Text)
			}
		}
	}
}

// showCost displays the total cost of a Claude Code response.
func showCost(resp *claude.Response) {
	if resp != nil && resp.TotalCostUSD > 0 {
		terminal.Detail("Cost", fmt.Sprintf("$%.4f", resp.TotalCostUSD))
	}
}

// extractSpinnerStatus extracts a short status line from assistant text for spinner display.
func extractSpinnerStatus(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	// Find first sentence boundary
	for i, ch := range text {
		if ch == '.' || ch == '\n' {
			text = text[:i]
			break
		}
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	// Truncate to fit spinner line
	const maxWidth = 60
	if len(text) > maxWidth {
		text = text[:maxWidth] + "..."
	}

	return text
}

// extractToolInputString extracts a string field from a tool input JSON.
func extractToolInputString(input json.RawMessage, key string) string {
	if len(input) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(input, &m); err != nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
