package service

import (
	"context"
	"encoding/json"
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

		// AI-driven backend detection: ask the LLM what integrations this prompt needs,
		// then run interactive setup for any that aren't configured yet.
		activeProviders = s.detectAndProvisionBackends(ctx, prompt, activeProviders, ac.AppName)

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

	// For non-Claude runtimes (Codex, OpenCode), inject nw_* tool descriptions
	// as markdown so the LLM can invoke them via CLI: echo JSON | nanowave tool <name>
	// For Claude Code, nw_* tools are registered as MCP tools and don't need
	// prompt-based descriptions — Claude Code discovers them via --mcp-config.
	if s.runtimeKind != agentruntime.KindClaude {
		systemPrompt += nwtool.NewDefaultRegistry().ToolDescriptionsMarkdown()
	}

	// --- Diagnostics: log what the agentic call will receive ---
	platform := ac.Platform
	if platform == "" {
		platform = "ios"
	}
	ruleDiag := orchestration.LoadCoreRulesDiagnostics(platform)
	terminal.Detail("System prompt", fmt.Sprintf("%d chars", len(systemPrompt)))
	terminal.Detail("Skills injected", fmt.Sprintf("%d core, %d always, %d platform (%s) — %d chars",
		ruleDiag.CoreRulesLoaded, ruleDiag.AlwaysRulesLoaded, ruleDiag.PlatformRulesLoaded, platform, ruleDiag.CoreRulesChars))
	terminal.Detail("Model", s.phaseModel(agentruntime.PhaseBuild))

	// Log MCP servers
	var mcpServerNames []string
	for _, srv := range reg.Servers() {
		mcpServerNames = append(mcpServerNames, fmt.Sprintf("%s(%d)", srv.Name, len(srv.Tools)))
	}
	terminal.Detail("MCP servers", strings.Join(mcpServerNames, ", "))

	// Log tool counts by category
	coreToolCount := len(orchestration.CoreAgenticToolsList())
	mcpToolCount := len(reg.AllTools())
	nwToolCount := len(nwtool.NewDefaultRegistry().Names())
	terminal.Detail("Tools", fmt.Sprintf("%d core + %d MCP + %d nw_*", coreToolCount, mcpToolCount, nwToolCount))

	if len(ac.ActiveIntegrations) > 0 {
		terminal.Detail("Integrations", strings.Join(ac.ActiveIntegrations, ", "))
	}

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

		// Write .mcp.json and .claude/settings.json to the catalog root BEFORE
		// the agentic call so MCPs are available from turn 1, even before creating
		// the project directory via nw_scaffold_project.
		mcpErr := orchestration.WriteMCPConfigExternal(workDir)
		settingsErr := orchestration.WriteSettingsSharedExternal(workDir)
		if mcpErr != nil {
			terminal.Warning(fmt.Sprintf("Failed to write .mcp.json: %v", mcpErr))
		}
		if settingsErr != nil {
			terminal.Warning(fmt.Sprintf("Failed to write settings.json: %v", settingsErr))
		}
		terminal.Detail("MCP config", filepath.Join(workDir, ".mcp.json"))
		terminal.Detail("Settings", filepath.Join(workDir, ".claude", "settings.json"))
	}

	// Explicitly pass the .mcp.json path so Claude Code loads MCP servers
	// reliably — auto-discovery from WorkDir is not guaranteed.
	mcpConfigPath := filepath.Join(workDir, ".mcp.json")
	if _, err := os.Stat(mcpConfigPath); os.IsNotExist(err) {
		mcpConfigPath = "" // don't pass if file doesn't exist
	}

	terminal.Detail("WorkDir", workDir)

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
		MCPConfig:    mcpConfigPath,
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
			MCPConfig:    mcpConfigPath,
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
					BundleID:    orchestration.ReadProjectBundleID(projectDir),
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

// backendNeedsResult is the structured output from the AI backend detection call.
type backendNeedsResult struct {
	NeedsSupabase  bool `json:"needs_supabase"`
	NeedsRevenuecat bool `json:"needs_revenuecat"`
}

// detectAndProvisionBackends uses a fast LLM call to determine if the user's prompt
// requires backend integrations (Supabase for auth/database/storage, RevenueCat for
// subscriptions/in-app purchases). For any needed-but-unconfigured integration,
// runs the interactive setup flow so credentials are available before the main build.
func (s *Service) detectAndProvisionBackends(ctx context.Context, prompt string, existing []integrations.ActiveProvider, appName string) []integrations.ActiveProvider {
	if s.manager == nil {
		return existing
	}

	// Check what's already configured
	configuredIDs := make(map[string]bool, len(existing))
	for _, ap := range existing {
		configuredIDs[string(ap.Provider.ID())] = true
	}

	// If both are already configured, nothing to detect
	if configuredIDs["supabase"] && configuredIDs["revenuecat"] {
		return existing
	}

	// Fast LLM call: detect backend needs from the prompt
	needs := s.detectBackendNeeds(ctx, prompt)
	if needs == nil {
		return existing
	}

	// Determine which integrations need setup
	var toSetup []string
	if needs.NeedsSupabase && !configuredIDs["supabase"] {
		toSetup = append(toSetup, "supabase")
	}
	if needs.NeedsRevenuecat && !configuredIDs["revenuecat"] {
		toSetup = append(toSetup, "revenuecat")
	}
	if len(toSetup) == 0 {
		return existing
	}

	// Run interactive setup for each needed integration
	for _, providerID := range toSetup {
		terminal.Info(fmt.Sprintf("Your app needs %s — starting setup...", integrationDisplayName(providerID)))
		result, err := orchestration.SetupIntegrationExternal(ctx, providerID, appName, "")
		if err != nil {
			terminal.Warning(fmt.Sprintf("%s setup failed: %v", integrationDisplayName(providerID), err))
			continue
		}
		if success, ok := result["success"].(bool); ok && success {
			terminal.Success(fmt.Sprintf("%s configured", integrationDisplayName(providerID)))
		} else {
			terminal.Warning(fmt.Sprintf("%s setup skipped", integrationDisplayName(providerID)))
		}
	}

	// Re-resolve all providers now that setup is done
	return s.manager.ResolveExisting(appName)
}

// detectBackendNeeds runs a fast single-turn LLM call to determine if the user's
// prompt requires Supabase (auth, database, storage, realtime, cloud sync) or
// RevenueCat (subscriptions, in-app purchases, paywalls).
func (s *Service) detectBackendNeeds(ctx context.Context, prompt string) *backendNeedsResult {
	systemPrompt := `You are a backend requirements detector for an iOS app builder.
Analyze the user's app description and determine if it needs:
- Supabase: for authentication, user accounts, database, cloud storage, realtime sync, or multi-device data
- RevenueCat: for subscriptions, in-app purchases, paywalls, premium features, or monetization

Return ONLY valid JSON, no other text:
{"needs_supabase": true/false, "needs_revenuecat": true/false}

Rules:
- needs_supabase=true ONLY if the app explicitly needs user accounts, cloud database, server-side storage, or multi-device sync
- needs_supabase=false for apps that work with local-only data (notes, calculator, timer, etc.)
- needs_revenuecat=true ONLY if the app explicitly mentions subscriptions, premium, paywall, in-app purchases, or paid features
- needs_revenuecat=false unless monetization is explicitly requested
- When in doubt, return false — don't over-provision backends`

	resp, err := s.runtime.GenerateStreaming(ctx, prompt, agentruntime.GenerateOpts{
		SystemPrompt: systemPrompt,
		MaxTurns:     1,
		Model:        s.phaseModel(agentruntime.PhaseIntent),
	}, func(_ agentruntime.StreamEvent) {})
	if err != nil || resp == nil || resp.Result == "" {
		return nil
	}

	var needs backendNeedsResult
	cleaned := orchestration.ExtractJSON(resp.Result)
	if err := json.Unmarshal([]byte(cleaned), &needs); err != nil {
		return nil
	}

	return &needs
}

func integrationDisplayName(id string) string {
	switch id {
	case "supabase":
		return "Supabase"
	case "revenuecat":
		return "RevenueCat"
	default:
		return id
	}
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
