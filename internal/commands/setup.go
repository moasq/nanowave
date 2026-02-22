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

	// ── 4. Homebrew ────────────────────────────────────────────
	fmt.Print("  Checking Homebrew... ")
	if config.CheckHomebrew() {
		terminal.Success("installed")
	} else {
		terminal.Warning("not found")
		if askConfirm(reader, "    Install Homebrew?") {
			fmt.Println("    Installing Homebrew (may ask for your password)...")
			installCmd := exec.Command("/bin/bash", "-c",
				`$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)`)
			installCmd.Stdin = os.Stdin
			installCmd.Stdout = os.Stdout
			installCmd.Stderr = os.Stderr
			if err := installCmd.Run(); err != nil {
				terminal.Error(fmt.Sprintf("failed to install Homebrew: %v", err))
				terminal.Detail("Install manually", "https://brew.sh")
				allGood = false
			} else {
				terminal.Success("Homebrew installed")
			}
		} else {
			terminal.Detail("Install manually", "https://brew.sh")
			allGood = false
		}
	}

	// ── 5. Node.js + npm ───────────────────────────────────────
	fmt.Print("  Checking Node.js... ")
	if config.CheckNode() && config.CheckNpm() {
		version := config.NodeVersion()
		terminal.Success(fmt.Sprintf("installed (%s)", version))
	} else {
		terminal.Warning("not found")
		if config.CheckHomebrew() {
			if askConfirm(reader, "    Install Node.js via Homebrew?") {
				fmt.Print("    Installing Node.js... ")
				installCmd := exec.Command("brew", "install", "node")
				if err := installCmd.Run(); err != nil {
					terminal.Error(fmt.Sprintf("failed: %v", err))
					terminal.Detail("Install manually", "brew install node")
					allGood = false
				} else {
					terminal.Success("installed")
				}
			} else {
				terminal.Detail("Install manually", "brew install node")
				allGood = false
			}
		} else {
			terminal.Detail("Install", "Install Homebrew first, then run: brew install node")
			terminal.Detail("Or download from", "https://nodejs.org")
			allGood = false
		}
	}

	// ── 6. Claude Code CLI ─────────────────────────────────────
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
		if config.CheckNpm() {
			if askConfirm(reader, "    Install Claude Code CLI via npm?") {
				fmt.Print("    Installing Claude Code CLI... ")
				installCmd := exec.Command("npm", "install", "-g", "@anthropic-ai/claude-code")
				if err := installCmd.Run(); err != nil {
					terminal.Error(fmt.Sprintf("failed: %v", err))
					terminal.Detail("Install manually", "npm install -g @anthropic-ai/claude-code")
					allGood = false
				} else {
					terminal.Success("installed")
				}
			} else {
				terminal.Detail("Install manually", "npm install -g @anthropic-ai/claude-code")
				allGood = false
			}
		} else {
			terminal.Detail("Install", "Install Node.js first, then run: npm install -g @anthropic-ai/claude-code")
			allGood = false
		}
	}

	// ── 7. XcodeGen ────────────────────────────────────────────
	fmt.Print("  Checking XcodeGen... ")
	if config.CheckXcodegen() {
		terminal.Success("installed")
	} else {
		terminal.Warning("not found")
		if config.CheckHomebrew() {
			if askConfirm(reader, "    Install XcodeGen via Homebrew?") {
				fmt.Print("    Installing XcodeGen... ")
				installCmd := exec.Command("brew", "install", "xcodegen")
				if err := installCmd.Run(); err != nil {
					terminal.Error(fmt.Sprintf("failed: %v", err))
					terminal.Detail("Install manually", "brew install xcodegen")
					allGood = false
				} else {
					terminal.Success("installed")
				}
			} else {
				terminal.Detail("Install manually", "brew install xcodegen")
				allGood = false
			}
		} else {
			terminal.Detail("Install", "Install Homebrew first, then run: brew install xcodegen")
			allGood = false
		}
	}

	// ── 8. MCP Servers (only if Claude Code is available) ──────
	if config.CheckClaude() {
		fmt.Println()
		terminal.Info("Configuring MCP servers...")

		fmt.Print("  Setting up XcodeBuildMCP... ")
		mcpCmd := exec.Command("claude", "mcp", "add", "XcodeBuildMCP", "-s", "user",
			"-e", "XCODEBUILDMCP_SENTRY_DISABLED=true",
			"--", "npx", "-y", "xcodebuildmcp@latest", "mcp")
		if output, err := mcpCmd.CombinedOutput(); err != nil {
			if strings.Contains(string(output), "already exists") {
				terminal.Success("already configured")
			} else {
				terminal.Warning(fmt.Sprintf("could not add: %v", err))
			}
		} else {
			terminal.Success("configured")
		}

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
