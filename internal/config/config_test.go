package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/moasq/nanowave/internal/agentruntime"
)

func TestSaveRuntimePreferencesPreservesConfiguredModels(t *testing.T) {
	root := t.TempDir()
	initial := runtimePreferences{
		RuntimeKind: "codex",
		Model:       "gpt-5.4",
		DefaultModels: map[string]string{
			"codex": "gpt-5.4",
		},
		RuntimeModels: map[string][]runtimeModelPreference{
			"codex": {
				{ID: "gpt-5.4", Description: "Current default"},
				{ID: "o3", Description: "Reasoning"},
			},
		},
	}
	data, err := json.MarshalIndent(initial, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "config.json"), data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg := &Config{NanowaveRoot: root}
	if err := cfg.SaveRuntimePreferences(agentruntime.KindClaude, "opus"); err != nil {
		t.Fatalf("SaveRuntimePreferences() error = %v", err)
	}

	prefs := loadRuntimePreferences(root)
	if got := prefs.defaultModelFor(agentruntime.KindClaude); got != "opus" {
		t.Fatalf("defaultModelFor(claude) = %q, want %q", got, "opus")
	}
	if got := prefs.defaultModelFor(agentruntime.KindCodex); got != "gpt-5.4" {
		t.Fatalf("defaultModelFor(codex) = %q, want %q", got, "gpt-5.4")
	}
	if len(prefs.RuntimeModels["codex"]) != 2 {
		t.Fatalf("len(RuntimeModels[codex]) = %d, want 2", len(prefs.RuntimeModels["codex"]))
	}
}

func TestRuntimeModelOptionsMergesConfiguredAndDiscoveredModels(t *testing.T) {
	root := t.TempDir()
	prefs := runtimePreferences{
		RuntimeKind: "codex",
		Model:       "gpt-5.4",
		DefaultModels: map[string]string{
			"codex": "gpt-5.4",
		},
		RuntimeModels: map[string][]runtimeModelPreference{
			"codex": {
				{ID: "gpt-5.4", Description: "Configured default"},
				{ID: "o4-mini", Description: "Configured lightweight model"},
			},
		},
	}
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "config.json"), data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg := &Config{NanowaveRoot: root}
	models := cfg.RuntimeModelOptions(agentruntime.KindCodex, []agentruntime.ModelOption{
		{ID: "gpt-5.4", Description: "Discovered from Codex config"},
		{ID: "gpt-5.5", Description: "Discovered next model"},
	})

	want := []string{"gpt-5.4", "o4-mini", "gpt-5.5"}
	if len(models) != len(want) {
		t.Fatalf("len(RuntimeModelOptions()) = %d, want %d", len(models), len(want))
	}
	for i, id := range want {
		if models[i].ID != id {
			t.Fatalf("RuntimeModelOptions()[%d] = %q, want %q", i, models[i].ID, id)
		}
	}
	if models[0].Description != "Configured default" {
		t.Fatalf("RuntimeModelOptions()[0].Description = %q, want %q", models[0].Description, "Configured default")
	}
}

func TestLoadRuntimePreferencesAcceptsStringModelEntries(t *testing.T) {
	root := t.TempDir()
	raw := `{
  "runtime_kind": "codex",
  "runtime_models": {
    "codex": ["gpt-5.4", "o3"]
  }
}`
	if err := os.WriteFile(filepath.Join(root, "config.json"), []byte(raw), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	prefs := loadRuntimePreferences(root)
	if len(prefs.RuntimeModels["codex"]) != 2 {
		t.Fatalf("len(RuntimeModels[codex]) = %d, want 2", len(prefs.RuntimeModels["codex"]))
	}
	if prefs.RuntimeModels["codex"][0].ID != "gpt-5.4" {
		t.Fatalf("RuntimeModels[codex][0].ID = %q, want %q", prefs.RuntimeModels["codex"][0].ID, "gpt-5.4")
	}
}
