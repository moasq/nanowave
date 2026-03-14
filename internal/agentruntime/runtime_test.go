package agentruntime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindBinaryFallsBackToHomeLocalBin(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	binPath := filepath.Join(binDir, "opencode")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin")

	got, err := FindBinary(KindOpenCode)
	if err != nil {
		t.Fatalf("FindBinary() error = %v", err)
	}
	if got != binPath {
		t.Fatalf("FindBinary() = %q, want %q", got, binPath)
	}
}
