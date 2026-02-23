package claude

import (
	"fmt"
	"os/exec"
	"strings"
)

// ValidateEnvironment checks that Claude Code CLI is installed and authenticated.
func ValidateEnvironment() error {
	// Check claude CLI exists
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("Claude Code CLI not found.\nInstall: curl -fsSL https://claude.ai/install.sh | bash")
	}

	// Check version
	cmd := exec.Command(claudePath, "--version")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Claude Code CLI found but cannot get version: %w", err)
	}

	version := strings.TrimSpace(string(out))
	if version == "" {
		return fmt.Errorf("Claude Code CLI returned empty version")
	}

	return nil
}
