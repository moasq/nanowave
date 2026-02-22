package orchestration

import (
	"os"
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

	if err := writeAlwaysSkills(projectDir); err != nil {
		t.Fatalf("writeAlwaysSkills() error: %v", err)
	}

	// Verify flat skills are wrapped into SKILL.md
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

	// Verify multi-file skill (swiftui/) is copied as directory
	swiftuiFiles := []string{"SKILL.md", "animations.md", "forms.md", "lists.md", "scroll.md", "state.md", "performance.md", "text.md", "media.md", "modern-apis.md", "liquid-glass.md"}
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

	// Verify SKILL.md has frontmatter
	skillMD, err := os.ReadFile(filepath.Join(skillsDir, "design-system", "SKILL.md"))
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}
	if !strings.HasPrefix(string(skillMD), "---") {
		t.Error("SKILL.md should have YAML frontmatter")
	}
	if !strings.Contains(string(skillMD), "user-invocable: false") {
		t.Error("SKILL.md should have user-invocable: false")
	}
}

func TestWriteConditionalSkills(t *testing.T) {
	projectDir := t.TempDir()
	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("failed to create skills dir: %v", err)
	}

	ruleKeys := []string{"camera", "localization", "gestures"}
	if err := writeConditionalSkills(projectDir, ruleKeys); err != nil {
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

	if err := writeConditionalSkills(projectDir, nil); err != nil {
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
			name:    "adaptive_layout loads NavigationSplitView",
			key:     "adaptive_layout",
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
