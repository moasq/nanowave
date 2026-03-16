package orchestration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/moasq/nanowave/internal/agentruntime"
	"github.com/moasq/nanowave/internal/config"
	"github.com/moasq/nanowave/internal/integrations"
	"github.com/moasq/nanowave/internal/mcpregistry"
	"github.com/moasq/nanowave/internal/terminal"
)

// DetectProjectBuildHints reads project_config.json and extracts build-relevant hints.
// Returns ("ios", nil, "") when missing or unreadable (backward compat).
func DetectProjectBuildHints(projectDir string) (platform string, platforms []string, watchProjectShape string) {
	data, err := os.ReadFile(filepath.Join(projectDir, "project_config.json"))
	if err != nil {
		return PlatformIOS, nil, ""
	}
	var cfg struct {
		Platform          string   `json:"platform"`
		Platforms         []string `json:"platforms"`
		WatchProjectShape string   `json:"watch_project_shape"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return PlatformIOS, nil, ""
	}
	if cfg.Platform == "" {
		cfg.Platform = PlatformIOS
	}
	return cfg.Platform, cfg.Platforms, cfg.WatchProjectShape
}

// ReadProjectAppName returns the Xcode app name for an existing project.
// It reads app_name from project_config.json (the canonical source of truth written at build time).
// Falls back to filepath.Base(projectDir) for projects predating the suffixed-dir feature.
func ReadProjectAppName(projectDir string) string {
	data, err := os.ReadFile(filepath.Join(projectDir, "project_config.json"))
	if err != nil {
		return filepath.Base(projectDir)
	}
	var cfg struct {
		AppName string `json:"app_name"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil || cfg.AppName == "" {
		return filepath.Base(projectDir)
	}
	return cfg.AppName
}

// coreAgenticTools is the set of non-MCP tools the supported agent runtimes need.
// MCP tools (apple-docs, xcodegen) are provided by the registry.
var coreAgenticTools = []string{
	"Write", "Edit", "Read", "Bash", "Glob", "Grep",
	"WebFetch", "WebSearch",
}

// ActionContext provides context for the unified pipeline.
// For new builds, all fields are zero-valued.
// For edits to existing projects, fields carry existing project info.
type ActionContext struct {
	ProjectDir        string   // empty for new builds
	AppName           string   // empty for new builds (analyzer will name it)
	SessionID         string   // for conversation continuity
	Platform          string   // detected from existing project_config.json
	Platforms         []string // detected from existing project_config.json
	WatchProjectShape string   // detected from existing project_config.json
}

// IsEdit returns true when acting on an existing project.
func (ac ActionContext) IsEdit() bool {
	return ac.ProjectDir != ""
}

// Pipeline orchestrates the multi-phase app generation process.
type Pipeline struct {
	runtime         agentruntime.Runtime
	runtimeKind     agentruntime.Kind
	config          *config.Config
	model           string                         // user-selected model for code generation
	manager         *integrations.Manager          // provider-based integration manager (nil = no integrations)
	registry        *mcpregistry.Registry          // internal MCP server registry (apple-docs, xcodegen)
	activeProviders []integrations.ActiveProvider  // resolved providers for current build (transient)
	onStreamEvent   func(agentruntime.StreamEvent) // optional hook for web UI streaming (nil = CLI-only)
}

// SetManager sets the integration manager for provider-based integrations.
func (p *Pipeline) SetManager(m *integrations.Manager) {
	p.manager = m
}

// SetStreamHook sets an optional callback invoked for every streaming event.
// Used by the web UI to mirror CLI progress in the browser.
func (p *Pipeline) SetStreamHook(hook func(agentruntime.StreamEvent)) {
	p.onStreamEvent = hook
}

func (p *Pipeline) progressRuntimeLabel() string {
	label := ""
	if p.runtime != nil {
		label = strings.TrimSpace(p.runtime.DisplayName())
	}
	if label == "" {
		label = strings.TrimSpace(agentruntime.DescriptorForKind(p.runtimeKind).DisplayName)
	}
	return strings.TrimSpace(strings.TrimSuffix(label, " Code"))
}

func runtimeReadyActivityLabel(runtimeLabel string) string {
	runtimeLabel = strings.TrimSpace(runtimeLabel)
	if runtimeLabel == "" {
		return "Runtime ready"
	}
	return runtimeLabel + " ready"
}

func (p *Pipeline) newProgressDisplay(mode string, totalFiles int) *terminal.ProgressDisplay {
	progress := terminal.NewProgressDisplay(mode, totalFiles)
	progress.SetRuntimeLabel(p.progressRuntimeLabel())
	return progress
}

// makeStreamCallback wraps the terminal progress callback and the optional web hook.
func (p *Pipeline) makeStreamCallback(progress *terminal.ProgressDisplay) func(agentruntime.StreamEvent) {
	termCb := newProgressCallback(progress, p.progressRuntimeLabel())
	if p.onStreamEvent == nil {
		return termCb
	}
	hook := p.onStreamEvent
	return func(ev agentruntime.StreamEvent) {
		termCb(ev)
		hook(ev)
	}
}

// NewPipeline creates a new pipeline orchestrator.
func NewPipeline(runtimeClient agentruntime.Runtime, runtimeKind agentruntime.Kind, cfg *config.Config, model string) *Pipeline {
	reg := mcpregistry.New()
	mcpregistry.RegisterAll(reg)
	return &Pipeline{
		runtime:     runtimeClient,
		runtimeKind: runtimeKind,
		config:      cfg,
		model:       model,
		registry:    reg,
	}
}

// baseAgenticTools returns core tools plus all MCP tools from the registry.
func (p *Pipeline) baseAgenticTools() []string {
	tools := make([]string, len(coreAgenticTools))
	copy(tools, coreAgenticTools)
	tools = append(tools, p.registry.AllTools()...)
	return tools
}

func (p *Pipeline) modelForPhase(phase agentruntime.Phase) string {
	if p.model != "" {
		return p.model
	}
	if p.runtime == nil {
		return ""
	}
	return p.runtime.DefaultModel(phase)
}

