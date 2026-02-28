package orchestration

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moasq/nanowave/internal/claude"
	"github.com/moasq/nanowave/internal/integrations"
	"github.com/moasq/nanowave/internal/terminal"
)

// inlineSetupChoice represents the user's choice when prompted to set up an integration during build.
type inlineSetupChoice int

const (
	setupChoiceSkip   inlineSetupChoice = iota // Continue without backend
	setupChoiceAuto                            // Automatic setup via CLI
	setupChoiceManual                          // Enter credentials manually
)

// askSetupConfirm prompts the user to set up an integration inline during build.
// Returns setupChoiceSkip if the user cancels (Esc) or picks Skip.
func askSetupConfirm(providerID string) inlineSetupChoice {
	integ := integrations.LookupIntegration(integrations.ProviderID(providerID))
	name := providerID
	if integ != nil {
		name = integ.Name
	}

	var options []terminal.PickerOption
	switch integrations.ProviderID(providerID) {
	case integrations.ProviderRevenueCat:
		options = []terminal.PickerOption{
			{Label: "Guided", Desc: "Enter API key → pick project & app from your account"},
			{Label: "Manual", Desc: "Enter project ID, app ID, and API keys directly"},
			{Label: "Skip", Desc: "Continue without monetization — use placeholder keys"},
		}
	default:
		options = []terminal.PickerOption{
			{Label: "Automatic", Desc: "Connect via " + name + " CLI (opens browser, ~30 seconds)"},
			{Label: "Manual", Desc: "Enter project URL and anon key manually"},
			{Label: "Skip", Desc: "Continue without backend — use placeholder credentials"},
		}
	}

	picked := terminal.Pick(fmt.Sprintf("Set up %s now?", name), options, "")
	switch picked {
	case "Automatic", "Guided":
		return setupChoiceAuto
	case "Manual":
		return setupChoiceManual
	default:
		return setupChoiceSkip
	}
}

// pipelinePrintFn bridges integrations output to terminal for inline setup.
func pipelinePrintFn(level, msg string) {
	switch level {
	case "success":
		terminal.Success(msg)
	case "warning":
		terminal.Warning(msg)
	case "error":
		terminal.Error(msg)
	case "info":
		terminal.Info(msg)
	case "header":
		terminal.Header(msg)
	case "detail":
		fmt.Printf("    %s%s%s\n", terminal.Dim, msg, terminal.Reset)
	}
}

// pipelinePickFn bridges integrations pick to terminal.Pick for inline setup.
func pipelinePickFn(title string, options []string) string {
	pickerOpts := make([]terminal.PickerOption, len(options))
	for i, opt := range options {
		pickerOpts[i] = terminal.PickerOption{Label: opt}
	}
	return terminal.Pick(title, pickerOpts, "")
}

// pipelineReadLineFn reads a line of input with a label prompt.
func pipelineReadLineFn(label string) string {
	fmt.Printf("  %s: ", label)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

// finalize ensures xcodegen has run, then does git init + commit.
func (p *Pipeline) finalize(ctx context.Context, projectDir, appName string) {
	// Safety net: re-run xcodegen if .xcodeproj is missing (shouldn't happen since we run it in scaffold phase)
	xcodeprojPath := filepath.Join(projectDir, appName+".xcodeproj")
	if _, err := os.Stat(xcodeprojPath); os.IsNotExist(err) {
		_ = runXcodeGen(projectDir)
	}

	// Git operations are best-effort
	resp, _ := p.claude.Generate(ctx, fmt.Sprintf(`Run these git commands in order:
1. git init
2. git add -A
3. git commit -m "Initial build: %s"

Just run the commands, no explanation needed.`, appName), claude.GenerateOpts{
		MaxTurns:     3,
		Model:        "haiku",
		WorkDir:      projectDir,
		AllowedTools: []string{"Bash"},
	})
	_ = resp
}

// modelsToModelRefs converts orchestration ModelPlan slice to integrations ModelRef slice.
// This is the bridge between packages (like sql.DB converting Go types to driver.Value).
func modelsToModelRefs(models []ModelPlan) []integrations.ModelRef {
	refs := make([]integrations.ModelRef, len(models))
	for i, m := range models {
		props := make([]integrations.PropertyRef, len(m.Properties))
		for j, p := range m.Properties {
			props[j] = integrations.PropertyRef{
				Name:         p.Name,
				Type:         p.Type,
				DefaultValue: p.DefaultValue,
			}
		}
		refs[i] = integrations.ModelRef{
			Name:       m.Name,
			Storage:    m.Storage,
			Properties: props,
		}
	}
	return refs
}

// writeMCPConfig writes .mcp.json using Manager-provided MCPServerConfig entries.
func writeMCPConfig(projectDir string, configs []integrations.MCPServerConfig) error {
	type mcpServerEntry struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env,omitempty"`
	}

	servers := map[string]mcpServerEntry{
		"apple-docs": {
			Command: "npx",
			Args:    []string{"-y", "@kimsungwhee/apple-docs-mcp"},
		},
		"xcodegen": {
			Command: "nanowave",
			Args:    []string{"mcp", "xcodegen"},
		},
	}

	for _, cfg := range configs {
		servers[cfg.Name] = mcpServerEntry{
			Command: cfg.Command,
			Args:    cfg.Args,
			Env:     cfg.Env,
		}
	}

	wrapper := struct {
		MCPServers map[string]mcpServerEntry `json:"mcpServers"`
	}{MCPServers: servers}

	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(projectDir, ".mcp.json"), data, 0o600)
}

// writeSettingsShared writes team-shared Claude Code settings with Manager-provided MCP tools.
func writeSettingsShared(projectDir string, mcpTools []string) error {
	allow := []string{
		"mcp__apple-docs__search_apple_docs",
		"mcp__apple-docs__get_apple_doc_content",
		"mcp__apple-docs__search_framework_symbols",
		"mcp__apple-docs__get_sample_code",
		"mcp__apple-docs__get_related_apis",
		"mcp__apple-docs__find_similar_apis",
		"mcp__apple-docs__get_platform_compatibility",
		"mcp__xcodegen__add_permission",
		"mcp__xcodegen__add_extension",
		"mcp__xcodegen__add_entitlement",
		"mcp__xcodegen__add_localization",
		"mcp__xcodegen__set_build_setting",
		"mcp__xcodegen__get_project_config",
		"mcp__xcodegen__regenerate_project",
		"SlashCommand",
		"Task",
		"ViewImage",
		"WebFetch",
		"WebSearch",
	}
	allow = append(allow, mcpTools...)

	// Delegate to the existing writeSettingsSharedWithTools
	return writeSettingsSharedWithTools(projectDir, allow)
}

// writeSettingsSharedWithTools writes settings.json with the given tool allowlist.
// The template matches writeSettingsShared() in setup.go exactly.
func writeSettingsSharedWithTools(projectDir string, allow []string) error {
	var allowJSON strings.Builder
	for i, tool := range allow {
		if i > 0 {
			allowJSON.WriteString(",\n")
		}
		allowJSON.WriteString(fmt.Sprintf("      %q", tool))
	}

	settings := fmt.Sprintf(`{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "permissions": {
    "allow": [
%s
    ],
    "deny": [
      "Read(./.env)",
      "Read(./.env.*)",
      "Read(./secrets/**)"
    ]
  },
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": "./scripts/claude/check-project-config-edits.sh"
          }
        ]
      },
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "./scripts/claude/check-bash-safety.sh"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": "./scripts/claude/check-swift-structure.sh"
          },
          {
            "type": "command",
            "command": "./scripts/claude/check-no-placeholders.sh --hook"
          },
          {
            "type": "command",
            "command": "./scripts/claude/check-previews.sh --hook"
          },
          {
            "type": "command",
            "command": "./scripts/claude/check-a11y-dynamic-type.sh --hook"
          },
          {
            "type": "command",
            "command": "./scripts/claude/check-a11y-icon-buttons.sh --hook"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "./scripts/claude/run-build-check.sh --hook"
          }
        ]
      }
    ]
  }
}
`, allowJSON.String())
	claudeDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settings), 0o644)
}

// pipelineSetupUI implements integrations.SetupUI for the build pipeline.
type pipelineSetupUI struct{}

func (u *pipelineSetupUI) PromptSetup(ctx context.Context, sc integrations.SetupCapable, p integrations.Provider, store *integrations.IntegrationStore, appName string) *integrations.IntegrationConfig {
	id := string(p.ID())

	// Try auto-setup if CLI available
	if sc.CLIAvailable() {
		terminal.Info(fmt.Sprintf("Setting up %s for %s...", p.Meta().Name, appName))
		err := sc.Setup(ctx, integrations.SetupRequest{
			Store:      store,
			AppName:    appName,
			ReadLineFn: pipelineReadLineFn,
			PrintFn:    pipelinePrintFn,
			PickFn:     pipelinePickFn,
		})
		if err != nil {
			terminal.Error(fmt.Sprintf("Auto-setup failed: %v", err))
			return u.promptManualSetup(ctx, sc, p, store, appName)
		}
		cfg, _ := store.GetProvider(p.ID(), appName)
		return cfg
	}

	// CLI not available — prompt
	terminal.Warning(fmt.Sprintf("%s integration needed — let's set it up", id))
	return u.promptManualSetup(ctx, sc, p, store, appName)
}

func (u *pipelineSetupUI) promptManualSetup(ctx context.Context, sc integrations.SetupCapable, p integrations.Provider, store *integrations.IntegrationStore, appName string) *integrations.IntegrationConfig {
	for {
		choice := askSetupConfirm(string(p.ID()))
		switch choice {
		case setupChoiceAuto:
			err := sc.Setup(ctx, integrations.SetupRequest{
				Store:      store,
				AppName:    appName,
				ReadLineFn: pipelineReadLineFn,
				PrintFn:    pipelinePrintFn,
				PickFn:     pipelinePickFn,
			})
			if err != nil {
				terminal.Error(fmt.Sprintf("Setup failed: %v", err))
				continue
			}
			cfg, _ := store.GetProvider(p.ID(), appName)
			return cfg
		case setupChoiceManual:
			err := sc.Setup(ctx, integrations.SetupRequest{
				Store:      store,
				AppName:    appName,
				Manual:     true,
				ReadLineFn: pipelineReadLineFn,
				PrintFn:    pipelinePrintFn,
			})
			if err != nil {
				terminal.Error(fmt.Sprintf("Setup failed: %v", err))
				continue
			}
			cfg, _ := store.GetProvider(p.ID(), appName)
			return cfg
		default:
			return nil
		}
	}
}

func (u *pipelineSetupUI) ValidateExisting(ctx context.Context, sc integrations.SetupCapable, p integrations.Provider, store *integrations.IntegrationStore, appName string, cfg *integrations.IntegrationConfig) *integrations.IntegrationConfig {
	projectLabel := cfg.ProjectURL
	if projectLabel == "" {
		projectLabel = cfg.ProjectRef
	}

	// RevenueCat: each new build should confirm credentials since the user may
	// want a different RC project/app. Other providers (Supabase) reuse silently.
	if p.ID() == integrations.ProviderRevenueCat {
		options := []terminal.PickerOption{
			{Label: "Reuse", Desc: fmt.Sprintf("Use existing credentials (project: %s)", projectLabel)},
			{Label: "New", Desc: "Set up with a different RevenueCat project/key"},
		}
		picked := terminal.Pick("Found existing RevenueCat config", options, "")
		if picked == "New" {
			_ = store.RemoveProvider(p.ID(), appName)
			return u.promptManualSetup(ctx, sc, p, store, appName)
		}
	}

	terminal.Success(fmt.Sprintf("%s connected (project: %s)", p.Meta().Name, projectLabel))
	terminal.Detail("Config details", fmt.Sprintf("URL=%s, has_anon_key=%t, has_PAT=%t",
		cfg.ProjectURL, cfg.AnonKey != "", cfg.PAT != ""))

	if cfg.PAT == "" {
		terminal.Warning(fmt.Sprintf("%s PAT is missing — MCP tools will not work. Re-running setup...", p.Meta().Name))
		if sc.CLIAvailable() {
			err := sc.Setup(context.Background(), integrations.SetupRequest{
				Store:   store,
				AppName: appName,
				PrintFn: pipelinePrintFn,
				PickFn:  pipelinePickFn,
			})
			if err == nil {
				updated, _ := store.GetProvider(p.ID(), appName)
				return updated
			}
			terminal.Error(fmt.Sprintf("Setup failed: %v", err))
		}
	}
	return cfg
}

func (u *pipelineSetupUI) Info(msg string)    { terminal.Info(msg) }
func (u *pipelineSetupUI) Warning(msg string) { terminal.Warning(msg) }

// monetizationPlanToRef converts orchestration.MonetizationPlan to integrations.MonetizationPlan.
// Same bridge pattern as modelsToModelRefs.
func monetizationPlanToRef(plan *MonetizationPlan) *integrations.MonetizationPlan {
	if plan == nil {
		return nil
	}
	products := make([]integrations.MonetizationProduct, len(plan.Products))
	for i, p := range plan.Products {
		products[i] = integrations.MonetizationProduct{
			Identifier:  p.Identifier,
			Type:        p.Type,
			DisplayName: p.DisplayName,
			Price:       p.Price,
			Credits:     p.Credits,
			Duration:    p.Duration,
		}
	}
	return &integrations.MonetizationPlan{
		Model:       plan.Model,
		Products:    products,
		Entitlement: plan.Entitlement,
		FreeCredits: plan.FreeCredits,
	}
}

// writeStoreKitConfig generates a .storekit configuration file for local testing.
func writeStoreKitConfig(projectDir, appName string, plan *MonetizationPlan) error {
	type storeKitProduct struct {
		DisplayName           string `json:"displayName"`
		Identifier            string `json:"identifier"`
		ProductCategory       string `json:"productCategory"`
		ReferenceName         string `json:"referenceName"`
		Type                  string `json:"type"`
	}

	type storeKitSubscription struct {
		DisplayName     string `json:"displayName"`
		Identifier      string `json:"identifier"`
		ProductCategory string `json:"productCategory"`
		ReferenceName   string `json:"referenceName"`
		Type            string `json:"type"`
		SubscriptionPeriod string `json:"subscriptionPeriod,omitempty"`
	}

	type storeKitSubscriptionGroup struct {
		ID            string                 `json:"id"`
		Name          string                 `json:"name"`
		Subscriptions []storeKitSubscription `json:"subscriptions"`
	}

	type storeKitConfig struct {
		Identifier           string                      `json:"identifier"`
		NonRenewingSubscriptions []any                   `json:"nonRenewingSubscriptions"`
		Products             []storeKitProduct           `json:"products"`
		Settings             map[string]any              `json:"settings"`
		SubscriptionGroups   []storeKitSubscriptionGroup `json:"subscriptionGroups"`
		Version              map[string]int              `json:"version"`
	}

	var products []storeKitProduct
	var subscriptions []storeKitSubscription

	for _, p := range plan.Products {
		if p.Type == "consumable" {
			products = append(products, storeKitProduct{
				DisplayName:     p.DisplayName,
				Identifier:      p.Identifier,
				ProductCategory: "nonConsumable",
				ReferenceName:   p.DisplayName,
				Type:            "Consumable",
			})
		} else {
			period := "P1M"
			if p.Duration != "" {
				period = p.Duration
			}
			subscriptions = append(subscriptions, storeKitSubscription{
				DisplayName:        p.DisplayName,
				Identifier:         p.Identifier,
				ProductCategory:    "subscription",
				ReferenceName:      p.DisplayName,
				Type:               "RecurringSubscription",
				SubscriptionPeriod: period,
			})
		}
	}

	config := storeKitConfig{
		Identifier:               fmt.Sprintf("%s.storekit", appName),
		NonRenewingSubscriptions: []any{},
		Products:                 products,
		Settings: map[string]any{
			"_applicationInternalID": "",
			"_developerTeamID":       "",
		},
		Version: map[string]int{"major": 3, "minor": 0},
	}

	if len(subscriptions) > 0 {
		config.SubscriptionGroups = []storeKitSubscriptionGroup{
			{
				ID:            "group_default",
				Name:          "default",
				Subscriptions: subscriptions,
			},
		}
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	// Place inside the app source directory so XcodeGen's syncedFolder picks it up automatically.
	appDir := filepath.Join(projectDir, appName)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return fmt.Errorf("create app directory: %w", err)
	}
	filename := fmt.Sprintf("%s.storekit", appName)
	return os.WriteFile(filepath.Join(appDir, filename), data, 0o644)
}
