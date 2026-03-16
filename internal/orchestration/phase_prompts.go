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

// ComposeAgenticSystemPrompt assembles a single system prompt for agentic mode.
// Core rules are loaded natively by each runtime from disk (e.g. .claude/rules/ for Claude,
// codex.md for Codex, AGENTS.md for OpenCode). This prompt only adds role and context.
// Feature-specific skills are available on-demand via the nw_get_skills tool.
func ComposeAgenticSystemPrompt(ac ActionContext) string {
	platform := ac.Platform
	if platform == "" {
		platform = PlatformIOS
	}

	var b strings.Builder

	appendPromptSection(&b, "Role", `You are an autonomous Apple app builder. You have tools to set up workspaces, scaffold Xcode projects, build, verify, and finalize. You make all decisions yourself. Never ask clarifying questions.`)

	appendPromptSection(&b, "Coder", coderPromptForPlatform(platform))

	appendPromptSection(&b, "Skills", `Feature-specific skills (camera, authentication, supabase, charts, widgets, etc.) are available via the nw_get_skills tool. Call it with the relevant keys before implementing features you're unfamiliar with. Call nw_get_skills with list_available:true to discover all available skills.`)

	if ac.IsEdit() {
		editCtx := fmt.Sprintf("Operating on existing project:\n- Project dir: %s\n- App name: %s\n- Platform: %s", ac.ProjectDir, ac.AppName, ac.Platform)
		if len(ac.Platforms) > 1 {
			editCtx += fmt.Sprintf("\n- Platforms: %s", strings.Join(ac.Platforms, ", "))
		}
		appendPromptSection(&b, "Edit Context", editCtx)
	}

	return b.String()
}
