package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/moasq/nanowave/internal/config"
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
	return !config.CheckClaude() || !config.CheckXcodegen() || !config.CheckXcode() || !config.CheckSimulator()
}

// runSetup checks and installs all prerequisites. Returns nil on success.
func runSetup() error {
	terminal.Header("Nanowave Setup")
	fmt.Println()

	allGood := true
	reader := bufio.NewReader(os.Stdin)

	// ── 1. Xcode (manual only) ─────────────────────────────────
	fmt.Print("  Checking Xcode... ")
	if config.CheckXcode() {
		terminal.Success("installed")
	} else {
		terminal.Error("not found")
		terminal.Detail("Install", "Download Xcode from the Mac App Store")
		terminal.Detail("URL", "https://apps.apple.com/app/xcode/id497799835")
		allGood = false
	}

	// ── 2. Xcode Command Line Tools ────────────────────────────
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

	// ── 3. iOS Simulator ───────────────────────────────────────
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

	// ── 4. Claude Code CLI (native install) ────────────────────
	fmt.Print("  Checking Claude Code CLI... ")
	if config.CheckClaude() {
		cfg, _ := config.Load()
		if cfg != nil {
			version := config.ClaudeVersion(cfg.ClaudePath)
			terminal.Success(fmt.Sprintf("installed (v%s)", version))
		} else {
			terminal.Success("installed")
		}
	} else {
		terminal.Warning("not found")
		if askConfirm(reader, "    Install Claude Code CLI?") {
			fmt.Println("    Installing Claude Code CLI...")
			installCmd := exec.Command("/bin/bash", "-c",
				`curl -fsSL https://claude.ai/install.sh | bash`)
			installCmd.Stdin = os.Stdin
			installCmd.Stdout = os.Stdout
			installCmd.Stderr = os.Stderr
			if err := installCmd.Run(); err != nil {
				terminal.Error(fmt.Sprintf("failed: %v", err))
				terminal.Detail("Install manually", "curl -fsSL https://claude.ai/install.sh | bash")
				allGood = false
			} else {
				terminal.Success("Claude Code CLI installed")
			}
		} else {
			terminal.Detail("Install manually", "curl -fsSL https://claude.ai/install.sh | bash")
			allGood = false
		}
	}

	// ── 5. XcodeGen ────────────────────────────────────────────
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

	// ── 6. Supabase CLI (optional — for backend integration) ──
	fmt.Print("  Checking Supabase CLI... ")
	if config.CheckSupabaseCLI() {
		terminal.Success("installed")
	} else {
		terminal.Warning("not found (optional — needed for backend integration)")
		if askConfirm(reader, "    Install Supabase CLI?") {
			fmt.Print("    Installing Supabase CLI... ")
			if _, err := exec.LookPath("brew"); err == nil {
				tapCmd := exec.Command("brew", "install", "supabase/tap/supabase")
				if err := tapCmd.Run(); err != nil {
					terminal.Error(fmt.Sprintf("failed: %v", err))
					terminal.Detail("Install manually", "brew install supabase/tap/supabase")
				} else {
					terminal.Success("installed")
				}
			} else {
				terminal.Error("Homebrew not found")
				terminal.Detail("Install Homebrew", "https://brew.sh")
				terminal.Detail("Then run", "brew install supabase/tap/supabase")
			}
		} else {
			terminal.Detail("Install later", "brew install supabase/tap/supabase")
		}
	}

	// ── 7. MCP Servers (only if Claude Code is available) ──────
	if config.CheckClaude() {
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
