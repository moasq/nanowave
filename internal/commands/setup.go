package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/moasq/nanowave/internal/agentruntime"
	"github.com/moasq/nanowave/internal/config"
	"github.com/moasq/nanowave/internal/service"
	"github.com/moasq/nanowave/internal/terminal"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install and verify prerequisites",
	Long:  "Check and install all prerequisites needed to use Nanowave.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSetup()
	},
}

// needsSetup returns true if any critical dependency is missing.
func needsSetup() bool {
	cfg, err := config.Load()
	if err != nil {
		return true
	}
	return needsSetupForRuntime(cfg.RuntimeKind)
}

func needsSetupForRuntime(kind agentruntime.Kind) bool {
	return !config.CheckRuntime(kind) || !config.CheckXcodegen() || !config.CheckXcode() || !config.CheckSimulator()
}

// runSetup checks and installs all prerequisites. Returns nil on success.
func runSetup() error {
	terminal.Header("Nanowave Setup")
	fmt.Println()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	reader := bufio.NewReader(os.Stdin)

	// ── 1. Select AI agent ─────────────────────────────────────
	runtimeKind := cfg.RuntimeKind
	if AgentFlag() != "" {
		runtimeKind = agentruntime.NormalizeKind(AgentFlag())
	} else {
		allStatuses := service.SupportedRuntimeStatuses()
		agentOptions := make([]terminal.PickerOption, 0, len(allStatuses))
		for _, d := range allStatuses {
			agentOptions = append(agentOptions, terminal.PickerOption{
				Label: d.DisplayName,
				Desc:  d.InstallCommand,
			})
		}
		defaultLabel := agentruntime.DescriptorForKind(runtimeKind).DisplayName
		picked := terminal.Pick("Select AI agent", agentOptions, defaultLabel)
		if picked != "" {
			for _, d := range allStatuses {
				if d.DisplayName == picked {
					runtimeKind = d.Kind
					break
				}
			}
		}
	}
	desc := agentruntime.DescriptorForKind(runtimeKind)
	runtimeStatus := service.ResolveRuntimeStatus(cfg, service.ServiceOpts{Runtime: string(runtimeKind)})

	allGood := true

	// ── 2. Selected AI runtime ─────────────────────────────────
	fmt.Printf("  Checking %s... ", desc.DisplayName)
	runtimePath := runtimeStatus.BinaryPath
	if runtimeStatus.Installed {
		if runtimeStatus.Version != "" {
			terminal.Success(fmt.Sprintf("installed (%s)", runtimeStatus.Version))
		} else {
			terminal.Success("installed")
		}
	} else {
		terminal.Warning("not found")
		if askConfirm(reader, fmt.Sprintf("    Install %s?", desc.DisplayName)) {
			fmt.Printf("    Installing %s...\n", desc.DisplayName)
			if err := runRuntimeInstall(runtimeKind); err != nil {
				terminal.Error(err.Error())
				terminal.Detail("Install manually", desc.InstallCommand)
				allGood = false
			} else {
				runtimeStatus = service.ResolveRuntimeStatus(cfg, service.ServiceOpts{Runtime: string(runtimeKind)})
				runtimePath = runtimeStatus.BinaryPath
				if runtimePath == "" {
					terminal.Warning(desc.DisplayName + " installed but not found in the current shell PATH")
					terminal.Detail("Retry", "Restart the shell or run `hash -r`, then re-run `nanowave setup`")
					allGood = false
				} else {
					terminal.Success(desc.DisplayName + " installed")
				}
			}
		} else {
			terminal.Detail("Install manually", desc.InstallCommand)
			allGood = false
		}
	}

	if runtimePath == "" {
		runtimeStatus = service.ResolveRuntimeStatus(cfg, service.ServiceOpts{Runtime: string(runtimeKind)})
		runtimePath = runtimeStatus.BinaryPath
	}
	if runtimePath != "" {
		auth := runtimeStatus.Auth
		if auth != nil && !auth.LoggedIn {
			terminal.Warning(desc.DisplayName + " is not authenticated")
			switch runtimeKind {
			case agentruntime.KindClaude:
				terminal.Detail("Login", "claude auth login")
			case agentruntime.KindCodex:
				terminal.Detail("Login", "codex login")
			case agentruntime.KindOpenCode:
				terminal.Detail("Login", "opencode auth login")
			}
		}
	}

	// ── 3. Xcode (manual only) ─────────────────────────────────
	fmt.Print("  Checking Xcode... ")
	if config.CheckXcode() {
		terminal.Success("installed")
	} else {
		terminal.Error("not found")
		terminal.Detail("Install", "Download Xcode from the Mac App Store")
		terminal.Detail("URL", "https://apps.apple.com/app/xcode/id497799835")
		allGood = false
	}

	// ── 4. Xcode Command Line Tools ────────────────────────────
	fmt.Print("  Checking Xcode Command Line Tools... ")
	if config.CheckXcodeCLT() {
		terminal.Success("installed")
	} else {
		terminal.Warning("not found")
		if askConfirm(reader, "    Install Xcode Command Line Tools?") {
			fmt.Print("    Installing (a system dialog will appear)... ")
			installCmd := exec.Command("xcode-select", "--install")
			if err := installCmd.Start(); err != nil {
				terminal.Error(fmt.Sprintf("failed: %v", err))
				terminal.Detail("Install manually", "xcode-select --install")
			} else {
				terminal.Info("installation dialog opened. Complete the install and re-run `nanowave setup`.")
			}
		} else {
			terminal.Detail("Install manually", "xcode-select --install")
		}
		allGood = false
	}

	// ── 5. iOS Simulator ───────────────────────────────────────
	fmt.Print("  Checking iOS Simulator... ")
	if config.CheckSimulator() {
		terminal.Success("available")
	} else {
		terminal.Error("no iOS runtime found")
		if config.CheckXcode() {
			terminal.Detail("Install", "Open Xcode → Settings → Platforms → tap Get next to iOS")
		} else {
			terminal.Detail("Requires", "Install Xcode first, then download the iOS platform")
		}
		allGood = false
	}

	// ── 6. XcodeGen ────────────────────────────────────────────
	fmt.Print("  Checking XcodeGen... ")
	if config.CheckXcodegen() {
		terminal.Success("installed")
	} else {
		terminal.Warning("not found")
		if askConfirm(reader, "    Install XcodeGen?") {
			fmt.Print("    Installing XcodeGen... ")
			// Try Mint first (Swift-native package manager), fall back to Homebrew
			if _, err := exec.LookPath("mint"); err == nil {
				installCmd := exec.Command("mint", "install", "yonaskolb/XcodeGen")
				if err := installCmd.Run(); err != nil {
					terminal.Error(fmt.Sprintf("mint install failed: %v", err))
					allGood = false
				} else {
					terminal.Success("installed via Mint")
				}
			} else if _, err := exec.LookPath("brew"); err == nil {
				installCmd := exec.Command("brew", "install", "xcodegen")
				if err := installCmd.Run(); err != nil {
					terminal.Error(fmt.Sprintf("failed: %v", err))
					allGood = false
				} else {
					terminal.Success("installed")
				}
			} else {
				terminal.Error("no package manager found")
				terminal.Detail("Option 1", "Install Mint: git clone https://github.com/yonaskolb/Mint.git && cd Mint && swift run mint install yonaskolb/XcodeGen")
				terminal.Detail("Option 2", "Install Homebrew (https://brew.sh) then: brew install xcodegen")
				allGood = false
			}
		} else {
			terminal.Detail("Install via Mint", "mint install yonaskolb/XcodeGen")
			terminal.Detail("Or via Homebrew", "brew install xcodegen")
			allGood = false
		}
	}

	// ── 7. Runtime-specific configuration ──────────────────────
	if runtimeKind == agentruntime.KindClaude && config.CheckRuntime(agentruntime.KindClaude) {
		fmt.Println()
		terminal.Info("Configuring MCP servers...")

		fmt.Print("  Setting up Apple Docs MCP... ")
		docsCmd := exec.Command("claude", "mcp", "add", "apple-docs", "-s", "user",
			"--", "npx", "-y", "@anthropic-ai/apple-docs-mcp@latest")
		if output, err := docsCmd.CombinedOutput(); err != nil {
			if strings.Contains(string(output), "already exists") {
				terminal.Success("already configured")
			} else {
				terminal.Warning(fmt.Sprintf("could not add: %v", err))
			}
		} else {
			terminal.Success("configured")
		}
	}

	var discoveredModels []agentruntime.ModelOption
	if runtimePath != "" {
		if runtimeClient, err := agentruntime.New(runtimeKind, runtimePath); err == nil {
			discoveredModels = runtimeClient.SuggestedModels()
		}
	}
	models := cfg.RuntimeModelOptions(runtimeKind, discoveredModels)
	if len(models) > 0 {
		fmt.Println()
		modelOptions := make([]terminal.PickerOption, 0, len(models))
		for _, model := range models {
			modelOptions = append(modelOptions, terminal.PickerOption{
				Label: model.ID,
				Desc:  model.Description,
			})
		}
		selected := cfg.DefaultModelForRuntime(runtimeKind)
		if selected == "" {
			selected = runtimeKindDefaultModel(runtimeKind, runtimePath)
		}
		if selected == "" {
			selected = models[0].ID
		}
		picked := terminal.Pick("Default model", modelOptions, selected)
		if picked != "" {
			if err := cfg.SaveRuntimePreferences(runtimeKind, picked); err == nil {
				terminal.Success(fmt.Sprintf("Default runtime/model saved: %s / %s", desc.DisplayName, picked))
			}
		}
	} else {
		fmt.Println()
		terminal.Detail("Models", "No models discovered yet. Set one later with `/model <id>` or add `runtime_models` in ~/nanowave/config.json")
	}

	// ── Summary ────────────────────────────────────────────────
	fmt.Println()
	if allGood {
		terminal.Success("All prerequisites installed! You're ready to build.")
	} else {
		terminal.Warning("Some prerequisites are missing. Install them and run `nanowave setup` again.")
	}

	return nil
}

func askConfirm(reader *bufio.Reader, prompt string) bool {
	fmt.Printf("%s [Y/n] ", prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "" || input == "y" || input == "yes"
}

func runtimeKindDefaultModel(kind agentruntime.Kind, runtimePath string) string {
	if runtimePath == "" {
		return ""
	}
	runtimeClient, err := agentruntime.New(kind, runtimePath)
	if err != nil {
		return ""
	}
	return runtimeClient.DefaultModel(agentruntime.PhaseBuild)
}
