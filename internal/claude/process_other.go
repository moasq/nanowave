//go:build !unix

package claude

import "os/exec"

func configureCommandProcess(cmd *exec.Cmd) {}

func killCommandProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
