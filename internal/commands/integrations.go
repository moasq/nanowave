package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moasq/nanowave/internal/integrations"
	"github.com/moasq/nanowave/internal/integrations/providers"
	"github.com/moasq/nanowave/internal/terminal"
	"github.com/spf13/cobra"
)

// newCmdManager creates a Manager with all registered providers.
func newCmdManager() *integrations.Manager {
	r := integrations.NewRegistry()
	providers.RegisterAll(r)
	return integrations.NewManager(r, loadIntegrationStore())
}

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
	m := newCmdManager()
	p, ok := m.GetProvider(integrations.ProviderID(provider))
	if !ok {
		return fmt.Errorf("unknown provider: %s", provider)
	}
	sc, ok := p.(integrations.SetupCapable)
	if !ok {
		return fmt.Errorf("provider %s does not support setup", provider)
	}
	if !sc.CLIAvailable() {
		terminal.Warning(fmt.Sprintf("%s CLI not found. Install it with the provider's install instructions.", p.Meta().Name))
		return fmt.Errorf("%s CLI not installed", provider)
	}
	return sc.Setup(context.Background(), integrations.SetupRequest{
		Store:   m.Store(),
		AppName: "my-app",
		PrintFn: terminalPrintFn,
		PickFn:  terminalPickFn,
	})
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
	m := newCmdManager()
	pid := integrations.ProviderID(provider)
	p, ok := m.GetProvider(pid)
	if !ok {
		return fmt.Errorf("unknown provider: %s", provider)
	}
	sc, ok := p.(integrations.SetupCapable)
	if !ok {
		return fmt.Errorf("provider %s does not support removal", provider)
	}
	store := m.Store()
	appNames := store.AllAppNames(pid)
	if len(appNames) == 0 {
		terminal.Info(fmt.Sprintf("No %s integrations configured", p.Meta().Name))
		return nil
	}
	var options []terminal.PickerOption
	for _, name := range appNames {
		label := name
		if label == "" || label == integrations.DefaultAppKey() {
			label = "(default)"
		}
		options = append(options, terminal.PickerOption{Label: label})
	}
	options = append(options, terminal.PickerOption{Label: "All", Desc: fmt.Sprintf("Remove all %s integrations and cached credentials", p.Meta().Name)})
	picked := terminal.Pick(fmt.Sprintf("Remove %s for which app?", p.Meta().Name), options, "")
	if picked == "" {
		return nil
	}
	if picked == "All" {
		for _, name := range appNames {
			_ = sc.Remove(context.Background(), store, name)
		}
		terminal.Success(fmt.Sprintf("All %s integrations and cached credentials removed", p.Meta().Name))
	} else {
		appName := picked
		if picked == "(default)" {
			appName = integrations.DefaultAppKey()
		}
		if err := sc.Remove(context.Background(), store, appName); err != nil {
			return err
		}
		terminal.Success(fmt.Sprintf("%s integration removed for %s", p.Meta().Name, picked))
	}
	fmt.Println()
	return nil
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
	m := newCmdManager()
	store := m.Store()

	fmt.Println()
	terminal.Header("Integrations")

	// Build options from all registered providers
	var options []terminal.PickerOption
	for _, p := range m.AllProviders() {
		appNames := store.AllAppNames(p.ID())
		status := "Not configured"
		if len(appNames) > 0 {
			status = fmt.Sprintf("✓ %d app(s) configured", len(appNames))
		}
		options = append(options, terminal.PickerOption{
			Label: p.Meta().Name,
			Desc:  status,
		})
	}
	options = append(options, terminal.PickerOption{Label: "Firebase", Desc: "Coming soon"})

	picked := terminal.Pick("Integrations", options, "")
	if picked == "" || picked == "Firebase" {
		fmt.Println()
		return
	}

	// Find the provider by name
	var selectedProvider integrations.Provider
	for _, p := range m.AllProviders() {
		if p.Meta().Name == picked {
			selectedProvider = p
			break
		}
	}
	if selectedProvider == nil {
		fmt.Println()
		return
	}

	sc, isSetupCapable := selectedProvider.(integrations.SetupCapable)
	if !isSetupCapable {
		terminal.Warning(fmt.Sprintf("%s does not support setup", picked))
		fmt.Println()
		return
	}

	pid := selectedProvider.ID()
	appNames := store.AllAppNames(pid)

	if len(appNames) > 0 {
		// Already configured — show statuses + offer actions
		fmt.Println()
		for _, appName := range appNames {
			cfg, _ := store.GetProvider(pid, appName)
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
			{Label: "Add new", Desc: fmt.Sprintf("Set up %s for another app", picked)},
			{Label: "Remove", Desc: fmt.Sprintf("Remove a %s integration", picked)},
		}, "")

		switch action {
		case "Remove":
			_ = integrationsRemoveRun(string(pid))
		case "Add new":
			if sc.CLIAvailable() {
				_ = sc.Setup(context.Background(), integrations.SetupRequest{
					Store:   store,
					AppName: "my-app",
					PrintFn: terminalPrintFn,
					PickFn:  terminalPickFn,
				})
			} else {
				terminal.Warning(fmt.Sprintf("%s CLI not found.", picked))
			}
		}
	} else {
		// Not configured — show setup options
		action := terminal.Pick(fmt.Sprintf("%s Setup", picked), []terminal.PickerOption{
			{Label: "Set up automatically", Desc: fmt.Sprintf("Login + create project via %s CLI", picked)},
			{Label: "Enter credentials manually", Desc: "Paste project URL and anon key"},
			{Label: "Cancel"},
		}, "")

		switch action {
		case "Set up automatically":
			if sc.CLIAvailable() {
				_ = sc.Setup(context.Background(), integrations.SetupRequest{
					Store:   store,
					AppName: "my-app",
					PrintFn: terminalPrintFn,
					PickFn:  terminalPickFn,
				})
			} else {
				terminal.Warning(fmt.Sprintf("%s CLI not found.", picked))
			}
		case "Enter credentials manually":
			readLineFn := func(label string) string {
				fmt.Printf("  %s: ", label)
				reader := bufio.NewReader(os.Stdin)
				line, _ := reader.ReadString('\n')
				return strings.TrimSpace(line)
			}
			_ = sc.Setup(context.Background(), integrations.SetupRequest{
				Store:      store,
				AppName:    "my-app",
				Manual:     true,
				ReadLineFn: readLineFn,
				PrintFn:    terminalPrintFn,
			})
		}
	}
	fmt.Println()
}
