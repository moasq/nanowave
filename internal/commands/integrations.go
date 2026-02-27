package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moasq/nanowave/internal/config"
	"github.com/moasq/nanowave/internal/integrations"
	"github.com/moasq/nanowave/internal/terminal"
	"github.com/spf13/cobra"
)

var integrationsCmd = &cobra.Command{
	Use:   "integrations",
	Short: "Manage backend integrations (Supabase, etc.)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return integrationsListRun()
	},
}

var integrationsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show available integrations and their status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return integrationsListRun()
	},
}

var integrationsSetupCmd = &cobra.Command{
	Use:   "setup [provider]",
	Short: "Set up a backend integration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return integrationsSetupRun(args[0])
	},
}

var integrationsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show configured integration details",
	RunE: func(cmd *cobra.Command, args []string) error {
		return integrationsStatusRun()
	},
}

var integrationsRemoveCmd = &cobra.Command{
	Use:   "remove [provider]",
	Short: "Remove an integration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return integrationsRemoveRun(args[0])
	},
}

func init() {
	integrationsCmd.AddCommand(integrationsListCmd)
	integrationsCmd.AddCommand(integrationsSetupCmd)
	integrationsCmd.AddCommand(integrationsStatusCmd)
	integrationsCmd.AddCommand(integrationsRemoveCmd)
}

func nanowaveRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".nanowave")
}

func loadIntegrationStore() *integrations.IntegrationStore {
	store := integrations.NewIntegrationStore(nanowaveRoot())
	_ = store.Load()
	return store
}

func integrationsListRun() error {
	fmt.Println()
	terminal.Header("Integrations")
	store := loadIntegrationStore()
	statuses := store.AllStatuses()

	// Group by provider — show configured count
	seen := make(map[integrations.ProviderID]bool)
	for _, s := range statuses {
		if seen[s.Provider] {
			continue
		}
		seen[s.Provider] = true

		integ := integrations.LookupIntegration(s.Provider)
		if integ == nil {
			continue
		}

		// Count configured apps for this provider
		configuredCount := 0
		for _, s2 := range statuses {
			if s2.Provider == s.Provider && s2.Configured {
				configuredCount++
			}
		}

		status := fmt.Sprintf("%s✗ Not configured%s", terminal.Dim, terminal.Reset)
		if configuredCount > 0 {
			status = fmt.Sprintf("%s✓ %d app(s) configured%s", terminal.Green, configuredCount, terminal.Reset)
		}
		fmt.Printf("  %s%-15s%s %s  %s%s%s\n", terminal.Bold, integ.Name, terminal.Reset, status, terminal.Dim, integ.Description, terminal.Reset)
	}

	// Show coming-soon providers
	fmt.Printf("  %s%-15s%s %sComing soon%s\n", terminal.Bold, "Firebase", terminal.Reset, terminal.Dim, terminal.Reset)
	fmt.Println()
	return nil
}

func integrationsSetupRun(provider string) error {
	id := integrations.ProviderID(provider)
	switch id {
	case integrations.ProviderSupabase:
		if !config.CheckSupabaseCLI() {
			terminal.Warning("Supabase CLI not found. Install it with: brew install supabase/tap/supabase")
			return fmt.Errorf("supabase CLI not installed")
		}
		store := loadIntegrationStore()
		// When run from CLI directly, use a generic project name
		return integrations.SetupSupabase(store, "my-app", terminalPrintFn, terminalPickFn)
	default:
		return fmt.Errorf("unknown provider: %s", provider)
	}
}

func integrationsStatusRun() error {
	fmt.Println()
	terminal.Header("Integration Status")
	store := loadIntegrationStore()
	statuses := store.AllStatuses()

	anyConfigured := false
	for _, s := range statuses {
		if !s.Configured {
			continue
		}
		anyConfigured = true
		integ := integrations.LookupIntegration(s.Provider)
		if integ == nil {
			continue
		}
		appLabel := s.AppName
		if appLabel == "" || appLabel == integrations.DefaultAppKey() {
			appLabel = "(default)"
		}
		terminal.Detail("Provider", fmt.Sprintf("%s [%s]", integ.Name, appLabel))
		if s.ProjectURL != "" {
			terminal.Detail("URL", s.ProjectURL)
		}
		mark := func(ok bool) string {
			if ok {
				return terminal.Green + "✓" + terminal.Reset
			}
			return terminal.Red + "✗" + terminal.Reset
		}
		terminal.Detail("Anon Key", mark(s.HasAnonKey))
		terminal.Detail("PAT (MCP)", mark(s.HasPAT))
		fmt.Println()
	}

	if !anyConfigured {
		terminal.Info("No integrations configured. Run: nanowave integrations setup supabase")
		fmt.Println()
	}
	return nil
}

func integrationsRemoveRun(provider string) error {
	id := integrations.ProviderID(provider)
	switch id {
	case integrations.ProviderSupabase:
		store := loadIntegrationStore()
		appNames := store.AllAppNames(integrations.ProviderSupabase)
		if len(appNames) == 0 {
			terminal.Info("No Supabase integrations configured")
			return nil
		}

		// Let user pick which app's integration to remove
		var options []terminal.PickerOption
		for _, name := range appNames {
			label := name
			if label == "" || label == integrations.DefaultAppKey() {
				label = "(default)"
			}
			options = append(options, terminal.PickerOption{Label: label})
		}
		options = append(options, terminal.PickerOption{Label: "All", Desc: "Remove all Supabase integrations and cached credentials"})

		picked := terminal.Pick("Remove Supabase for which app?", options, "")
		if picked == "" {
			return nil
		}

		if picked == "All" {
			for _, name := range appNames {
				_ = integrations.RevokeSupabase(store, name)
			}
			terminal.Success("All Supabase integrations and cached credentials removed")
		} else {
			// Map "(default)" back to the actual key
			appName := picked
			if picked == "(default)" {
				appName = integrations.DefaultAppKey()
			}
			if err := integrations.RevokeSupabase(store, appName); err != nil {
				return err
			}
			terminal.Success(fmt.Sprintf("Supabase integration removed for %s", picked))
		}
		fmt.Println()
		return nil
	default:
		return fmt.Errorf("unknown provider: %s", provider)
	}
}

// terminalPrintFn bridges integrations print calls to terminal UI.
func terminalPrintFn(level, msg string) {
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
		// For detail, msg is the value; we don't have a label here
		fmt.Printf("    %s%s%s\n", terminal.Dim, msg, terminal.Reset)
	default:
		fmt.Println(msg)
	}
}

// terminalPickFn bridges integrations pick calls to terminal.Pick.
func terminalPickFn(title string, options []string) string {
	pickerOpts := make([]terminal.PickerOption, len(options))
	for i, opt := range options {
		pickerOpts[i] = terminal.PickerOption{Label: opt}
	}
	return terminal.Pick(title, pickerOpts, "")
}


// RunIntegrationsInteractive provides the /integrations slash command flow.
func RunIntegrationsInteractive() {
	store := loadIntegrationStore()
	statuses := store.AllStatuses()

	fmt.Println()
	terminal.Header("Integrations")

	// Build options for picker
	var options []terminal.PickerOption
	seen := make(map[integrations.ProviderID]bool)
	for _, s := range statuses {
		if seen[s.Provider] {
			continue
		}
		seen[s.Provider] = true

		integ := integrations.LookupIntegration(s.Provider)
		if integ == nil {
			continue
		}

		// Count configured apps
		configuredCount := 0
		for _, s2 := range statuses {
			if s2.Provider == s.Provider && s2.Configured {
				configuredCount++
			}
		}

		status := "Not configured"
		if configuredCount > 0 {
			status = fmt.Sprintf("✓ %d app(s) configured", configuredCount)
		}
		options = append(options, terminal.PickerOption{
			Label: integ.Name,
			Desc:  status,
		})
	}
	options = append(options, terminal.PickerOption{Label: "Firebase", Desc: "Coming soon"})

	picked := terminal.Pick("Integrations", options, "")
	if picked == "" || picked == "Firebase" {
		fmt.Println()
		return
	}

	// Map picked name back to provider
	switch picked {
	case "Supabase":
		appNames := store.AllAppNames(integrations.ProviderSupabase)
		if len(appNames) > 0 {
			// Already configured — show statuses + offer actions
			fmt.Println()
			for _, appName := range appNames {
				cfg, _ := store.GetProvider(integrations.ProviderSupabase, appName)
				if cfg == nil {
					continue
				}
				appLabel := appName
				if appLabel == "" || appLabel == integrations.DefaultAppKey() {
					appLabel = "(default)"
				}
				terminal.Detail("App", appLabel)
				terminal.Detail("URL", cfg.ProjectURL)
				mark := func(ok bool) string {
					if ok {
						return terminal.Green + "✓" + terminal.Reset
					}
					return terminal.Red + "✗" + terminal.Reset
				}
				terminal.Detail("Anon Key", mark(cfg.AnonKey != ""))
				terminal.Detail("PAT (MCP)", mark(cfg.PAT != ""))
				fmt.Println()
			}

			action := terminal.Pick("Action", []terminal.PickerOption{
				{Label: "Keep", Desc: "No changes"},
				{Label: "Add new", Desc: "Set up Supabase for another app"},
				{Label: "Remove", Desc: "Remove a Supabase integration"},
			}, "")

			switch action {
			case "Remove":
				_ = integrationsRemoveRun("supabase")
			case "Add new":
				if config.CheckSupabaseCLI() {
					_ = integrations.SetupSupabase(store, "my-app", terminalPrintFn, terminalPickFn)
				} else {
					terminal.Warning("Supabase CLI not found. Install: brew install supabase/tap/supabase")
				}
			}
		} else {
			// Not configured — show options before running setup
			action := terminal.Pick("Supabase Setup", []terminal.PickerOption{
				{Label: "Set up automatically", Desc: "Login + create project via Supabase CLI"},
				{Label: "Enter credentials manually", Desc: "Paste project URL and anon key"},
				{Label: "Cancel"},
			}, "")

			switch action {
			case "Set up automatically":
				if config.CheckSupabaseCLI() {
					_ = integrations.SetupSupabase(store, "my-app", terminalPrintFn, terminalPickFn)
				} else {
					terminal.Warning("Supabase CLI not found. Install: brew install supabase/tap/supabase")
				}
			case "Enter credentials manually":
				readLineFn := func(label string) string {
					fmt.Printf("  %s: ", label)
					reader := bufio.NewReader(os.Stdin)
					line, _ := reader.ReadString('\n')
					return strings.TrimSpace(line)
				}
				_ = integrations.SetupSupabaseManual(store, "my-app", readLineFn, terminalPrintFn)
			}
		}
	}
	fmt.Println()
}
