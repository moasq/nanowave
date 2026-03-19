package terminal

import (
	"regexp"
	"strings"
)

var (
	boldRe       = regexp.MustCompile(`\*\*(.+?)\*\*`)
	inlineCodeRe = regexp.MustCompile("`([^`]+)`")
	headerRe     = regexp.MustCompile(`^(#{1,3})\s+(.+)$`)
	bulletRe     = regexp.MustCompile(`^[-*]\s+(.+)$`)
	numberedRe   = regexp.MustCompile(`^(\d+\.)\s+(.+)$`)
	ruleRe       = regexp.MustCompile(`^-{3,}$`)
	linkRe       = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	tableRowRe   = regexp.MustCompile(`^\|(.+)\|$`)
	tableSepRe   = regexp.MustCompile(`^\|[\s:]*[-]+`)
)

// RenderMarkdown converts markdown text to ANSI-styled terminal output.
// Handles headers, bold, inline code, bullets, numbered lists, rules,
// links, tables, and code blocks.
func RenderMarkdown(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Toggle code blocks — render content dimmed
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			out = append(out, "  "+Dim+"  "+trimmed+Reset)
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

		// Table separator rows (|---|---|) → skip
		if tableSepRe.MatchString(trimmed) {
			continue
		}

		// Table rows (| col | col |) → render as aligned text
		if m := tableRowRe.FindStringSubmatch(trimmed); m != nil {
			cells := strings.Split(m[1], "|")
			var parts []string
			for _, cell := range cells {
				cell = strings.TrimSpace(cell)
				if cell != "" {
					parts = append(parts, applyInline(cell))
				}
			}
			if len(parts) > 0 {
				out = append(out, "  "+strings.Join(parts, Dim+" · "+Reset))
			}
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

// applyInline applies bold, inline code, and link formatting.
func applyInline(s string) string {
	// Links: [text](url) → text (dimmed url)
	s = linkRe.ReplaceAllStringFunc(s, func(match string) string {
		m := linkRe.FindStringSubmatch(match)
		if len(m) == 3 {
			return m[1] + Dim + " (" + m[2] + ")" + Reset
		}
		return match
	})
	s = boldRe.ReplaceAllString(s, Bold+"${1}"+Reset)
	s = inlineCodeRe.ReplaceAllString(s, Cyan+"${1}"+Reset)
	return s
}
