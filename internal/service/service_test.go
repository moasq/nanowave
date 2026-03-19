package service

import (
	"testing"

	"github.com/moasq/nanowave/internal/agentruntime"
)

func TestPlatformBundleIDSuffix(t *testing.T) {
	tests := []struct {
		platform string
		want     string
	}{
		{"ios", ""},
		{"watchos", ""},
		{"tvos", ".tv"},
		{"visionos", ".vision"},
		{"macos", ".mac"},
	}
	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			got := platformBundleIDSuffix(tt.platform)
			if got != tt.want {
				t.Errorf("platformBundleIDSuffix(%q) = %q, want %q", tt.platform, got, tt.want)
			}
		})
	}
}

func TestRuntimeSupportsModel(t *testing.T) {
	models := []struct {
		id string
	}{
		{id: "gpt-5-codex"},
		{id: "openai/gpt-5-codex"},
	}

	opts := make([]agentruntime.ModelOption, 0, len(models))
	for _, model := range models {
		opts = append(opts, agentruntime.ModelOption{ID: model.id})
	}

	if !runtimeSupportsModel(opts, "gpt-5-codex") {
		t.Fatal("runtimeSupportsModel() should accept an exact model match")
	}
	if runtimeSupportsModel(opts, "gpt-5") {
		t.Fatal("runtimeSupportsModel() should reject a model not offered by the runtime")
	}
}
