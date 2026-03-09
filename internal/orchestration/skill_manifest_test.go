package orchestration

import (
	"testing"
)

func TestGenerateSkillManifest(t *testing.T) {
	manifest, err := GenerateSkillManifest()
	if err != nil {
		t.Fatalf("GenerateSkillManifest() error: %v", err)
	}
	if manifest.Version != "1" {
		t.Errorf("expected version 1, got %s", manifest.Version)
	}
	if len(manifest.Skills) == 0 {
		t.Fatal("expected at least one skill in manifest")
	}

	// Verify every skill has a name and description
	for _, skill := range manifest.Skills {
		if skill.Name == "" {
			t.Errorf("skill at path %s has empty name", skill.Path)
		}
		if skill.Category == "" {
			t.Errorf("skill %s has empty category", skill.Name)
		}
	}

	// Verify JSON serialization works
	data, err := manifest.MarshalManifestJSON()
	if err != nil {
		t.Fatalf("MarshalManifestJSON() error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty JSON output")
	}
	t.Logf("manifest: %d skills, %d bytes JSON", len(manifest.Skills), len(data))
}

func TestSkillsForContext(t *testing.T) {
	manifest, err := GenerateSkillManifest()
	if err != nil {
		t.Fatalf("GenerateSkillManifest() error: %v", err)
	}

	// Test with iOS platform and some feature keys
	skills := manifest.SkillsForContext("ios", []string{"camera", "dark-mode", "animations"})
	if len(skills) == 0 {
		t.Fatal("expected at least one skill for ios context")
	}

	// Verify always-loaded skills are included
	hasAlwaysSkill := false
	for _, s := range skills {
		if s.AlwaysLoad {
			hasAlwaysSkill = true
			break
		}
	}
	if !hasAlwaysSkill {
		t.Error("expected always-loaded skills to be included in context")
	}

	// Verify feature keys are matched
	hasCamera := false
	for _, s := range skills {
		if s.Name == "camera" {
			hasCamera = true
			break
		}
	}
	if !hasCamera {
		t.Error("expected camera skill to be matched by feature key")
	}
}

func TestFormatSkillsForLLM(t *testing.T) {
	manifest, err := GenerateSkillManifest()
	if err != nil {
		t.Fatalf("GenerateSkillManifest() error: %v", err)
	}

	skills := manifest.SkillsForContext("ios", []string{"camera"})
	content := FormatSkillsForLLM(skills)
	if content == "" {
		t.Fatal("expected non-empty formatted skills content")
	}
	t.Logf("formatted %d skills into %d bytes", len(skills), len(content))
}

func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"tag1, tag2", []string{"tag1", "tag2"}},
		{"tag1,tag2,tag3", []string{"tag1", "tag2", "tag3"}},
		{"[tag1, tag2]", []string{"tag1", "tag2"}},
		{`"tag1", "tag2"`, []string{"tag1", "tag2"}},
		{"", nil},
	}

	for _, tc := range tests {
		result := parseCommaSeparated(tc.input)
		if len(result) != len(tc.expected) {
			t.Errorf("parseCommaSeparated(%q) = %v, want %v", tc.input, result, tc.expected)
			continue
		}
		for i, v := range result {
			if v != tc.expected[i] {
				t.Errorf("parseCommaSeparated(%q)[%d] = %q, want %q", tc.input, i, v, tc.expected[i])
			}
		}
	}
}
