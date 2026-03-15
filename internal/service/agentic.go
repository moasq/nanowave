package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/moasq/nanowave/internal/agentruntime"
	"github.com/moasq/nanowave/internal/mcpregistry"
	"github.com/moasq/nanowave/internal/nwtool"
	"github.com/moasq/nanowave/internal/orchestration"
	"github.com/moasq/nanowave/internal/storage"
	"github.com/moasq/nanowave/internal/terminal"
)

// AgenticSend runs the agentic mode: a single LLM call with all nanowave tools
// available. The LLM decides the workflow — no rigid phases.
func (s *Service) AgenticSend(ctx context.Context, prompt string, images []string) error {
	s.stopBackgroundLogStreaming()

	isEdit := s.config.HasProject()

	var ac orchestration.ActionContext
	if isEdit {
		project, err := s.projectStore.Load()
		if err != nil || project == nil {
			return fmt.Errorf("no active project found")
		}

		terminal.Header("Nanowave")
		terminal.Detail("Project", projectName(project))

		platform, platforms, watchProjectShape := orchestration.DetectProjectBuildHints(project.ProjectPath)
		ac = orchestration.ActionContext{
			ProjectDir:        project.ProjectPath,
			AppName:           orchestration.ReadProjectAppName(project.ProjectPath),
			SessionID:         project.SessionID,
			Platform:          platform,
			Platforms:         platforms,
			WatchProjectShape: watchProjectShape,
		}
	} else {
		terminal.Header("Nanowave Build")
	}

	// Compose system prompt — domain knowledge only, no workflow steps
	systemPrompt := orchestration.ComposeAgenticSystemPrompt(ac)

	// Build tool list: core tools + all MCP tools
	reg := mcpregistry.New()
	mcpregistry.RegisterAll(reg)
	tools := orchestration.CoreAgenticToolsList()
	tools = append(tools, reg.AllTools()...)

	// Add integration tools if available
	if s.manager != nil {
		activeProviders := s.manager.ResolveExisting(ac.AppName)
		tools = append(tools, s.manager.AgentTools(activeProviders)...)
	}

	// For non-Claude runtimes, inject tool descriptions into the system prompt
	if s.runtimeKind != agentruntime.KindClaude {
		systemPrompt += nwtool.NewDefaultRegistry().ToolDescriptionsMarkdown()
	}

	workDir := ""
	if isEdit {
		workDir = ac.ProjectDir
	}

	// Progress display — "agentic" mode shows tool activity without rigid phase numbers
	progress := terminal.NewProgressDisplay("agentic", 0)
	runtimeLabel := ""
	if s.runtime != nil {
		runtimeLabel = s.runtime.DisplayName()
	}
	progress.SetRuntimeLabel(runtimeLabel)
	progress.Start()

	streamCb := orchestration.NewProgressCallbackExported(progress, runtimeLabel)

	// Single call — LLM drives everything
	resp, err := s.runtime.GenerateStreaming(ctx, prompt, agentruntime.GenerateOpts{
		SystemPrompt: systemPrompt,
		MaxTurns:     50,
		Model:        s.phaseModel(agentruntime.PhaseBuild),
		AllowedTools: tools,
		WorkDir:      workDir,
		Images:       images,
		SessionID:    ac.SessionID,
	}, streamCb)

	if err != nil {
		progress.StopWithError("Build failed")
		terminal.Error(fmt.Sprintf("Build failed: %v", err))
		return err
	}

	progress.StopWithSuccess("Done")

	if resp != nil {
		s.usageStore.RecordUsage(resp.TotalCostUSD, resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CacheReadInputTokens, resp.Usage.CacheCreationInputTokens)
		if resp.TotalCostUSD > 0 {
			terminal.Detail("Cost", fmt.Sprintf("$%.4f", resp.TotalCostUSD))
		}
	}

	if isEdit {
		project, _ := s.projectStore.Load()
		if project != nil && resp != nil && resp.SessionID != "" {
			project.SessionID = resp.SessionID
			project.RuntimeKind = string(s.runtimeKind)
			project.ModelID = s.CurrentModel()
			s.projectStore.Save(project)
		}
		s.historyStore.Append(storage.HistoryMessage{Role: "user", Content: prompt})
		summary := "edit"
		if resp != nil {
			summary = truncateStr(resp.Result, 100)
		}
		s.historyStore.Append(storage.HistoryMessage{Role: "assistant", Content: summary})
		return nil
	}

	// For new builds, detect newly created project in the catalog
	if resp != nil {
		projectDir := detectNewestProject(s.config.CatalogRoot())
		if projectDir != "" {
			s.config.SetProject(projectDir)
			s.projectStore = storage.NewProjectStore(s.config.NanowaveDir)
			s.historyStore = storage.NewHistoryStore(s.config.NanowaveDir)
			s.usageStore = storage.NewUsageStore(s.config.NanowaveDir)

			appName := orchestration.ReadProjectAppName(projectDir)
			if err := s.config.EnsureNanowaveDir(); err == nil {
				proj := &storage.Project{
					ID:          1,
					Name:        &appName,
					Status:      "active",
					ProjectPath: projectDir,
					SessionID:   resp.SessionID,
					RuntimeKind: string(s.runtimeKind),
					ModelID:     s.CurrentModel(),
				}
				platform, platforms, _ := orchestration.DetectProjectBuildHints(projectDir)
				proj.Platform = platform
				proj.Platforms = platforms
				s.projectStore.Save(proj)
			}

			s.historyStore.Append(storage.HistoryMessage{Role: "user", Content: prompt})
			s.historyStore.Append(storage.HistoryMessage{Role: "assistant", Content: fmt.Sprintf("Built %s", appName)})

			fmt.Println()
			terminal.Success(fmt.Sprintf("%s is ready!", appName))
			terminal.Detail("Location", projectDir)

			xcodeproj := filepath.Join(projectDir, SanitizeToPascalCase(appName)+".xcodeproj")
			if _, err := os.Stat(xcodeproj); err == nil {
				terminal.Detail("Open in Xcode", fmt.Sprintf("open %s", xcodeproj))
			}
		}
	}

	return nil
}

// detectNewestProject finds the most recently modified project_config.json in the catalog.
func detectNewestProject(catalogRoot string) string {
	entries, err := os.ReadDir(catalogRoot)
	if err != nil {
		return ""
	}

	type candidate struct {
		path    string
		modTime int64
	}
	var candidates []candidate

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		configPath := filepath.Join(catalogRoot, entry.Name(), "project_config.json")
		info, err := os.Stat(configPath)
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{
			path:    filepath.Join(catalogRoot, entry.Name()),
			modTime: info.ModTime().UnixNano(),
		})
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime > candidates[j].modTime
	})
	return candidates[0].path
}
