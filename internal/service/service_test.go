package service

import (
	"context"
	"testing"

	"github.com/moasq/nanowave/internal/agentruntime"
	"github.com/moasq/nanowave/internal/config"
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

func TestRuntimeModelsIncludesCurrentModelWhenDiscoveryIsEmpty(t *testing.T) {
	svc := &Service{
		config:      &config.Config{},
		runtime:     stubRuntime{},
		runtimeKind: agentruntime.KindClaude,
		model:       "claude-opus-4-6",
	}

	models := svc.RuntimeModels()
	if len(models) != 1 {
		t.Fatalf("len(RuntimeModels()) = %d, want 1", len(models))
	}
	if got := models[0].ID; got != "claude-opus-4-6" {
		t.Fatalf("RuntimeModels()[0].ID = %q, want %q", got, "claude-opus-4-6")
	}
}

type stubRuntime struct{}

func (stubRuntime) Kind() agentruntime.Kind { return agentruntime.KindClaude }
func (stubRuntime) DisplayName() string     { return "Claude Code" }
func (stubRuntime) Descriptor() agentruntime.Descriptor {
	return agentruntime.DescriptorForKind(agentruntime.KindClaude)
}
func (stubRuntime) BinaryPath() string { return "claude" }
func (stubRuntime) Version() string    { return "" }
func (stubRuntime) AuthStatus() *agentruntime.AuthStatus {
	return nil
}
func (stubRuntime) DefaultModel(agentruntime.Phase) string { return "sonnet" }
func (stubRuntime) SuggestedModels() []agentruntime.ModelOption {
	return nil
}
func (stubRuntime) SupportsInteractive() bool { return true }
func (stubRuntime) GenerateStreaming(context.Context, string, agentruntime.GenerateOpts, func(agentruntime.StreamEvent)) (*agentruntime.Response, error) {
	return nil, nil
}
func (stubRuntime) Generate(context.Context, string, agentruntime.GenerateOpts) (*agentruntime.Response, error) {
	return nil, nil
}
func (stubRuntime) RunInteractive(context.Context, string, agentruntime.InteractiveOpts, func(agentruntime.StreamEvent), func(string) string) (*agentruntime.Response, error) {
	return nil, nil
}
