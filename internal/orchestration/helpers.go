package orchestration

import (
	"encoding/json"
	"fmt"
	"os/user"
	"strings"
	"unicode"

	"github.com/moasq/nanowave/internal/claude"
	"github.com/moasq/nanowave/internal/terminal"
)

// bundleIDPrefix returns a user-specific bundle ID prefix derived from the macOS username.
// Example: "mohammedal-quraini" → "com.mohammedalquraini"
func bundleIDPrefix() string {
	u, err := user.Current()
	if err != nil || u.Username == "" {
		return "com.app"
	}
	name := sanitizeBundleID(u.Username)
	if name == "" {
		return "com.app"
	}
	return "com." + name
}

// sanitizeBundleID strips non-alphanumeric characters from a string for use in a bundle ID.
func sanitizeBundleID(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

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

	// Platform validation
	if err := ValidatePlatform(plan.Platform); err != nil {
		return nil, err
	}
	if err := ValidateWatchShape(plan.WatchProjectShape); err != nil {
		return nil, err
	}

	// Reject device_family when platform is watchOS
	if IsWatchOS(plan.Platform) && plan.DeviceFamily != "" {
		return nil, fmt.Errorf("device_family must not be set when platform is watchOS (got %q)", plan.DeviceFamily)
	}

	// Filter rule_keys for platform compatibility
	if IsWatchOS(plan.Platform) {
		filtered, _ := FilterRuleKeysForPlatform(plan.Platform, plan.RuleKeys)
		plan.RuleKeys = filtered
	}

	// Validate extensions for platform compatibility
	if err := ValidateExtensionsForPlatform(plan.Platform, plan.Extensions); err != nil {
		return nil, err
	}

	return &plan, nil
}

// extractJSON finds and extracts the first JSON object from a string.
// Handles responses that contain thinking text before/after a ```json code block.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)

	// Try to extract content from a markdown code fence first.
	// This handles: text...\n```json\n{...}\n```\nmore text...
	if idx := strings.Index(s, "```"); idx >= 0 {
		// Find the opening fence line end
		fenceStart := idx
		lineEnd := strings.Index(s[fenceStart:], "\n")
		if lineEnd >= 0 {
			contentStart := fenceStart + lineEnd + 1
			// Find the closing fence
			closingFence := strings.Index(s[contentStart:], "```")
			if closingFence >= 0 {
				s = s[contentStart : contentStart+closingFence]
			} else {
				s = s[contentStart:]
			}
		}
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
		case "content_block_delta":
			if ev.Text != "" {
				progress.OnStreamingText(ev.Text)
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
