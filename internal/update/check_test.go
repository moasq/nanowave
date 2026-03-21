package update

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpgradeCommandForPathDetectsHomebrewInstall(t *testing.T) {
	got := upgradeCommandForPath("/opt/homebrew/Cellar/nanowave/0.35.2/bin/nanowave")
	want := "brew upgrade moasq/tap/nanowave"
	if got != want {
		t.Fatalf("upgradeCommandForPath() = %q, want %q", got, want)
	}
}

func TestUpgradeCommandForPathDetectsSourceCheckout(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"))
	writeTestFile(t, filepath.Join(root, "Makefile"))
	binPath := filepath.Join(root, "bin", "nanowave")
	writeTestFile(t, binPath)

	got := upgradeCommandForPath(binPath)
	want := "git -C '" + root + "' pull && make -C '" + root + "' build"
	if got != want {
		t.Fatalf("upgradeCommandForPath() = %q, want %q", got, want)
	}
}

func TestUpgradeCommandForPathFallsBackWhenUnknown(t *testing.T) {
	got := upgradeCommandForPath("/tmp/nanowave")
	if got != "" {
		t.Fatalf("upgradeCommandForPath() = %q, want empty string", got)
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
