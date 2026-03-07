package terminal

import (
	"strings"
	"testing"
)

func TestRenderMarkdownBold(t *testing.T) {
	out := RenderMarkdown("This is **bold** text.")
	if !strings.Contains(out, Bold+"bold"+Reset) {
		t.Errorf("expected bold ANSI, got: %q", out)
	}
}

func TestRenderMarkdownInlineCode(t *testing.T) {
	out := RenderMarkdown("Run `asc builds list` now.")
	if !strings.Contains(out, Cyan+"asc builds list"+Reset) {
		t.Errorf("expected cyan code ANSI, got: %q", out)
	}
}

func TestRenderMarkdownHeader(t *testing.T) {
	out := RenderMarkdown("## Summary")
	if !strings.Contains(out, Bold+"SUMMARY"+Reset) {
		t.Errorf("expected bold uppercased header, got: %q", out)
	}
}

func TestRenderMarkdownBullet(t *testing.T) {
	out := RenderMarkdown("- First item\n* Second item")
	if !strings.Contains(out, "  \u2022") {
		t.Errorf("expected bullet character, got: %q", out)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), out)
	}
}

func TestRenderMarkdownNumberedList(t *testing.T) {
	out := RenderMarkdown("1. First\n2. Second")
	if !strings.Contains(out, "1. First") {
		t.Errorf("expected numbered list, got: %q", out)
	}
}

func TestRenderMarkdownRule(t *testing.T) {
	out := RenderMarkdown("---")
	if !strings.Contains(out, "───") {
		t.Errorf("expected horizontal rule, got: %q", out)
	}
}

func TestRenderMarkdownCodeBlockSkipped(t *testing.T) {
	input := "Before\n```\nsome code\n```\nAfter"
	out := RenderMarkdown(input)
	if strings.Contains(out, "some code") {
		t.Errorf("code block content should be skipped, got: %q", out)
	}
	if !strings.Contains(out, "Before") || !strings.Contains(out, "After") {
		t.Errorf("text outside code block should be preserved, got: %q", out)
	}
}

func TestRenderMarkdownEmptyLines(t *testing.T) {
	out := RenderMarkdown("Paragraph one.\n\nParagraph two.")
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines (with empty line), got %d: %q", len(lines), out)
	}
}

func TestRenderMarkdownMixed(t *testing.T) {
	input := `## Build Status

The build **1.0 (42)** is ready.

- Status: `+"`READY_FOR_REVIEW`"+`
- Testers: 3 invited

---

1. Submit for review
2. Wait for approval`

	out := RenderMarkdown(input)
	if !strings.Contains(out, Bold+"BUILD STATUS"+Reset) {
		t.Error("header not rendered")
	}
	if !strings.Contains(out, Bold+"1.0 (42)"+Reset) {
		t.Error("bold not rendered")
	}
	if !strings.Contains(out, Cyan+"READY_FOR_REVIEW"+Reset) {
		t.Error("inline code not rendered")
	}
	if !strings.Contains(out, "───") {
		t.Error("rule not rendered")
	}
}
