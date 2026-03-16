package orchestration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moasq/nanowave/internal/mcpregistry"
)

func TestWriteMCPConfigUsesPortableNanowaveCommand(t *testing.T) {
	projectDir := t.TempDir()
	reg := mcpregistry.New()
	mcpregistry.RegisterAll(reg)
	if err := writeMCPConfig(projectDir, reg, nil); err != nil {
		t.Fatalf("writeMCPConfig() error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(projectDir, ".mcp.json"))
	if err != nil {
		t.Fatalf("failed to read .mcp.json: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `"command": "nanowave"`) {
		t.Errorf(".mcp.json should use portable nanowave command, got:\n%s", text)
	}
}

func TestWriteGitignoreKeepsSharedClaudeAssetsTracked(t *testing.T) {
	projectDir := t.TempDir()
	if err := writeGitignore(projectDir); err != nil {
		t.Fatalf("writeGitignore() error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(projectDir, ".gitignore"))
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}
	text := string(data)
	if strings.Contains(text, "\n.claude/\n") {
		t.Fatal(".gitignore should not ignore the entire .claude directory")
	}
	if !strings.Contains(text, ".claude/settings.local.json") {
		t.Error(".gitignore should ignore .claude/settings.local.json")
	}
}

func TestLoadRuleContent(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		wantEmpty bool
		wantHas   string
	}{
		{
			name:    "core rule loads",
			key:     "swift-conventions",
			wantHas: "Swift",
		},
		{
			name:    "feature rule loads",
			key:     "camera",
			wantHas: "Camera",
		},
		{
			name:    "ui rule loads",
			key:     "gestures",
			wantHas: "Gesture",
		},
		{
			name:    "extension rule loads",
			key:     "widgets",
			wantHas: "Widget",
		},
		{
			name:    "always rule loads",
			key:     "design-system",
			wantHas: "AppTheme",
		},
		{
			name:    "multi-file always skill loads nested reference content",
			key:     "swiftui",
			wantHas: "Animation Process:",
		},
		{
			name:    "storage loads",
			key:     "storage",
			wantHas: "SwiftData",
		},
		{
			name:      "nonexistent key returns empty",
			key:       "nonexistent-key",
			wantEmpty: true,
		},
		{
			name:    "adaptive-layout loads NavigationSplitView",
			key:     "adaptive-layout",
			wantHas: "NavigationSplitView",
		},
		{
			name:    "navigation includes iPad content",
			key:     "navigation",
			wantHas: "NavigationSplitView",
		},
		{
			name:    "navigation includes base content",
			key:     "navigation",
			wantHas: "NavigationStack",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content := loadRuleContent(tc.key)
			if tc.wantEmpty {
				if content != "" {
					t.Errorf("expected empty content for %q, got %d chars", tc.key, len(content))
				}
				return
			}
			if content == "" {
				t.Fatalf("expected non-empty content for %q", tc.key)
			}
			if !strings.Contains(content, tc.wantHas) {
				t.Errorf("content for %q should contain %q", tc.key, tc.wantHas)
			}
			// Should NOT contain YAML frontmatter
			if strings.HasPrefix(content, "---") {
				t.Errorf("content for %q should have frontmatter stripped", tc.key)
			}
		})
	}
}

func TestCanonicalBuildCommandWatchOS(t *testing.T) {
	cmd := canonicalBuildCommand("WatchApp", "watchos")
	if !strings.Contains(cmd, "generic/platform=watchOS") {
		t.Errorf("watchOS build command should use watchOS device destination, got: %s", cmd)
	}
	if strings.Contains(cmd, "Simulator") {
		t.Errorf("watchOS build command should not use Simulator, got: %s", cmd)
	}
	if !strings.Contains(cmd, "CODE_SIGNING_ALLOWED=NO") {
		t.Errorf("watchOS build command should include CODE_SIGNING_ALLOWED=NO, got: %s", cmd)
	}
}

func TestCanonicalBuildCommandIOS(t *testing.T) {
	cmd := canonicalBuildCommand("IOSApp", "ios")
	if !strings.Contains(cmd, "generic/platform=iOS") {
		t.Errorf("iOS build command should use iOS device destination, got: %s", cmd)
	}
	if strings.Contains(cmd, "Simulator") {
		t.Errorf("iOS build command should not use Simulator by default, got: %s", cmd)
	}
	if !strings.Contains(cmd, "CODE_SIGNING_ALLOWED=NO") {
		t.Errorf("iOS build command should include CODE_SIGNING_ALLOWED=NO, got: %s", cmd)
	}
}

func TestCanonicalBuildCommandPairedWatchUsesIOSDestination(t *testing.T) {
	cmd := canonicalBuildCommandForShape("WristCounter", "watchos", WatchShapePaired)
	if !strings.Contains(cmd, "generic/platform=iOS") {
		t.Errorf("paired watch build command should use iOS device destination, got: %s", cmd)
	}
	if strings.Contains(cmd, "Simulator") {
		t.Errorf("paired watch build command should not use Simulator, got: %s", cmd)
	}
}

func TestPlatformSummaryWatchOS(t *testing.T) {
	summary := platformSummary("watchos", "")
	if !strings.Contains(summary, "Apple Watch") {
		t.Errorf("watchOS platform summary should mention Apple Watch, got: %s", summary)
	}
	if !strings.Contains(summary, "watchOS") {
		t.Errorf("watchOS platform summary should mention watchOS, got: %s", summary)
	}
}

func TestPlatformSummaryIOS(t *testing.T) {
	summary := platformSummary("ios", "iphone")
	if !strings.Contains(summary, "iPhone") {
		t.Errorf("iOS iphone summary should mention iPhone, got: %s", summary)
	}
}

func TestWriteAssetCatalogWatchOS(t *testing.T) {
	projectDir := t.TempDir()
	appDir := filepath.Join(projectDir, "WatchApp")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("failed to create app dir: %v", err)
	}

	if err := writeAssetCatalog(projectDir, "WatchApp", "watchos"); err != nil {
		t.Fatalf("writeAssetCatalog() error: %v", err)
	}

	iconPath := filepath.Join(projectDir, "WatchApp", "Assets.xcassets", "AppIcon.appiconset", "Contents.json")
	data, err := os.ReadFile(iconPath)
	if err != nil {
		t.Fatalf("failed to read AppIcon Contents.json: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "watchos") {
		t.Error("watchOS asset catalog should specify watchos platform")
	}
}

func TestScaffoldSourceDirsPaired(t *testing.T) {
	projectDir := t.TempDir()

	plan := &PlannerResult{
		Platform:          "watchos",
		WatchProjectShape: "paired_ios_watch",
	}

	if err := scaffoldSourceDirs(projectDir, "PairedApp", plan); err != nil {
		t.Fatalf("scaffoldSourceDirs() error: %v", err)
	}

	// Both main and watch directories should exist
	if _, err := os.Stat(filepath.Join(projectDir, "PairedApp")); err != nil {
		t.Error("expected PairedApp directory to exist")
	}
	if _, err := os.Stat(filepath.Join(projectDir, "PairedAppWatch")); err != nil {
		t.Error("expected PairedAppWatch directory to exist for paired watchOS")
	}
}

func TestScaffoldSourceDirsWatchOnly(t *testing.T) {
	projectDir := t.TempDir()

	plan := &PlannerResult{
		Platform:          "watchos",
		WatchProjectShape: "watch_only",
	}

	if err := scaffoldSourceDirs(projectDir, "WatchOnlyApp", plan); err != nil {
		t.Fatalf("scaffoldSourceDirs() error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(projectDir, "WatchOnlyApp")); err != nil {
		t.Error("expected WatchOnlyApp directory to exist")
	}
	// Should NOT create a Watch subdirectory for standalone
	if _, err := os.Stat(filepath.Join(projectDir, "WatchOnlyAppWatch")); !os.IsNotExist(err) {
		t.Error("watch_only should not create a separate Watch directory")
	}
}

func TestScaffoldSourceDirsMultiPlatform(t *testing.T) {
	projectDir := t.TempDir()

	plan := &PlannerResult{
		Platform:          "ios",
		Platforms:         []string{"ios", "watchos", "tvos"},
		DeviceFamily:      "universal",
		WatchProjectShape: "paired_ios_watch",
	}

	if err := scaffoldSourceDirs(projectDir, "FocusFlow", plan); err != nil {
		t.Fatalf("scaffoldSourceDirs() error: %v", err)
	}

	expected := []string{"FocusFlow", "FocusFlowWatch", "FocusFlowTV", "Shared"}
	for _, dir := range expected {
		if _, err := os.Stat(filepath.Join(projectDir, dir)); err != nil {
			t.Errorf("expected %s directory to exist", dir)
		}
	}
}

func TestMultiPlatformBuildCommands(t *testing.T) {
	cmds := multiPlatformBuildCommands("FocusFlow", []string{"ios", "watchos", "tvos"})

	// watchOS is built via iOS scheme (paired), so we expect iOS + tvOS commands
	if len(cmds) < 2 {
		t.Fatalf("expected at least 2 build commands, got %d", len(cmds))
	}

	hasIOS := false
	hasTV := false
	for _, cmd := range cmds {
		if strings.Contains(cmd, "FocusFlow.xcodeproj") && strings.Contains(cmd, "generic/platform=iOS") && strings.Contains(cmd, "CODE_SIGNING_ALLOWED=NO") {
			hasIOS = true
		}
		if strings.Contains(cmd, "FocusFlowTV") && strings.Contains(cmd, "generic/platform=tvOS") && strings.Contains(cmd, "CODE_SIGNING_ALLOWED=NO") {
			hasTV = true
		}
	}
	if !hasIOS {
		t.Error("expected iOS device build command with CODE_SIGNING_ALLOWED=NO")
	}
	if !hasTV {
		t.Error("expected tvOS device build command with CODE_SIGNING_ALLOWED=NO")
	}
}
