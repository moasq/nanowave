package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectDerivedDataPath(t *testing.T) {
	projectPath := filepath.Join(string(filepath.Separator), "tmp", "SampleApp")
	got := projectDerivedDataPath(projectPath)
	want := filepath.Join(projectPath, ".nanowave", "DerivedData")
	if got != want {
		t.Fatalf("projectDerivedDataPath() = %q, want %q", got, want)
	}
}

func TestFindBuiltAppInDerivedDataIOSExactMatch(t *testing.T) {
	derived := t.TempDir()
	productsDir := filepath.Join(derived, "Build", "Products", "Debug-iphonesimulator")
	if err := os.MkdirAll(productsDir, 0o755); err != nil {
		t.Fatalf("failed to create products dir: %v", err)
	}

	// Multiple .app bundles present; exact scheme match must win deterministically.
	for _, name := range []string{"Other.app", "MyApp.app"} {
		if err := os.MkdirAll(filepath.Join(productsDir, name), 0o755); err != nil {
			t.Fatalf("failed to create app bundle %s: %v", name, err)
		}
	}

	got, err := findBuiltAppInDerivedData(derived, "MyApp", "ios")
	if err != nil {
		t.Fatalf("findBuiltAppInDerivedData() error = %v", err)
	}
	want := filepath.Join(productsDir, "MyApp.app")
	if got != want {
		t.Fatalf("findBuiltAppInDerivedData() = %q, want %q", got, want)
	}
}

func TestFindBuiltAppInDerivedDataWatchOSUsesWatchProductsDir(t *testing.T) {
	derived := t.TempDir()
	productsDir := filepath.Join(derived, "Build", "Products", "Debug-watchsimulator")
	if err := os.MkdirAll(filepath.Join(productsDir, "WatchApp.app"), 0o755); err != nil {
		t.Fatalf("failed to create watch app bundle: %v", err)
	}

	got, err := findBuiltAppInDerivedData(derived, "WatchApp", "watchos")
	if err != nil {
		t.Fatalf("findBuiltAppInDerivedData() error = %v", err)
	}
	want := filepath.Join(productsDir, "WatchApp.app")
	if got != want {
		t.Fatalf("findBuiltAppInDerivedData() = %q, want %q", got, want)
	}
}

func TestFindBuiltAppInDerivedDataMissingExactMatchReturnsError(t *testing.T) {
	derived := t.TempDir()
	productsDir := filepath.Join(derived, "Build", "Products", "Debug-iphonesimulator")
	if err := os.MkdirAll(productsDir, 0o755); err != nil {
		t.Fatalf("failed to create products dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(productsDir, "Other.app"), 0o755); err != nil {
		t.Fatalf("failed to create app bundle: %v", err)
	}

	_, err := findBuiltAppInDerivedData(derived, "MyApp", "ios")
	if err == nil {
		t.Fatal("expected error when exact app bundle is missing")
	}
	if !strings.Contains(err.Error(), "MyApp.app") {
		t.Fatalf("expected error to mention MyApp.app, got %q", err.Error())
	}
}

func TestFindBuiltAppInDerivedDataNoAppsReturnsError(t *testing.T) {
	derived := t.TempDir()
	productsDir := filepath.Join(derived, "Build", "Products", "Debug-iphonesimulator")
	if err := os.MkdirAll(productsDir, 0o755); err != nil {
		t.Fatalf("failed to create products dir: %v", err)
	}

	_, err := findBuiltAppInDerivedData(derived, "MyApp", "ios")
	if err == nil {
		t.Fatal("expected error when no app bundles exist")
	}
	if !strings.Contains(err.Error(), "no .app bundle") {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}

func TestFindBuiltAppInDerivedDataTvOSUsesAppleTVProductsDir(t *testing.T) {
	derived := t.TempDir()
	productsDir := filepath.Join(derived, "Build", "Products", "Debug-appletvsimulator")
	if err := os.MkdirAll(filepath.Join(productsDir, "TVApp.app"), 0o755); err != nil {
		t.Fatalf("failed to create tvOS app bundle: %v", err)
	}

	got, err := findBuiltAppInDerivedData(derived, "TVApp", "tvos")
	if err != nil {
		t.Fatalf("findBuiltAppInDerivedData() error = %v", err)
	}
	want := filepath.Join(productsDir, "TVApp.app")
	if got != want {
		t.Fatalf("findBuiltAppInDerivedData() = %q, want %q", got, want)
	}
}

func TestRankSimulatorTvOS(t *testing.T) {
	// Apple TV 4K should rank highest
	score4K := rankSimulator("com.apple.CoreSimulator.SimDeviceType.Apple-TV-4K-3rd-generation-4K", "", "tvos")
	if score4K != 100 {
		t.Fatalf("Apple TV 4K score = %d, want 100", score4K)
	}

	// Non-TV device should be rejected
	scoreIPhone := rankSimulator("com.apple.CoreSimulator.SimDeviceType.iPhone-16-Pro", "", "tvos")
	if scoreIPhone != -1 {
		t.Fatalf("iPhone score for tvOS = %d, want -1", scoreIPhone)
	}
}

func TestIsAlreadyBootedSimError(t *testing.T) {
	tests := []struct {
		name   string
		errMsg string
		output string
		want   bool
	}{
		{
			name:   "already booted text",
			errMsg: "exit status 149",
			output: "Device is already booted",
			want:   true,
		},
		{
			name:   "current state booted text",
			errMsg: "exit status 149",
			output: "Unable to boot device in current state: Booted",
			want:   true,
		},
		{
			name:   "different simctl error",
			errMsg: "exit status 1",
			output: "No devices are booted",
			want:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isAlreadyBootedSimError(fakeErr(tc.errMsg), []byte(tc.output))
			if got != tc.want {
				t.Fatalf("isAlreadyBootedSimError() = %v, want %v", got, tc.want)
			}
		})
	}
}

type fakeErr string

func (e fakeErr) Error() string { return string(e) }
