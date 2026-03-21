package service

import (
	"fmt"
	"strings"

	"github.com/moasq/nanowave/internal/agentruntime"
	"github.com/moasq/nanowave/internal/config"
	"github.com/moasq/nanowave/internal/storage"
)

// RuntimeStatus is the UI-facing runtime abstraction used by commands.
type RuntimeStatus struct {
	Kind                agentruntime.Kind
	DisplayName         string
	BinaryPath          string
	InstallCommand      string
	SupportsInteractive bool
	Installed           bool
	Version             string
	Auth                *agentruntime.AuthStatus
}

type runtimeSelection struct {
	kind   agentruntime.Kind
	path   string
	model  string
	client agentruntime.Runtime
}

func resolveRuntimeSelection(cfg *config.Config, project *storage.Project, opts ServiceOpts) (*runtimeSelection, error) {
	runtimeKind := cfg.RuntimeKind
	model := ""
	if project != nil {
		if project.RuntimeKind != "" {
			runtimeKind = agentruntime.NormalizeKind(project.RuntimeKind)
		}
		if strings.TrimSpace(project.ModelID) != "" {
			model = strings.TrimSpace(project.ModelID)
		}
	}
	explicitModel := model != ""
	if strings.TrimSpace(opts.Runtime) != "" {
		runtimeKind = agentruntime.NormalizeKind(opts.Runtime)
	}
	if strings.TrimSpace(opts.Model) != "" {
		model = strings.TrimSpace(opts.Model)
		explicitModel = true
	}
	if runtimeKind == "" {
		runtimeKind = agentruntime.KindClaude
	}

	runtimePath := cfg.RuntimePath
	if runtimeKind != cfg.RuntimeKind || strings.TrimSpace(runtimePath) == "" {
		runtimePath, _ = agentruntime.FindBinary(runtimeKind)
	}
	if strings.TrimSpace(runtimePath) == "" {
		desc := agentruntime.DescriptorForKind(runtimeKind)
		return nil, fmt.Errorf("%s CLI is not installed. Install with: %s", desc.DisplayName, desc.InstallCommand)
	}

	runtimeClient, err := agentruntime.New(runtimeKind, runtimePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize %s runtime: %w", runtimeKind, err)
	}

	runtimeModels := cfg.RuntimeModelOptions(runtimeKind, runtimeClient.SuggestedModels())
	if !explicitModel {
		model = strings.TrimSpace(cfg.DefaultModelForRuntime(runtimeKind))
	}
	if model == "" || !runtimeSupportsModel(runtimeModels, model) {
		model = runtimeClient.DefaultModel(agentruntime.PhaseBuild)
	}

	return &runtimeSelection{
		kind:   runtimeKind,
		path:   runtimePath,
		model:  model,
		client: runtimeClient,
	}, nil
}

func runtimeStatusFrom(kind agentruntime.Kind, path string, client agentruntime.Runtime) RuntimeStatus {
	desc := agentruntime.DescriptorForKind(kind)
	status := RuntimeStatus{
		Kind:                kind,
		DisplayName:         desc.DisplayName,
		BinaryPath:          strings.TrimSpace(path),
		InstallCommand:      desc.InstallCommand,
		SupportsInteractive: desc.SupportsInteractive,
		Installed:           strings.TrimSpace(path) != "",
	}
	if client == nil && status.Installed {
		client, _ = agentruntime.New(kind, path)
	}
	if client != nil {
		status.Version = strings.TrimSpace(client.Version())
		status.Auth = client.AuthStatus()
	}
	return status
}

// ResolveRuntimeStatus returns the runtime selected by config/flags without requiring callers
// to touch the agentruntime package directly.
func ResolveRuntimeStatus(cfg *config.Config, opts ServiceOpts) RuntimeStatus {
	if cfg == nil {
		return runtimeStatusFrom(agentruntime.KindClaude, "", nil)
	}

	kind := cfg.RuntimeKind
	if strings.TrimSpace(opts.Runtime) != "" {
		kind = agentruntime.NormalizeKind(opts.Runtime)
	}
	if kind == "" {
		kind = agentruntime.KindClaude
	}

	path := cfg.RuntimePath
	if kind != cfg.RuntimeKind || strings.TrimSpace(path) == "" {
		path, _ = agentruntime.FindBinary(kind)
	}

	return runtimeStatusFrom(kind, path, nil)
}

// SupportedRuntimeStatuses returns all supported runtimes with install/auth metadata.
func SupportedRuntimeStatuses() []RuntimeStatus {
	descs := agentruntime.AllDescriptors()
	statuses := make([]RuntimeStatus, 0, len(descs))
	for _, desc := range descs {
		path, _ := agentruntime.FindBinary(desc.Kind)
		statuses = append(statuses, runtimeStatusFrom(desc.Kind, path, nil))
	}
	return statuses
}
