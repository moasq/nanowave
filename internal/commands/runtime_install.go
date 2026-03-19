package commands

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/moasq/nanowave/internal/agentruntime"
	"github.com/moasq/nanowave/internal/terminal"
)

var errRuntimeInstallDeclined = errors.New("runtime installation declined")

func promptInstallRuntime(kind agentruntime.Kind) error {
	if path, err := agentruntime.FindBinary(kind); err == nil && path != "" {
		return nil
	}

	desc := agentruntime.DescriptorForKind(kind)
	terminal.Warning(desc.DisplayName + " CLI is not installed.")

	reader := bufio.NewReader(os.Stdin)
	if !askConfirm(reader, fmt.Sprintf("    Install %s now?", desc.DisplayName)) {
		return errRuntimeInstallDeclined
	}

	fmt.Printf("    Installing %s...\n", desc.DisplayName)
	if err := runRuntimeInstall(kind); err != nil {
		return err
	}

	if path, err := agentruntime.FindBinary(kind); err != nil || path == "" {
		return fmt.Errorf("%s was installed but Nanowave still can't find `%s`. Restart your shell or add it to PATH and try again", desc.DisplayName, desc.BinaryName)
	}

	terminal.Success(desc.DisplayName + " installed")
	return nil
}

func runRuntimeInstall(kind agentruntime.Kind) error {
	desc := agentruntime.DescriptorForKind(kind)
	installCmd := exec.Command("/bin/bash", "-lc", desc.InstallCommand)
	installCmd.Stdin = os.Stdin
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install %s: %w", desc.DisplayName, err)
	}
	return nil
}
