package terminal

import (
	"regexp"
	"strings"
)

var (
	boldRe      = regexp.MustCompile(`\*\*(.+?)\*\*`)
	inlineCodeRe = regexp.MustCompile("`([^`]+)`")
	headerRe    = regexp.MustCompile(`^(#{1,3})\s+(.+)$`)
	bulletRe    = regexp.MustCompile(`^[-*]\s+(.+)$`)
	numberedRe  = regexp.MustCompile(`^(\d+\.)\s+(.+)$`)
	ruleRe      = regexp.MustCompile(`^-{3,}$`)
)

// RenderMarkdown converts markdown text to ANSI-styled terminal output.
// Handles headers, bold, inline code, bullets, numbered lists, and rules.
// Code blocks (```) are skipped entirely.
func RenderMarkdown(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Toggle code blocks — skip their content
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		// Empty lines → preserved for paragraph spacing
		if trimmed == "" {
			out = append(out, "")
			continue
		}

		// Horizontal rule
		if ruleRe.MatchString(trimmed) {
			out = append(out, "  "+Dim+strings.Repeat("─", 40)+Reset)
			continue
		}

		// Headers
		if m := headerRe.FindStringSubmatch(trimmed); m != nil {
			out = append(out, "  "+Bold+strings.ToUpper(m[2])+Reset)
			continue
		}

		// Bullet lists
		if m := bulletRe.FindStringSubmatch(trimmed); m != nil {
			out = append(out, "    \u2022 "+applyInline(m[1]))
			continue
		}

		// Numbered lists
		if m := numberedRe.FindStringSubmatch(trimmed); m != nil {
			out = append(out, "  "+m[1]+" "+applyInline(m[2]))
			continue
		}

		// Regular text
		out = append(out, "  "+applyInline(trimmed))
	}

	return strings.Join(out, "\n") + "\n"
}

// applyInline applies bold and inline code formatting.
func applyInline(s string) string {
	s = boldRe.ReplaceAllString(s, Bold+"${1}"+Reset)
	s = inlineCodeRe.ReplaceAllString(s, Cyan+"${1}"+Reset)
	return s
}
