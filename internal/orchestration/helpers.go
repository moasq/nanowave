package orchestration

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/moasq/nanowave/internal/claude"
	"github.com/moasq/nanowave/internal/terminal"
)

// bundleIDPrefix returns a user-specific bundle ID prefix derived from the macOS username.
// Example: "jane-doe" → "com.janedoe"
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
	analysis, err := parseClaudeJSON[AnalysisResult](result, "analysis")
	if err != nil {
		return nil, err
	}

	if analysis.AppName == "" {
		return nil, fmt.Errorf("analysis has no app name")
	}

	return analysis, nil
}

// parseIntentDecision parses the intent-router JSON response.
func parseIntentDecision(result string) (*IntentDecision, error) {
	decision, err := parseClaudeJSON[IntentDecision](result, "intent decision")
	if err != nil {
		return nil, err
	}

	// Accept only canonical operation values; fall back to "build" for anything else.
	// The skill docs instruct the AI to return exactly "build", "edit", or "fix".
	switch decision.Operation {
	case "build", "edit", "fix":
		// valid — keep as-is
	default:
		decision.Operation = "build"
	}

	// Validate platform_hints — drop invalid entries gracefully.
	if len(decision.PlatformHints) > 0 {
		decision.PlatformHints = ValidatePlatforms(decision.PlatformHints)
		// If PlatformHint is empty, set it to first valid entry from PlatformHints
		if decision.PlatformHint == "" && len(decision.PlatformHints) > 0 {
			decision.PlatformHint = decision.PlatformHints[0]
		}
	}

	// If the AI returned an unrecognized platform_hint, fall back to iOS
	// rather than crashing. The skill docs instruct it to return only valid
	// values, but graceful degradation is still important.
	if err := ValidatePlatform(decision.PlatformHint); err != nil {
		decision.PlatformHint = PlatformIOS
	}
	if err := ValidateWatchShape(decision.WatchProjectShapeHint); err != nil {
		decision.WatchProjectShapeHint = ""
	}
	switch decision.DeviceFamilyHint {
	case "", "iphone", "ipad", "universal":
		// valid
	default:
		decision.DeviceFamilyHint = ""
	}

	if decision.Confidence < 0 {
		decision.Confidence = 0
	}
	if decision.Confidence > 1 {
		decision.Confidence = 1
	}
	if decision.Operation == "" {
		decision.Operation = "unknown"
	}

	return decision, nil
}

// parsePlan parses the planner JSON response.
func parsePlan(result string) (*PlannerResult, error) {
	plan, err := parseClaudeJSON[PlannerResult](result, "plan")
	if err != nil {
		return nil, err
	}
	normalizePlannerResult(plan)

	if len(plan.Files) == 0 {
		return nil, fmt.Errorf("plan has no files")
	}

	// Validate Platforms entries
	if len(plan.Platforms) > 0 {
		plan.Platforms = ValidatePlatforms(plan.Platforms)
	}

	// Platform validation
	if err := ValidatePlatform(plan.Platform); err != nil {
		return nil, err
	}
	if err := ValidateWatchShape(plan.WatchProjectShape); err != nil {
		return nil, err
	}

	// Reject device_family when platform is watchOS (single-platform only)
	if !plan.IsMultiPlatform() && IsWatchOS(plan.Platform) && plan.DeviceFamily != "" {
		return nil, fmt.Errorf("device_family must not be set when platform is watchOS (got %q)", plan.DeviceFamily)
	}

	// Validate FilePlan.Platform entries — invalid values gracefully default to ""
	for i := range plan.Files {
		if plan.Files[i].Platform != "" {
			if err := ValidatePlatform(plan.Files[i].Platform); err != nil {
				plan.Files[i].Platform = ""
			}
		}
	}

	// Validate ExtensionPlan.Platform entries
	for i := range plan.Extensions {
		if plan.Extensions[i].Platform != "" {
			if err := ValidatePlatform(plan.Extensions[i].Platform); err != nil {
				plan.Extensions[i].Platform = ""
			}
		}
	}

	// Filter rule_keys for platform compatibility (single-platform)
	if !plan.IsMultiPlatform() && IsWatchOS(plan.Platform) {
		filtered, _ := FilterRuleKeysForPlatform(plan.Platform, plan.RuleKeys)
		plan.RuleKeys = filtered
	}

	// For multi-platform, filter rule_keys for each platform and keep the union
	if plan.IsMultiPlatform() {
		allKeys := map[string]bool{}
		for _, plat := range plan.GetPlatforms() {
			filtered, _ := FilterRuleKeysForPlatform(plat, plan.RuleKeys)
			for _, k := range filtered {
				allKeys[k] = true
			}
		}
		var merged []string
		for _, k := range plan.RuleKeys {
			if allKeys[k] {
				merged = append(merged, k)
			}
		}
		plan.RuleKeys = merged
	}

	// Validate extensions for platform compatibility
	if plan.IsMultiPlatform() {
		// For multi-platform, validate each extension against its target platform
		for _, ext := range plan.Extensions {
			if ext.Platform != "" {
				if err := ValidateExtensionsForPlatform(ext.Platform, []ExtensionPlan{ext}); err != nil {
					return nil, err
				}
			}
		}
	} else {
		if err := ValidateExtensionsForPlatform(plan.Platform, plan.Extensions); err != nil {
			return nil, err
		}
	}

	return plan, nil
}

func normalizePlannerResult(plan *PlannerResult) {
	if plan == nil {
		return
	}

	plan.Platform = normalizePlannerPlatform(plan.Platform)
	plan.WatchProjectShape = normalizePlannerWatchShape(plan.WatchProjectShape)

	// Normalize Platforms entries
	for i, p := range plan.Platforms {
		plan.Platforms[i] = normalizePlannerPlatform(p)
	}

	// When Platforms has entries, set Platform to first entry for backward compat.
	if len(plan.Platforms) > 0 && plan.Platform == "" {
		plan.Platform = plan.Platforms[0]
	}

	// If the planner emits a watch project shape, it intends a watch build path.
	if plan.WatchProjectShape != "" && !IsWatchOS(plan.Platform) && !plan.IsMultiPlatform() {
		plan.Platform = PlatformWatchOS
	}

	// Default WatchProjectShape when watchOS is in a multi-platform list
	if plan.IsMultiPlatform() && plan.WatchProjectShape == "" {
		for _, p := range plan.GetPlatforms() {
			if IsWatchOS(p) {
				plan.WatchProjectShape = WatchShapePaired
				break
			}
		}
	}

	// watchOS-only plans should not carry iOS device family hints.
	if !plan.IsMultiPlatform() && IsWatchOS(plan.Platform) {
		plan.DeviceFamily = ""
	}

	// Normalize packages: ensure non-nil, drop empty names, deduplicate.
	if plan.Packages == nil {
		plan.Packages = []PackagePlan{}
	} else {
		seen := make(map[string]bool)
		filtered := plan.Packages[:0]
		for _, pkg := range plan.Packages {
			name := strings.TrimSpace(pkg.Name)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			pkg.Name = name
			filtered = append(filtered, pkg)
		}
		plan.Packages = filtered
	}
}

// normalizePlannerPlatform gracefully handles unrecognized platform values
// from the AI planner. If the value is not a recognized constant, it passes
// through to ValidatePlatform which will reject it. Empty means default (ios).
func normalizePlannerPlatform(platform string) string {
	trimmed := strings.TrimSpace(platform)
	if trimmed == "" {
		return ""
	}
	// Accept the canonical constants as-is
	lower := strings.ToLower(trimmed)
	switch lower {
	case PlatformIOS, PlatformWatchOS, PlatformTvOS:
		return lower
	default:
		// Unrecognized — pass through; ValidatePlatform will catch it downstream
		return lower
	}
}

// normalizePlannerWatchShape gracefully handles the watch_project_shape field.
// Only exact canonical values are accepted; anything else passes through to
// ValidateWatchShape which will reject it.
func normalizePlannerWatchShape(shape string) string {
	trimmed := strings.TrimSpace(shape)
	if trimmed == "" {
		return ""
	}
	switch trimmed {
	case WatchShapeStandalone, WatchShapePaired:
		return trimmed
	default:
		// Unrecognized — pass through; ValidateWatchShape will catch it downstream
		return trimmed
	}
}

func parseClaudeJSON[T any](result string, label string) (*T, error) {
	cleaned := extractJSON(result)

	var parsed T
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w\nRaw output:\n%s", label, err, truncateStr(result, 500))
	}

	return &parsed, nil
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

// uniqueProjectDir returns a project directory path that does not already exist.
// If <catalogDir>/<appName> is free it is returned as-is.
// Otherwise it appends a counter: <appName>2, <appName>3, …
func uniqueProjectDir(catalogDir, appName string) string {
	candidate := filepath.Join(catalogDir, appName)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	for n := 2; n <= 999; n++ {
		candidate = filepath.Join(catalogDir, fmt.Sprintf("%s%d", appName, n))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return candidate
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
