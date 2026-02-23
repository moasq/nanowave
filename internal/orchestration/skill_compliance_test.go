package orchestration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceSkillsAnthropicComplianceStrict(t *testing.T) {
	report, err := ValidateAnthropicSourceSkills(os.DirFS("."), "skills", true)
	if err != nil {
		t.Fatalf("ValidateAnthropicSourceSkills() error: %v", err)
	}
	for _, note := range report.Notes {
		t.Log(note)
	}
	t.Logf("validated %d skills and %d reference files", report.SkillsChecked, report.ReferenceFilesChecked)

	if report.HasIssues() {
		var b strings.Builder
		b.WriteString("source skill compliance issues:\n")
		for _, issue := range report.Issues {
			b.WriteString("- ")
			b.WriteString(issue.String())
			b.WriteString("\n")
		}
		t.Fatal(b.String())
	}
}

func TestValidateAnthropicSourceSkillsStrictFixtureFailures(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(t *testing.T, root string)
		wantCodes []string
	}{
		{
			name: "extra frontmatter key",
			mutate: func(t *testing.T, root string) {
				p := filepath.Join(root, "skills", "features", "camera", "SKILL.md")
				data, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				text := strings.Replace(string(data), "description:", "user-invocable: false\ndescription:", 1)
				if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantCodes: []string{"unsupported_frontmatter_field"},
		},
		{
			name: "invalid name",
			mutate: func(t *testing.T, root string) {
				p := filepath.Join(root, "skills", "features", "camera", "SKILL.md")
				replaceInFile(t, p, `name: "camera"`, `name: "Camera_Invalid"`)
			},
			wantCodes: []string{"invalid_skill_name"},
		},
		{
			name: "missing use when clause",
			mutate: func(t *testing.T, root string) {
				p := filepath.Join(root, "skills", "features", "camera", "SKILL.md")
				replaceInFile(t, p, " Use when implementing app features related to camera.", "")
			},
			wantCodes: []string{"description_missing_use_when"},
		},
		{
			name: "description too long",
			mutate: func(t *testing.T, root string) {
				p := filepath.Join(root, "skills", "features", "camera", "SKILL.md")
				longDesc := strings.Repeat("x", 1030) + " Use when testing."
				replaceLinePrefix(t, p, "description:", fmt.Sprintf(`description: %q`, longDesc))
			},
			wantCodes: []string{"description_too_long"},
		},
		{
			name: "skill body over 500 lines",
			mutate: func(t *testing.T, root string) {
				p := filepath.Join(root, "skills", "features", "camera", "SKILL.md")
				data, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				fmEnd := strings.Index(string(data), "\n---\n")
				if fmEnd < 0 {
					t.Fatal("missing frontmatter")
				}
				prefix := string(data)[:fmEnd+len("\n---\n")]
				var body strings.Builder
				body.WriteString("# Camera\n\n")
				for i := 0; i < 501; i++ {
					body.WriteString("- line\n")
				}
				if err := os.WriteFile(p, []byte(prefix+body.String()), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantCodes: []string{"skill_body_too_long"},
		},
		{
			name: "missing linked reference file",
			mutate: func(t *testing.T, root string) {
				p := filepath.Join(root, "skills", "always", "guide", "reference", "guide.md")
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
			},
			wantCodes: []string{"broken_local_link"},
		},
		{
			name: "nested reference link",
			mutate: func(t *testing.T, root string) {
				p := filepath.Join(root, "skills", "always", "guide", "reference", "guide.md")
				replaceInFile(t, p, "## Section\n", "## Section\n\nSee [extra](other.md).\n")
				other := filepath.Join(root, "skills", "always", "guide", "reference", "other.md")
				if err := os.WriteFile(other, []byte("# Other\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantCodes: []string{"nested_reference_markdown_link"},
		},
		{
			name: "long reference without toc",
			mutate: func(t *testing.T, root string) {
				p := filepath.Join(root, "skills", "always", "guide", "reference", "guide.md")
				var b strings.Builder
				b.WriteString("# Guide\n\n")
				for i := 0; i < 105; i++ {
					b.WriteString("Line\n")
				}
				if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantCodes: []string{"long_reference_missing_toc"},
		},
		{
			name: "non-directory skill category entry",
			mutate: func(t *testing.T, root string) {
				p := filepath.Join(root, "skills", "ui", "oops.txt")
				if err := os.WriteFile(p, []byte("oops"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantCodes: []string{"non_directory_skill_entry"},
		},
		{
			name: "local link uses backslashes",
			mutate: func(t *testing.T, root string) {
				p := filepath.Join(root, "skills", "always", "guide", "SKILL.md")
				replaceInFile(t, p, "(reference/guide.md)", `(reference\guide.md)`)
			},
			wantCodes: []string{"local_link_must_use_forward_slashes"},
		},
		{
			name: "core rule excluded from anthropic skill checks",
			mutate: func(t *testing.T, root string) {
				p := filepath.Join(root, "skills", "core", "bad.md")
				if err := os.WriteFile(p, []byte("not a skill and intentionally malformed"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantCodes: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeValidSourceSkillsFixture(t, root)
			tc.mutate(t, root)

			report, err := ValidateAnthropicSourceSkills(os.DirFS(root), "skills", true)
			if err != nil {
				t.Fatalf("ValidateAnthropicSourceSkills() error: %v", err)
			}

			if tc.wantCodes == nil {
				if report.HasIssues() {
					t.Fatalf("expected no issues, got %v", issueCodes(report.Issues))
				}
				if len(report.Notes) == 0 || !strings.Contains(strings.ToLower(strings.Join(report.Notes, " ")), "core rules intentionally excluded") {
					t.Fatalf("expected core exclusion note, got %v", report.Notes)
				}
				return
			}

			gotCodes := issueCodes(report.Issues)
			for _, want := range tc.wantCodes {
				if !containsString(gotCodes, want) {
					t.Fatalf("expected issue code %q, got %v", want, gotCodes)
				}
			}
		})
	}
}

func writeValidSourceSkillsFixture(t *testing.T, root string) {
	t.Helper()
	cats := []string{"always", "always-watchos", "always-tvos", "features", "ui", "extensions", "watchos", "tvos", "phases"}
	for _, cat := range cats {
		skillDirName := "sample-" + cat
		skillName := "sample-" + strings.ReplaceAll(cat, "_", "-")
		desc := "Provides fixture guidance. Use when validating " + cat + " skills."
		title := titleize(cat)
		if cat == "features" {
			skillDirName = "camera"
			skillName = "camera"
			desc = "Camera fixture guidance. Use when implementing app features related to camera."
			title = "Camera"
		}

		skillDir := filepath.Join(root, "skills", cat, skillDirName)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", skillDir, err)
		}
		skillMD := fmt.Sprintf(`---
name: %q
description: %q
---
# %s

Base skill body.
`, skillName, desc, title)
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
			t.Fatalf("write skill: %v", err)
		}
	}

	// A multi-file skill used by fixture mutations.
	guideDir := filepath.Join(root, "skills", "always", "guide")
	if err := os.MkdirAll(filepath.Join(guideDir, "reference"), 0o755); err != nil {
		t.Fatalf("mkdir guide: %v", err)
	}
	guideSkill := `---
name: "guide"
description: "Provides fixture guide navigation. Use when validating reference links."
---
# Guide

See [guide reference](reference/guide.md).
`
	if err := os.WriteFile(filepath.Join(guideDir, "SKILL.md"), []byte(guideSkill), 0o644); err != nil {
		t.Fatalf("write guide skill: %v", err)
	}
	guideRef := `# Guide Reference

## Contents
- [Section](#section)

## Section
Reference content.
`
	if err := os.WriteFile(filepath.Join(guideDir, "reference", "guide.md"), []byte(guideRef), 0o644); err != nil {
		t.Fatalf("write guide ref: %v", err)
	}

	coreDir := filepath.Join(root, "skills", "core")
	if err := os.MkdirAll(coreDir, 0o755); err != nil {
		t.Fatalf("mkdir core: %v", err)
	}
	coreRule := `---
description: "Core rule fixture"
---
# Core Rule
`
	if err := os.WriteFile(filepath.Join(coreDir, "file-structure.md"), []byte(coreRule), 0o644); err != nil {
		t.Fatalf("write core rule: %v", err)
	}
}

func issueCodes(issues []SourceSkillComplianceIssue) []string {
	codes := make([]string, 0, len(issues))
	for _, issue := range issues {
		codes = append(codes, issue.Code)
	}
	return codes
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func replaceInFile(t *testing.T, path, old, new string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, old) {
		t.Fatalf("replaceInFile: %q not found in %s", old, path)
	}
	text = strings.Replace(text, old, new, 1)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func replaceLinePrefix(t *testing.T, path, prefix, replacement string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[i] = replacement
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("replaceLinePrefix: prefix %q not found in %s", prefix, path)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func titleize(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	parts := strings.Fields(s)
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}
