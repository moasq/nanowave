package orchestration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePlannedFilePath(t *testing.T) {
	projectDir := t.TempDir()
	appName := "MyApp"

	// Create the files so os.Stat can find them
	filesToCreate := []string{
		filepath.Join(projectDir, "Targets", "MyWidget", "Widget.swift"),
		filepath.Join(projectDir, "Shared", "ActivityAttributes.swift"),
	}
	for _, f := range filesToCreate {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		if err := os.WriteFile(f, []byte("// test"), 0o644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	tests := []struct {
		name       string
		planned    string
		wantSuffix string
	}{
		{
			name:       "app file (not on disk) falls back to appName prefix",
			planned:    "Models/Meal.swift",
			wantSuffix: filepath.Join("MyApp", "Models", "Meal.swift"),
		},
		{
			name:       "targets file found directly",
			planned:    "Targets/MyWidget/Widget.swift",
			wantSuffix: filepath.Join("Targets", "MyWidget", "Widget.swift"),
		},
		{
			name:       "shared file found directly",
			planned:    "Shared/ActivityAttributes.swift",
			wantSuffix: filepath.Join("Shared", "ActivityAttributes.swift"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolvePlannedFilePath(projectDir, appName, tc.planned)
			want := filepath.Join(projectDir, tc.wantSuffix)
			if got != want {
				t.Fatalf("resolvePlannedFilePath() = %q, want %q", got, want)
			}
		})
	}
}

func TestVerifyPlannedFilesMissingFile(t *testing.T) {
	projectDir := t.TempDir()
	plan := &PlannerResult{
		Files: []FilePlan{
			{Path: "Models/Meal.swift", TypeName: "Meal"},
		},
	}

	report, err := verifyPlannedFiles(projectDir, "MyApp", plan)
	if err != nil {
		t.Fatalf("verifyPlannedFiles() returned error: %v", err)
	}
	if report.Complete {
		t.Fatalf("expected incomplete report")
	}
	if report.TotalPlanned != 1 || report.ValidCount != 0 {
		t.Fatalf("unexpected totals: %+v", report)
	}
	if len(report.Missing) != 1 {
		t.Fatalf("expected 1 missing file, got %d", len(report.Missing))
	}
	if len(report.Invalid) != 0 {
		t.Fatalf("expected 0 invalid files, got %d", len(report.Invalid))
	}
}

func TestVerifyPlannedFilesValidationRules(t *testing.T) {
	tests := []struct {
		name              string
		content           string
		wantComplete      bool
		wantInvalidReason string
	}{
		{
			name:              "empty file is invalid",
			content:           " \n\t",
			wantComplete:      false,
			wantInvalidReason: "empty",
		},
		{
			name:              "placeholder only is invalid",
			content:           "// Placeholder - replaced by generated code\nimport Foundation\n",
			wantComplete:      false,
			wantInvalidReason: "placeholder",
		},
		{
			name:              "missing type token is invalid",
			content:           "import Foundation\nstruct OtherType {}\n",
			wantComplete:      false,
			wantInvalidReason: "missing expected type",
		},
		{
			name:         "valid file passes",
			content:      "import Foundation\nstruct Meal {}\n",
			wantComplete: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			projectDir := t.TempDir()
			filePath := filepath.Join(projectDir, "MyApp", "Models", "Meal.swift")
			if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
				t.Fatalf("failed to create directories: %v", err)
			}
			if err := os.WriteFile(filePath, []byte(tc.content), 0o644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			plan := &PlannerResult{
				Files: []FilePlan{
					{Path: "Models/Meal.swift", TypeName: "Meal"},
				},
			}

			report, err := verifyPlannedFiles(projectDir, "MyApp", plan)
			if err != nil {
				t.Fatalf("verifyPlannedFiles() returned error: %v", err)
			}

			if report.Complete != tc.wantComplete {
				t.Fatalf("report.Complete = %v, want %v (report=%+v)", report.Complete, tc.wantComplete, report)
			}

			if tc.wantComplete {
				if report.ValidCount != 1 || len(report.Invalid) != 0 || len(report.Missing) != 0 {
					t.Fatalf("expected 1 valid file and no unresolved files, got %+v", report)
				}
				return
			}

			if len(report.Invalid) != 1 {
				t.Fatalf("expected 1 invalid file, got %d", len(report.Invalid))
			}
			if !strings.Contains(strings.ToLower(report.Invalid[0].Reason), strings.ToLower(tc.wantInvalidReason)) {
				t.Fatalf("expected invalid reason to contain %q, got %q", tc.wantInvalidReason, report.Invalid[0].Reason)
			}
		})
	}
}

func TestShouldRetryCompletion(t *testing.T) {
	t.Run("complete report does not retry", func(t *testing.T) {
		report := &FileCompletionReport{Complete: true}
		retry, err := shouldRetryCompletion(report, 1, 6)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if retry {
			t.Fatalf("expected retry=false")
		}
	})

	t.Run("incomplete report retries before limit", func(t *testing.T) {
		report := &FileCompletionReport{
			Complete: false,
			Missing:  []PlannedFileStatus{{PlannedPath: "Models/Meal.swift", Reason: "file does not exist"}},
		}
		retry, err := shouldRetryCompletion(report, 2, 6)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !retry {
			t.Fatalf("expected retry=true")
		}
	})

	t.Run("incomplete report fails at retry cap", func(t *testing.T) {
		report := &FileCompletionReport{
			Complete: false,
			Invalid:  []PlannedFileStatus{{PlannedPath: "Models/Meal.swift", Reason: "missing expected type \"Meal\""}},
		}
		retry, err := shouldRetryCompletion(report, 6, 6)
		if err == nil {
			t.Fatalf("expected error at retry cap")
		}
		if retry {
			t.Fatalf("expected retry=false at retry cap")
		}
		if !strings.Contains(err.Error(), "Models/Meal.swift") {
			t.Fatalf("expected error to include unresolved file path, got %q", err.Error())
		}
	})
}
