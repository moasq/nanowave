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
func ComposeAgenticSystemPrompt(ac ActionContext, catalogRoot string) string {
	platform := ac.Platform
	if platform == "" {
		platform = PlatformIOS
	}

	var b strings.Builder

	appendPromptSection(&b, "Role", `You are an autonomous Apple app builder. You have tools to set up workspaces, scaffold Xcode projects, build, verify, and finalize. You make all decisions yourself. Never ask clarifying questions.`)

	appendPromptSection(&b, "Coder", coderPromptForPlatform(platform))

	appendPromptSection(&b, "Skills", `Feature-specific skills (camera, authentication, supabase, charts, widgets, etc.) are available via the nw_get_skills tool. Call it with the relevant keys before implementing features you're unfamiliar with. Call nw_get_skills with list_available:true to discover all available skills.`)

	appendPromptSection(&b, "Backend Integrations", composeIntegrationSection(ac.ActiveIntegrations))

	if ac.IsEdit() {
		editCtx := fmt.Sprintf("Operating on existing project:\n- Project dir: %s\n- App name: %s\n- Platform: %s", ac.ProjectDir, ac.AppName, ac.Platform)
		if len(ac.Platforms) > 1 {
			editCtx += fmt.Sprintf("\n- Platforms: %s", strings.Join(ac.Platforms, ", "))
		}
		appendPromptSection(&b, "Edit Context", editCtx)
	} else if catalogRoot != "" {
		appendPromptSection(&b, "Project Location", fmt.Sprintf(
			"CRITICAL: Create the project directory inside `%s`. For example, if the app is called MyApp, create it at `%s/MyApp/`. Do NOT create projects anywhere else.",
			catalogRoot, catalogRoot))
	}

	return b.String()
}

// composeIntegrationSection generates the Backend Integrations prompt section.
func composeIntegrationSection(activeIntegrations []string) string {
	active := make(map[string]bool, len(activeIntegrations))
	for _, id := range activeIntegrations {
		active[id] = true
	}

	var b strings.Builder

	// Describe what IS available
	if len(activeIntegrations) > 0 {
		b.WriteString("The following backend integrations are configured and available for this project:\n")
		for _, id := range activeIntegrations {
			switch id {
			case "supabase":
				b.WriteString("- **Supabase**: Configured. You MAY use Supabase for authentication, database, storage, and realtime. Use the `nw_get_skills` tool with key `repositories` to learn the repository pattern. MCP tools for Supabase are available.\n")
			case "revenuecat":
				b.WriteString("- **RevenueCat**: Configured. You MAY use RevenueCat for in-app purchases and subscriptions. Use the `nw_get_skills` tool with key `paywall` to learn the paywall pattern. MCP tools for RevenueCat are available.\n")
			default:
				b.WriteString(fmt.Sprintf("- **%s**: Configured and available.\n", id))
			}
		}
	}

	// Describe what is NOT yet configured
	var unconfigured []string
	if !active["supabase"] {
		unconfigured = append(unconfigured, "supabase")
	}
	if !active["revenuecat"] {
		unconfigured = append(unconfigured, "revenuecat")
	}

	if len(unconfigured) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("**The following backends are NOT yet configured:**\n")
		for _, id := range unconfigured {
			b.WriteString(fmt.Sprintf("- **%s** is not configured.\n", integrationDisplayName(id)))
		}
		b.WriteString("\nWhen the user asks for features that require an unconfigured backend (authentication, database, subscriptions, in-app purchases, paywalls, etc.), you MUST:\n")
		b.WriteString("1. Explain what you'll build and which backend it needs\n")
		b.WriteString("2. End your response by telling the user to run the setup command:\n")
		for _, id := range unconfigured {
			name := integrationDisplayName(id)
			b.WriteString(fmt.Sprintf("   - For %s: \"Run `/%s` to connect your %s account, then I'll wire everything up.\"\n", name, id, name))
		}
		b.WriteString("3. Do NOT generate any code, imports, or SPM packages for the unconfigured backend. Wait until the user confirms setup is complete.\n")
		if !active["supabase"] && !active["revenuecat"] {
			b.WriteString("\nCurrently this app has no backend. If the user does not request backend features, store all data on-device using SwiftData or UserDefaults.")
		}
	}

	return b.String()
}

func integrationDisplayName(id string) string {
	switch id {
	case "supabase":
		return "Supabase"
	case "revenuecat":
		return "RevenueCat"
	default:
		return id
	}
}
