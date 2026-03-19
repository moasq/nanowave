package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moasq/nanowave/internal/agentruntime"
	"github.com/moasq/nanowave/internal/integrations"
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

	// Resolve configured integrations BEFORE composing the prompt —
	// the system prompt needs to know which backends are available.
	var activeProviders []integrations.ActiveProvider
	if s.manager != nil {
		activeProviders = s.manager.ResolveExisting(ac.AppName)
		for _, ap := range activeProviders {
			ac.ActiveIntegrations = append(ac.ActiveIntegrations, string(ap.Provider.ID()))
		}
	}

	// Compose system prompt — includes integration awareness
	catalogRoot := s.config.CatalogRoot()
	systemPrompt := orchestration.ComposeAgenticSystemPrompt(ac, catalogRoot)

	// Build tool list: core tools + all MCP tools + configured integration tools
	reg := mcpregistry.New()
	mcpregistry.RegisterAll(reg)
	tools := orchestration.CoreAgenticToolsList()
	tools = append(tools, reg.AllTools()...)
	if s.manager != nil {
		tools = append(tools, s.manager.AgentTools(activeProviders)...)
	}

	// Inject nanowave tool descriptions into the system prompt for all runtimes.
	// nw_* tools (scaffold, setup_integration, etc.) are invoked via CLI:
	//   echo '{"key":"value"}' | nanowave tool <tool_name>
	systemPrompt += nwtool.NewDefaultRegistry().ToolDescriptionsMarkdown()

	var workDir string
	// Snapshot existing projects before build so we can detect the new one after
	var preExistingProjects map[string]bool
	if isEdit {
		workDir = ac.ProjectDir
		// Ensure project has MCP config, settings, and skill files for the current runtime
		orchestration.EnsureProjectConfigsExternal(workDir)
	} else {
		// New builds: start in the catalog root so the agent doesn't see
		// the CLI source tree. The agent will create the project dir via
		// nw_scaffold_project inside this directory.
		workDir = s.config.CatalogRoot()
		os.MkdirAll(workDir, 0o755)
		preExistingProjects = listCatalogDirs(workDir)
	}

	// Progress display — "agentic" mode shows tool activity without rigid phase numbers
	progress := terminal.NewProgressDisplay("agentic", 0)
	progress.Start()
	progress.AddActivity(fmt.Sprintf("Starting %s", s.runtime.DisplayName()))

	streamCb := orchestration.NewProgressCallbackExported(progress)

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

	// If resume failed because the session doesn't exist (e.g., created by a different runtime),
	// retry without the session ID so we start a fresh conversation.
	if err != nil && ac.SessionID != "" && strings.Contains(err.Error(), "No conversation found") {
		progress.Stop()
		progress = terminal.NewProgressDisplay("agentic", 0)
		progress.Start()
		progress.AddActivity(fmt.Sprintf("Starting %s", s.runtime.DisplayName()))
		streamCb = orchestration.NewProgressCallbackExported(progress)

		resp, err = s.runtime.GenerateStreaming(ctx, prompt, agentruntime.GenerateOpts{
			SystemPrompt: systemPrompt,
			MaxTurns:     50,
			Model:        s.phaseModel(agentruntime.PhaseBuild),
			AllowedTools: tools,
			WorkDir:      workDir,
			Images:       images,
		}, streamCb)
	}

	if err != nil {
		if ctx.Err() != nil {
			progress.Stop()
			return ctx.Err()
		}
		progress.StopWithError("Build failed")
		terminal.Error(fmt.Sprintf("Build failed: %v", err))
		return err
	}

	progress.StopWithSuccess("Done")

	if resp != nil {
		s.usageStore.RecordUsage(resp.TotalCostUSD, resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CacheReadInputTokens, resp.Usage.CacheCreationInputTokens)
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

	// For new builds, detect the newly created project
	if resp != nil {
		projectDir := detectNewProject(catalogRoot, preExistingProjects)
		// Fallback: if diff detection failed but the agent mentioned a path, use that
		if projectDir == "" && resp.Result != "" {
			if extracted := extractProjectPathFromText(resp.Result, catalogRoot); extracted != "" {
				projectDir = extracted
			}
		}
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

// listCatalogDirs returns a set of all directory names in the catalog root.
func listCatalogDirs(catalogRoot string) map[string]bool {
	entries, err := os.ReadDir(catalogRoot)
	if err != nil {
		return nil
	}
	dirs := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() {
			dirs[entry.Name()] = true
		}
	}
	return dirs
}

// detectNewProject finds the project directory created during the build by
// comparing the catalog against the pre-build snapshot. Any new directory
// containing at least one .swift file or a project_config.json is a candidate.
// Falls back to the most recently modified directory if the diff finds nothing.
func detectNewProject(catalogRoot string, preExisting map[string]bool) string {
	entries, err := os.ReadDir(catalogRoot)
	if err != nil {
		return ""
	}

	var newDirs []string
	type candidate struct {
		path    string
		modTime int64
	}
	var allCandidates []candidate

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(catalogRoot, entry.Name())
		if !looksLikeProject(fullPath) {
			continue
		}
		info, _ := entry.Info()
		var modTime int64
		if info != nil {
			modTime = info.ModTime().UnixNano()
		}
		allCandidates = append(allCandidates, candidate{path: fullPath, modTime: modTime})

		if !preExisting[entry.Name()] {
			newDirs = append(newDirs, fullPath)
		}
	}

	if len(newDirs) == 1 {
		return newDirs[0]
	}
	if len(newDirs) > 1 {
		sort.Slice(newDirs, func(i, j int) bool {
			iInfo, _ := os.Stat(newDirs[i])
			jInfo, _ := os.Stat(newDirs[j])
			if iInfo == nil || jInfo == nil {
				return false
			}
			return iInfo.ModTime().After(jInfo.ModTime())
		})
		return newDirs[0]
	}

	// Fallback: newest directory by mtime
	if len(allCandidates) == 0 {
		return ""
	}
	sort.Slice(allCandidates, func(i, j int) bool {
		return allCandidates[i].modTime > allCandidates[j].modTime
	})
	return allCandidates[0].path
}

// extractProjectPathFromText scans the agent's response for an absolute path
// inside the catalog root that looks like a project directory.
func extractProjectPathFromText(text, catalogRoot string) string {
	// Look for the catalog root path in the text
	idx := strings.Index(text, catalogRoot)
	if idx < 0 {
		return ""
	}
	// Extract the path starting from the catalog root
	rest := text[idx:]
	// Find the end of the path (space, newline, backtick, quote, or end-of-string)
	end := len(rest)
	for i, ch := range rest {
		if i == 0 {
			continue
		}
		if ch == ' ' || ch == '\n' || ch == '\r' || ch == '`' || ch == '"' || ch == '\'' || ch == ')' || ch == '|' {
			end = i
			break
		}
	}
	candidate := strings.TrimRight(rest[:end], "/.")
	if candidate == catalogRoot {
		return "" // just the root, not a project
	}
	info, err := os.Stat(candidate)
	if err != nil || !info.IsDir() {
		return ""
	}
	if looksLikeProject(candidate) {
		return candidate
	}
	return ""
}

// looksLikeProject returns true if the directory looks like an app project
// (has a project_config.json, .xcodeproj, project.yml, or .swift files).
func looksLikeProject(dir string) bool {
	checks := []string{"project_config.json", "project.yml"}
	for _, name := range checks {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	// Check for .xcodeproj or any .swift file in immediate children
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".xcodeproj") {
			return true
		}
		if strings.HasSuffix(e.Name(), ".swift") {
			return true
		}
	}
	return false
}
