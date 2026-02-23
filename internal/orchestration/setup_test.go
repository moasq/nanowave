package orchestration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteCoreRules(t *testing.T) {
	projectDir := t.TempDir()
	rulesDir := filepath.Join(projectDir, ".claude", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatalf("failed to create rules dir: %v", err)
	}

	if err := writeCoreRules(projectDir); err != nil {
		t.Fatalf("writeCoreRules() error: %v", err)
	}

	expectedFiles := []string{
		"swift-conventions.md",
		"mvvm-architecture.md",
		"forbidden-patterns.md",
		"file-structure.md",
	}

	for _, name := range expectedFiles {
		path := filepath.Join(rulesDir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected core rule %s to exist, got error: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("core rule %s is empty", name)
		}
	}
}

func TestWriteAlwaysSkills(t *testing.T) {
	projectDir := t.TempDir()
	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("failed to create skills dir: %v", err)
	}

	if err := writeAlwaysSkills(projectDir, "ios"); err != nil {
		t.Fatalf("writeAlwaysSkills() error: %v", err)
	}

	// Verify always skills are emitted as Anthropic-style folders with SKILL.md
	flatSkills := []string{"design-system", "layout", "navigation", "components"}
	for _, name := range flatSkills {
		path := filepath.Join(skillsDir, name, "SKILL.md")
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected %s/SKILL.md to exist, got error: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("%s/SKILL.md is empty", name)
		}
	}

	// Verify multi-file skill (swiftui/) is copied as directory with reference/ companions
	swiftuiFiles := []string{
		"SKILL.md",
		"reference/animations.md",
		"reference/forms.md",
		"reference/lists.md",
		"reference/scroll.md",
		"reference/state.md",
		"reference/performance.md",
		"reference/text.md",
		"reference/media.md",
		"reference/modern-apis.md",
		"reference/liquid-glass.md",
	}
	for _, fileName := range swiftuiFiles {
		path := filepath.Join(skillsDir, "swiftui", fileName)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected swiftui/%s to exist, got error: %v", fileName, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("swiftui/%s is empty", fileName)
		}
	}

	// Verify SKILL.md has Anthropic-style frontmatter (name + description only)
	skillMD, err := os.ReadFile(filepath.Join(skillsDir, "design-system", "SKILL.md"))
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}
	if !strings.HasPrefix(string(skillMD), "---") {
		t.Error("SKILL.md should have YAML frontmatter")
	}
	if !strings.Contains(string(skillMD), "\nname: ") {
		t.Error("SKILL.md should include name in frontmatter")
	}
	if !strings.Contains(string(skillMD), "\ndescription: ") {
		t.Error("SKILL.md should include description in frontmatter")
	}
	if strings.Contains(string(skillMD), "user-invocable:") {
		t.Error("SKILL.md should not include legacy user-invocable frontmatter")
	}

	// Verify review multi-file skill is copied with reference/ companions
	reviewFiles := []string{"SKILL.md", "reference/quality-review.md", "reference/accessibility-audit.md", "reference/output-format.md"}
	for _, fileName := range reviewFiles {
		path := filepath.Join(skillsDir, "review", fileName)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected review/%s to exist, got error: %v", fileName, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("review/%s is empty", fileName)
		}
	}
}

func TestWriteConditionalSkills(t *testing.T) {
	projectDir := t.TempDir()
	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("failed to create skills dir: %v", err)
	}

	ruleKeys := []string{"camera", "localization", "gestures"}
	if err := writeConditionalSkills(projectDir, ruleKeys, "ios"); err != nil {
		t.Fatalf("writeConditionalSkills() error: %v", err)
	}

	// Verify matching keys are written
	for _, key := range ruleKeys {
		skillDir := filepath.Join(skillsDir, key)
		if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
			t.Errorf("expected %s/SKILL.md to exist: %v", key, err)
		}
	}

	// Verify non-matching keys are NOT written
	absent := []string{"maps", "widgets", "healthkit"}
	for _, key := range absent {
		skillDir := filepath.Join(skillsDir, key)
		if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
			t.Errorf("expected %s to NOT exist (not in ruleKeys)", key)
		}
	}
}

func TestWriteConditionalSkillsEmpty(t *testing.T) {
	projectDir := t.TempDir()
	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("failed to create skills dir: %v", err)
	}

	if err := writeConditionalSkills(projectDir, nil, "ios"); err != nil {
		t.Fatalf("writeConditionalSkills(nil) error: %v", err)
	}

	// Skills dir should be empty
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		t.Fatalf("failed to read skills dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty skills dir with nil ruleKeys, got %d entries", len(entries))
	}
}

func TestWriteInitialCLAUDEMDCreatesImportIndexAndMemory(t *testing.T) {
	projectDir := t.TempDir()
	if err := setupWorkspace(projectDir); err != nil {
		t.Fatalf("setupWorkspace() error: %v", err)
	}

	if err := writeInitialCLAUDEMD(projectDir, "SampleApp", "ios", "iphone"); err != nil {
		t.Fatalf("writeInitialCLAUDEMD() error: %v", err)
	}

	claudePath := filepath.Join(projectDir, ".claude", "CLAUDE.md")
	data, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "@memory/project-overview.md") {
		t.Errorf("CLAUDE.md should import project-overview memory module")
	}
	if !strings.Contains(text, "@memory/review-playbook.md") {
		t.Errorf("CLAUDE.md should import review-playbook memory module")
	}
	if !strings.Contains(text, "@memory/accessibility-policy.md") {
		t.Errorf("CLAUDE.md should import accessibility-policy memory module")
	}
	if !strings.Contains(text, "@rules/file-structure.md") {
		t.Errorf("CLAUDE.md should explicitly import core rules")
	}

	memoryFiles := []string{
		"project-overview.md",
		"architecture.md",
		"design-system.md",
		"xcodegen-policy.md",
		"build-fix-workflow.md",
		"review-playbook.md",
		"accessibility-policy.md",
		"quality-gates.md",
		"generated-plan.md",
	}
	for _, name := range memoryFiles {
		path := filepath.Join(projectDir, ".claude", "memory", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected memory file %s: %v", name, err)
		}
	}
}

func TestWriteClaudeProjectScaffoldCreatesArtifacts(t *testing.T) {
	projectDir := t.TempDir()
	if err := setupWorkspace(projectDir); err != nil {
		t.Fatalf("setupWorkspace() error: %v", err)
	}
	if err := writeAlwaysSkills(projectDir, "ios"); err != nil {
		t.Fatalf("writeAlwaysSkills() error: %v", err)
	}
	if err := writeConditionalSkills(projectDir, []string{"camera"}, "ios"); err != nil {
		t.Fatalf("writeConditionalSkills() error: %v", err)
	}

	if err := writeClaudeProjectScaffold(projectDir, "SampleApp", "ios"); err != nil {
		t.Fatalf("writeClaudeProjectScaffold() error: %v", err)
	}

	expected := []string{
		filepath.Join(projectDir, ".claude", "skills", "INDEX.md"),
		filepath.Join(projectDir, ".claude", "commands", "preflight.md"),
		filepath.Join(projectDir, ".claude", "commands", "accessibility-audit.md"),
		filepath.Join(projectDir, ".claude", "agents", "ios-api-researcher.md"),
		filepath.Join(projectDir, ".claude", "agents", "swiftui-accessibility-reviewer.md"),
		filepath.Join(projectDir, ".claude", "settings.json"),
		filepath.Join(projectDir, "scripts", "claude", "validate-skills.sh"),
		filepath.Join(projectDir, "scripts", "claude", "check-a11y-dynamic-type.sh"),
		filepath.Join(projectDir, "scripts", "claude", "check-a11y-icon-buttons.sh"),
		filepath.Join(projectDir, "docs", "claude-workflow.md"),
		filepath.Join(projectDir, "Makefile"),
		filepath.Join(projectDir, ".github", "workflows", "claude-quality.yml"),
	}
	for _, p := range expected {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected generated scaffold file %s: %v", p, err)
		}
	}

	skillIndex, err := os.ReadFile(filepath.Join(projectDir, ".claude", "skills", "INDEX.md"))
	if err != nil {
		t.Fatalf("failed to read skill index: %v", err)
	}
	if !strings.Contains(string(skillIndex), "design-system") {
		t.Error("skill index should include generated skills")
	}
	if !strings.Contains(string(skillIndex), "review") {
		t.Error("skill index should include review skill")
	}

	settingsData, err := os.ReadFile(filepath.Join(projectDir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("failed to read settings.json: %v", err)
	}
	settingsText := string(settingsData)
	if !strings.Contains(settingsText, `"ViewImage"`) {
		t.Error("settings.json should allow ViewImage for optional screenshot review")
	}
	if !strings.Contains(settingsText, "check-a11y-dynamic-type.sh --hook") {
		t.Error("settings.json should register dynamic type a11y hook")
	}
	if !strings.Contains(settingsText, "check-a11y-icon-buttons.sh --hook") {
		t.Error("settings.json should register icon button a11y hook")
	}

	makefileData, err := os.ReadFile(filepath.Join(projectDir, "Makefile"))
	if err != nil {
		t.Fatalf("failed to read Makefile: %v", err)
	}
	makefileText := string(makefileData)
	if !strings.Contains(makefileText, "./scripts/claude/check-a11y-dynamic-type.sh") {
		t.Error("Makefile claude-check should include dynamic type a11y check")
	}
	if !strings.Contains(makefileText, "./scripts/claude/check-a11y-icon-buttons.sh") {
		t.Error("Makefile claude-check should include icon-button a11y check")
	}

	workflowData, err := os.ReadFile(filepath.Join(projectDir, "docs", "claude-workflow.md"))
	if err != nil {
		t.Fatalf("failed to read docs/claude-workflow.md: %v", err)
	}
	if !strings.Contains(string(workflowData), "/accessibility-audit") {
		t.Error("workflow docs should mention /accessibility-audit")
	}
}

func TestWriteMCPConfigUsesPortableNanowaveCommand(t *testing.T) {
	projectDir := t.TempDir()
	if err := writeMCPConfig(projectDir); err != nil {
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

func TestGeneratedReviewCommandsReferenceNewMemoryAndOutputContract(t *testing.T) {
	projectDir := t.TempDir()
	if err := setupWorkspace(projectDir); err != nil {
		t.Fatalf("setupWorkspace() error: %v", err)
	}
	if err := writeAlwaysSkills(projectDir, "ios"); err != nil {
		t.Fatalf("writeAlwaysSkills() error: %v", err)
	}
	if err := writeClaudeProjectScaffold(projectDir, "SampleApp", "ios"); err != nil {
		t.Fatalf("writeClaudeProjectScaffold() error: %v", err)
	}

	qualityCmd, err := os.ReadFile(filepath.Join(projectDir, ".claude", "commands", "quality-review.md"))
	if err != nil {
		t.Fatalf("failed to read quality-review command: %v", err)
	}
	qualityText := string(qualityCmd)
	if !strings.Contains(qualityText, "@memory/review-playbook.md") {
		t.Error("quality-review should reference @memory/review-playbook.md")
	}
	if !strings.Contains(qualityText, "## Escalation") {
		t.Error("quality-review should require ## Escalation section")
	}
	if !strings.Contains(qualityText, "/accessibility-audit") {
		t.Error("quality-review should instruct escalation to /accessibility-audit")
	}

	a11yCmd, err := os.ReadFile(filepath.Join(projectDir, ".claude", "commands", "accessibility-audit.md"))
	if err != nil {
		t.Fatalf("failed to read accessibility-audit command: %v", err)
	}
	a11yText := string(a11yCmd)
	if !strings.Contains(a11yText, "@memory/accessibility-policy.md") {
		t.Error("accessibility-audit should reference @memory/accessibility-policy.md")
	}
	if !strings.Contains(a11yText, "## Checklist Coverage") {
		t.Error("accessibility-audit should require ## Checklist Coverage section")
	}
	if !strings.Contains(a11yText, "severity-ordered") {
		t.Error("accessibility-audit should require severity-ordered findings")
	}
}

func TestWriteClaudeMemoryFilesAddsReviewAndAccessibilityModules(t *testing.T) {
	projectDir := t.TempDir()
	if err := setupWorkspace(projectDir); err != nil {
		t.Fatalf("setupWorkspace() error: %v", err)
	}
	if err := writeClaudeMemoryFiles(projectDir, "SampleApp", "ios", "iphone", nil); err != nil {
		t.Fatalf("writeClaudeMemoryFiles() error: %v", err)
	}

	reviewData, err := os.ReadFile(filepath.Join(projectDir, ".claude", "memory", "review-playbook.md"))
	if err != nil {
		t.Fatalf("failed to read review-playbook.md: %v", err)
	}
	if !strings.Contains(string(reviewData), "/accessibility-audit") {
		t.Error("review-playbook should describe when to use /accessibility-audit")
	}

	a11yData, err := os.ReadFile(filepath.Join(projectDir, ".claude", "memory", "accessibility-policy.md"))
	if err != nil {
		t.Fatalf("failed to read accessibility-policy.md: %v", err)
	}
	if !strings.Contains(string(a11yData), "Icon-only controls must include") {
		t.Error("accessibility-policy should include icon-only controls rule")
	}
}

func TestCheckA11yDynamicTypeScript(t *testing.T) {
	projectDir := setupClaudeScriptTestProject(t)
	swiftPath := filepath.Join(projectDir, "App", "ContentView.swift")

	if err := os.WriteFile(swiftPath, []byte(`import SwiftUI
struct ContentView: View {
    var body: some View {
        Text("Hi").font(.system(size: 14))
    }
}
`), 0o644); err != nil {
		t.Fatalf("failed to write swift file: %v", err)
	}

	code, out := runClaudeScript(t, projectDir, "check-a11y-dynamic-type.sh")
	if code == 0 {
		t.Fatalf("expected dynamic type check to fail, got code %d output=%s", code, out)
	}
	if !strings.Contains(out, "Fixed font size usage detected") {
		t.Errorf("expected dynamic type check output to mention fixed font size, got: %s", out)
	}

	code, _ = runClaudeScript(t, projectDir, "check-a11y-dynamic-type.sh", "--hook")
	if code != 0 {
		t.Fatalf("expected hook mode to be advisory (exit 0), got %d", code)
	}

	if err := os.WriteFile(swiftPath, []byte(`import SwiftUI
struct ContentView: View {
    var body: some View {
        Text("Hi").font(.body)
    }
}
`), 0o644); err != nil {
		t.Fatalf("failed to rewrite swift file: %v", err)
	}
	code, out = runClaudeScript(t, projectDir, "check-a11y-dynamic-type.sh")
	if code != 0 {
		t.Fatalf("expected dynamic type check to pass for system text style, got %d output=%s", code, out)
	}

	if err := os.WriteFile(swiftPath, []byte(`import SwiftUI
struct ContentView: View {
    var body: some View {
        Text("Hi").font(.system(size: 14)) // claude-a11y:ignore fixed-font
    }
}
`), 0o644); err != nil {
		t.Fatalf("failed to rewrite swift file with ignore marker: %v", err)
	}
	code, out = runClaudeScript(t, projectDir, "check-a11y-dynamic-type.sh")
	if code != 0 {
		t.Fatalf("expected ignore marker to suppress dynamic type violation, got %d output=%s", code, out)
	}
}

func TestCheckA11yIconButtonsScript(t *testing.T) {
	projectDir := setupClaudeScriptTestProject(t)
	swiftPath := filepath.Join(projectDir, "Features", "SampleFeature", "IconButtonView.swift")
	if err := os.MkdirAll(filepath.Dir(swiftPath), 0o755); err != nil {
		t.Fatalf("failed to create feature dir: %v", err)
	}

	if err := os.WriteFile(swiftPath, []byte(`import SwiftUI
struct IconButtonView: View {
    var body: some View {
        Button {
            print("tap")
        } label: {
            Image(systemName: "plus")
        }
    }
}
`), 0o644); err != nil {
		t.Fatalf("failed to write swift file: %v", err)
	}

	code, out := runClaudeScript(t, projectDir, "check-a11y-icon-buttons.sh")
	if code == 0 {
		t.Fatalf("expected icon-button a11y check to fail, got code %d output=%s", code, out)
	}
	if !strings.Contains(out, "icon-only button") {
		t.Errorf("expected icon-button a11y output to mention icon-only button, got: %s", out)
	}

	code, _ = runClaudeScript(t, projectDir, "check-a11y-icon-buttons.sh", "--hook")
	if code != 0 {
		t.Fatalf("expected icon-button hook mode to be advisory (exit 0), got %d", code)
	}

	if err := os.WriteFile(swiftPath, []byte(`import SwiftUI
struct IconButtonView: View {
    var body: some View {
        Button {
            print("tap")
        } label: {
            Image(systemName: "plus")
        }
        .accessibilityLabel("Add item")
    }
}
`), 0o644); err != nil {
		t.Fatalf("failed to rewrite swift file with accessibilityLabel: %v", err)
	}
	code, out = runClaudeScript(t, projectDir, "check-a11y-icon-buttons.sh")
	if code != 0 {
		t.Fatalf("expected icon-button a11y check to pass with accessibilityLabel, got %d output=%s", code, out)
	}

	if err := os.WriteFile(swiftPath, []byte(`import SwiftUI
struct IconButtonView: View {
    var body: some View {
        Button {
            print("tap")
        } label: {
            Image(systemName: "plus") // claude-a11y:ignore icon-button-label
        }
    }
}
`), 0o644); err != nil {
		t.Fatalf("failed to rewrite swift file with ignore marker: %v", err)
	}
	code, out = runClaudeScript(t, projectDir, "check-a11y-icon-buttons.sh")
	if code != 0 {
		t.Fatalf("expected ignore marker to suppress icon-button violation, got %d output=%s", code, out)
	}
}

func TestWriteInitialCLAUDEMDWatchOS(t *testing.T) {
	projectDir := t.TempDir()
	if err := setupWorkspace(projectDir); err != nil {
		t.Fatalf("setupWorkspace() error: %v", err)
	}

	if err := writeInitialCLAUDEMD(projectDir, "WatchApp", "watchos", ""); err != nil {
		t.Fatalf("writeInitialCLAUDEMD() error: %v", err)
	}

	// Platform info is in project-overview.md memory file
	data, err := os.ReadFile(filepath.Join(projectDir, ".claude", "memory", "project-overview.md"))
	if err != nil {
		t.Fatalf("failed to read project-overview.md: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "watchOS") || !strings.Contains(text, "Apple Watch") {
		t.Error("watchOS project-overview should mention watchOS/Apple Watch platform")
	}
	if !strings.Contains(text, "watchOS Simulator") {
		t.Error("watchOS project-overview should have watchOS Simulator in build command")
	}
}

func TestCanonicalBuildCommandWatchOS(t *testing.T) {
	cmd := canonicalBuildCommand("WatchApp", "watchos")
	if !strings.Contains(cmd, "watchOS Simulator") {
		t.Errorf("watchOS build command should use watchOS Simulator, got: %s", cmd)
	}
	if strings.Contains(cmd, "iOS Simulator") {
		t.Errorf("watchOS build command should not use iOS Simulator, got: %s", cmd)
	}
}

func TestCanonicalBuildCommandIOS(t *testing.T) {
	cmd := canonicalBuildCommand("IOSApp", "ios")
	if !strings.Contains(cmd, "iOS Simulator") {
		t.Errorf("iOS build command should use iOS Simulator, got: %s", cmd)
	}
}

func TestCanonicalBuildCommandPairedWatchUsesIOSDestination(t *testing.T) {
	cmd := canonicalBuildCommandForShape("WristCounter", "watchos", WatchShapePaired)
	if !strings.Contains(cmd, "iOS Simulator") {
		t.Errorf("paired watch build command should use iOS Simulator destination, got: %s", cmd)
	}
	if strings.Contains(cmd, "watchOS Simulator") {
		t.Errorf("paired watch build command should not use watchOS Simulator destination, got: %s", cmd)
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

func setupClaudeScriptTestProject(t *testing.T) string {
	t.Helper()
	projectDir := t.TempDir()
	if err := setupWorkspace(projectDir); err != nil {
		t.Fatalf("setupWorkspace() error: %v", err)
	}
	for _, dir := range []string{
		filepath.Join(projectDir, "App"),
		filepath.Join(projectDir, "Features"),
		filepath.Join(projectDir, "Targets"),
		filepath.Join(projectDir, "Shared"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}
	if err := writeClaudeScripts(projectDir, "SampleApp", "ios"); err != nil {
		t.Fatalf("writeClaudeScripts() error: %v", err)
	}
	return projectDir
}

func TestWriteAlwaysSkillsWatchOS(t *testing.T) {
	projectDir := t.TempDir()
	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("failed to create skills dir: %v", err)
	}

	if err := writeAlwaysSkills(projectDir, "watchos"); err != nil {
		t.Fatalf("writeAlwaysSkills(watchos) error: %v", err)
	}

	// Verify watchOS overrides are loaded instead of iOS versions
	for _, name := range []string{"layout", "navigation", "components"} {
		path := filepath.Join(skillsDir, name, "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("expected %s/SKILL.md to exist: %v", name, err)
			continue
		}
		text := string(data)
		if !strings.Contains(text, "watchOS") && !strings.Contains(text, "watch") {
			t.Errorf("%s/SKILL.md should contain watchOS-specific content, got iOS content", name)
		}
	}

	// Verify layout does NOT contain iOS-specific layout patterns
	layoutData, err := os.ReadFile(filepath.Join(skillsDir, "layout", "SKILL.md"))
	if err != nil {
		t.Fatalf("failed to read layout SKILL.md: %v", err)
	}
	if strings.Contains(string(layoutData), "GeometryReader {") {
		t.Error("watchOS layout should not recommend GeometryReader usage")
	}
	if strings.Contains(string(layoutData), "iPad") {
		t.Error("watchOS layout should not mention iPad")
	}
}

func TestWriteAlwaysSkillsWatchOSFallthrough(t *testing.T) {
	projectDir := t.TempDir()
	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("failed to create skills dir: %v", err)
	}

	if err := writeAlwaysSkills(projectDir, "watchos"); err != nil {
		t.Fatalf("writeAlwaysSkills(watchos) error: %v", err)
	}

	// design-system has no watchOS override — should fall through to always/
	path := filepath.Join(skillsDir, "design-system", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected design-system/SKILL.md to exist (fallthrough): %v", err)
	}
	if !strings.Contains(string(data), "AppTheme") {
		t.Error("design-system should contain AppTheme (loaded from always/)")
	}

	// swiftui/ multi-file skill has no watchOS override — should fall through
	swiftuiPath := filepath.Join(skillsDir, "swiftui", "SKILL.md")
	if _, err := os.Stat(swiftuiPath); err != nil {
		t.Errorf("expected swiftui/SKILL.md to exist (fallthrough): %v", err)
	}
}

func TestWriteAlwaysSkillsWatchOSOnly(t *testing.T) {
	projectDir := t.TempDir()
	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("failed to create skills dir: %v", err)
	}

	if err := writeAlwaysSkills(projectDir, "watchos"); err != nil {
		t.Fatalf("writeAlwaysSkills(watchos) error: %v", err)
	}

	// watchos-patterns is watchOS-only (no iOS equivalent)
	path := filepath.Join(skillsDir, "watchos-patterns", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected watchos-patterns/SKILL.md to exist: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "Digital Crown") {
		t.Error("watchos-patterns should mention Digital Crown")
	}
	if !strings.Contains(text, "isLuminanceReduced") {
		t.Error("watchos-patterns should mention Always On Display (isLuminanceReduced)")
	}
}

func TestWriteConditionalSkillsWatchOSOverride(t *testing.T) {
	projectDir := t.TempDir()
	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("failed to create skills dir: %v", err)
	}

	ruleKeys := []string{"haptics", "gestures", "biometrics", "widgets"}
	if err := writeConditionalSkills(projectDir, ruleKeys, "watchos"); err != nil {
		t.Fatalf("writeConditionalSkills(watchos) error: %v", err)
	}

	// haptics should come from watchos/ not features/
	hapticsData, err := os.ReadFile(filepath.Join(skillsDir, "haptics", "SKILL.md"))
	if err != nil {
		t.Fatalf("expected haptics/SKILL.md: %v", err)
	}
	text := string(hapticsData)
	if !strings.Contains(text, "WKInterfaceDevice") {
		t.Error("watchOS haptics should mention WKInterfaceDevice")
	}
	if strings.Contains(text, "UIImpactFeedbackGenerator") {
		t.Error("watchOS haptics should NOT mention UIImpactFeedbackGenerator")
	}

	// gestures should come from watchos/ not ui/
	gesturesData, err := os.ReadFile(filepath.Join(skillsDir, "gestures", "SKILL.md"))
	if err != nil {
		t.Fatalf("expected gestures/SKILL.md: %v", err)
	}
	if !strings.Contains(string(gesturesData), "Digital Crown") {
		t.Error("watchOS gestures should mention Digital Crown")
	}

	// widgets should come from watchos/ not extensions/
	widgetsData, err := os.ReadFile(filepath.Join(skillsDir, "widgets", "SKILL.md"))
	if err != nil {
		t.Fatalf("expected widgets/SKILL.md: %v", err)
	}
	if !strings.Contains(string(widgetsData), "accessoryCircular") {
		t.Error("watchOS widgets should mention accessoryCircular complication family")
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

func TestWriteAlwaysSkillsMultiPlatform(t *testing.T) {
	projectDir := t.TempDir()
	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("failed to create skills dir: %v", err)
	}

	// Multi-platform: iOS primary + watchOS + tvOS extras
	if err := writeAlwaysSkills(projectDir, "ios", "watchos", "tvos"); err != nil {
		t.Fatalf("writeAlwaysSkills(ios, watchos, tvos) error: %v", err)
	}

	// Base iOS always skills should be present
	for _, name := range []string{"design-system", "layout", "navigation", "components"} {
		path := filepath.Join(skillsDir, name, "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s/SKILL.md to exist: %v", name, err)
		}
	}

	// watchOS-only skill should be loaded
	path := filepath.Join(skillsDir, "watchos-patterns", "SKILL.md")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected watchos-patterns/SKILL.md to exist: %v", err)
	}

	// tvOS-only skill should be loaded
	tvosPath := filepath.Join(skillsDir, "tvos-patterns", "SKILL.md")
	if _, err := os.Stat(tvosPath); err != nil {
		t.Errorf("expected tvos-patterns/SKILL.md to exist: %v", err)
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
		if strings.Contains(cmd, "FocusFlow.xcodeproj") && strings.Contains(cmd, "iOS Simulator") {
			hasIOS = true
		}
		if strings.Contains(cmd, "FocusFlowTV") && strings.Contains(cmd, "tvOS Simulator") {
			hasTV = true
		}
	}
	if !hasIOS {
		t.Error("expected iOS build command")
	}
	if !hasTV {
		t.Error("expected tvOS build command")
	}
}

func TestWriteClaudeMemoryFilesMultiPlatform(t *testing.T) {
	projectDir := t.TempDir()
	if err := setupWorkspace(projectDir); err != nil {
		t.Fatalf("setupWorkspace() error: %v", err)
	}

	plan := &PlannerResult{
		Platform:          "ios",
		Platforms:         []string{"ios", "watchos", "tvos"},
		DeviceFamily:      "universal",
		WatchProjectShape: "paired_ios_watch",
	}

	if err := writeClaudeMemoryFiles(projectDir, "FocusFlow", "ios", "universal", plan); err != nil {
		t.Fatalf("writeClaudeMemoryFiles() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(projectDir, ".claude", "memory", "project-overview.md"))
	if err != nil {
		t.Fatalf("failed to read project-overview.md: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "iOS") || !strings.Contains(text, "watchOS") || !strings.Contains(text, "tvOS") {
		t.Error("multi-platform project-overview should list all platforms")
	}
	if !strings.Contains(text, "FocusFlowWatch") {
		t.Error("multi-platform project-overview should mention watch source dir")
	}
	if !strings.Contains(text, "FocusFlowTV") {
		t.Error("multi-platform project-overview should mention TV source dir")
	}
}

func TestWriteConditionalSkillsWatchOSFallback(t *testing.T) {
	projectDir := t.TempDir()
	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("failed to create skills dir: %v", err)
	}

	// storage has no watchOS override — should fall through to features/
	ruleKeys := []string{"storage"}
	if err := writeConditionalSkills(projectDir, ruleKeys, "watchos"); err != nil {
		t.Fatalf("writeConditionalSkills(watchos) error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(skillsDir, "storage", "SKILL.md"))
	if err != nil {
		t.Fatalf("expected storage/SKILL.md (fallback): %v", err)
	}
	if !strings.Contains(string(data), "SwiftData") {
		t.Error("storage fallback should contain SwiftData content from features/")
	}
}

func runClaudeScript(t *testing.T, projectDir, scriptName string, args ...string) (int, string) {
	t.Helper()
	cmdArgs := []string{filepath.Join(projectDir, "scripts", "claude", scriptName)}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("sh", cmdArgs...)
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(output)
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), string(output)
	}
	t.Fatalf("failed to run script %s: %v\n%s", scriptName, err, string(output))
	return 0, ""
}
