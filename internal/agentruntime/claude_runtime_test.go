package agentruntime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeRuntimeReadsModelsFromStatsCache(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	stats := `{
  "dailyStats": [
    {
      "date": "2026-03-14",
      "tokensByModel": {
        "claude-opus-4-6": 3200,
        "claude-sonnet-4-5-20250929": 900
      }
    }
  ],
  "modelUsage": {
    "claude-opus-4-6": {
      "inputTokens": 3200,
      "outputTokens": 1200,
      "cacheReadInputTokens": 0,
      "cacheCreationInputTokens": 0
    },
    "claude-sonnet-4-5-20250929": {
      "inputTokens": 900,
      "outputTokens": 400,
      "cacheReadInputTokens": 0,
      "cacheCreationInputTokens": 0
    },
    "claude-haiku-4-5-20251001": {
      "inputTokens": 300,
      "outputTokens": 100,
      "cacheReadInputTokens": 0,
      "cacheCreationInputTokens": 0
    }
  }
}`
	if err := os.WriteFile(filepath.Join(claudeDir, "stats-cache.json"), []byte(stats), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("HOME", home)

	runtime := NewClaude("claude")
	if got := runtime.DefaultModel(PhaseBuild); got != "claude-opus-4-6" {
		t.Fatalf("DefaultModel() = %q, want %q", got, "claude-opus-4-6")
	}

	models := runtime.SuggestedModels()
	want := []string{
		"claude-opus-4-6",
		"claude-sonnet-4-5-20250929",
		"claude-haiku-4-5-20251001",
	}
	if len(models) != len(want) {
		t.Fatalf("len(SuggestedModels()) = %d, want %d", len(models), len(want))
	}
	for i, id := range want {
		if models[i].ID != id {
			t.Fatalf("SuggestedModels()[%d] = %q, want %q", i, models[i].ID, id)
		}
	}
}

func TestClaudeRuntimeReadsModelsFromTelemetry(t *testing.T) {
	home := t.TempDir()
	telemetryDir := filepath.Join(home, ".claude", "telemetry")
	if err := os.MkdirAll(telemetryDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	telemetry := `[
  {"event_data":{"model":"claude-opus-4-6"}},
  {"event_data":{"model":"claude-opus-4-6"}},
  {"event_data":{"model":"claude-sonnet-4-5-20250929"}},
  {"event_data":{"model":"claude-code-20250219"}}
]`
	if err := os.WriteFile(filepath.Join(telemetryDir, "recent.json"), []byte(telemetry), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("HOME", home)

	runtime := NewClaude("claude")
	models := runtime.SuggestedModels()
	want := []string{
		"claude-opus-4-6",
		"claude-sonnet-4-5-20250929",
	}
	if len(models) != len(want) {
		t.Fatalf("len(SuggestedModels()) = %d, want %d", len(models), len(want))
	}
	for i, id := range want {
		if models[i].ID != id {
			t.Fatalf("SuggestedModels()[%d] = %q, want %q", i, models[i].ID, id)
		}
	}
	if got := runtime.DefaultModel(PhaseBuild); got != "claude-opus-4-6" {
		t.Fatalf("DefaultModel() = %q, want %q", got, "claude-opus-4-6")
	}
}

func TestClaudeRuntimeFallsBackToHelpAliases(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	scriptPath := filepath.Join(home, "claude")
	script := "#!/bin/sh\nif [ \"$1\" = \"--help\" ]; then\ncat <<'EOF'\n--model <model> Provide an alias for the latest model (e.g. 'sonnet' or 'opus') or a model's full name (e.g. 'claude-sonnet-4-6').\nEOF\nfi\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("HOME", home)

	runtime := NewClaude(scriptPath)
	models := runtime.SuggestedModels()
	want := []string{"sonnet", "opus", "claude-sonnet-4-6"}
	if len(models) != len(want) {
		t.Fatalf("len(SuggestedModels()) = %d, want %d", len(models), len(want))
	}
	for i, id := range want {
		if models[i].ID != id {
			t.Fatalf("SuggestedModels()[%d] = %q, want %q", i, models[i].ID, id)
		}
	}
	if got := runtime.DefaultModel(PhaseBuild); got != "sonnet" {
		t.Fatalf("DefaultModel() = %q, want %q", got, "sonnet")
	}
}
