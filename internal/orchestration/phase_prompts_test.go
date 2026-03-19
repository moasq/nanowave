package orchestration

import (
	"strings"
	"testing"
)

func TestComposeIntegrationSectionNoIntegrations(t *testing.T) {
	section := composeIntegrationSection(nil)

	// Must mention both Supabase and RevenueCat as unconfigured
	if !strings.Contains(section, "Supabase") || !strings.Contains(section, "not configured") {
		t.Fatal("expected Supabase mentioned as not configured")
	}
	if !strings.Contains(section, "RevenueCat") || !strings.Contains(section, "not configured") {
		t.Fatal("expected RevenueCat mentioned as not configured")
	}
	// Should mention /revenuecat as the way to set them up
	if !strings.Contains(section, "/revenuecat") {
		t.Fatal("expected /revenuecat instruction")
	}
	if !strings.Contains(section, "no backend") {
		t.Fatal("expected no-backend notice when no integrations configured")
	}
}

func TestComposeIntegrationSectionSupabaseOnly(t *testing.T) {
	section := composeIntegrationSection([]string{"supabase"})

	// Supabase should be allowed
	if !strings.Contains(section, "Supabase") || !strings.Contains(section, "Configured") {
		t.Fatal("expected Supabase to be listed as configured")
	}
	// RevenueCat should be listed as unconfigured with setup instructions
	if !strings.Contains(section, "RevenueCat") || !strings.Contains(section, "not configured") {
		t.Fatal("expected RevenueCat listed as not configured")
	}
	// Should NOT say "no backend" since Supabase is active
	if strings.Contains(section, "no backend") {
		t.Fatal("should not say no backend when Supabase is configured")
	}
}

func TestComposeIntegrationSectionBothConfigured(t *testing.T) {
	section := composeIntegrationSection([]string{"supabase", "revenuecat"})

	// Both should be allowed
	if strings.Contains(section, "not configured") {
		t.Fatal("neither provider should be listed as not configured when both are configured")
	}
	if !strings.Contains(section, "Supabase") {
		t.Fatal("expected Supabase section")
	}
	if !strings.Contains(section, "RevenueCat") {
		t.Fatal("expected RevenueCat section")
	}
}

func TestComposeAgenticSystemPromptIncludesIntegrationSection(t *testing.T) {
	ac := ActionContext{
		ActiveIntegrations: []string{"supabase"},
	}
	prompt := ComposeAgenticSystemPrompt(ac, "/tmp/projects")

	if !strings.Contains(prompt, "Backend Integrations") {
		t.Fatal("expected Backend Integrations section in system prompt")
	}
	if !strings.Contains(prompt, "Supabase") {
		t.Fatal("expected Supabase to appear in system prompt")
	}
}

func TestComposeAgenticSystemPromptNoIntegrationsMentionsSetup(t *testing.T) {
	ac := ActionContext{}
	prompt := ComposeAgenticSystemPrompt(ac, "/tmp/projects")

	if !strings.Contains(prompt, "/revenuecat") {
		t.Fatal("expected /revenuecat in system prompt when no integrations")
	}
	if !strings.Contains(prompt, "/supabase") {
		t.Fatal("expected /supabase in system prompt when no integrations")
	}
	if !strings.Contains(prompt, "Run") {
		t.Fatal("expected setup instruction in system prompt")
	}
}
