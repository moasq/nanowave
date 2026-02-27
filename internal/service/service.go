package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"os/user"

	"github.com/moasq/nanowave/internal/claude"
	"github.com/moasq/nanowave/internal/config"
	"github.com/moasq/nanowave/internal/integrations"
	"github.com/moasq/nanowave/internal/integrations/providers"
	"github.com/moasq/nanowave/internal/orchestration"
	"github.com/moasq/nanowave/internal/storage"
	"github.com/moasq/nanowave/internal/terminal"
)

// Service coordinates app generation for CLI usage.
type Service struct {
	config       *config.Config
	claude       *claude.Client
	projectStore *storage.ProjectStore
	historyStore *storage.HistoryStore
	usageStore   *storage.UsageStore
	manager      *integrations.Manager
	model        string // user-selected model override (empty = default "sonnet")
}

// ServiceOpts holds optional configuration for the service.
type ServiceOpts struct {
	Model string // Claude model override (sonnet, opus, haiku)
}

// NewService creates a new service.
func NewService(cfg *config.Config, opts ...ServiceOpts) (*Service, error) {
	claudeClient := claude.NewClient(cfg.ClaudePath)

	var model string
	if len(opts) > 0 && opts[0].Model != "" {
		model = opts[0].Model
	}

	// Initialize integration manager with all registered providers.
	// Store lives at ~/.nanowave/ (global, not per-project).
	reg := integrations.NewRegistry()
	providers.RegisterAll(reg)
	home, _ := os.UserHomeDir()
	storeRoot := filepath.Join(home, ".nanowave")
	intStore := integrations.NewIntegrationStore(storeRoot)
	_ = intStore.Load()
	mgr := integrations.NewManager(reg, intStore)

	return &Service{
		config:       cfg,
		claude:       claudeClient,
		projectStore: storage.NewProjectStore(cfg.NanowaveDir),
		historyStore: storage.NewHistoryStore(cfg.NanowaveDir),
		usageStore:   storage.NewUsageStore(cfg.NanowaveDir),
		manager:      mgr,
		model:        model,
	}, nil
}

// Send auto-routes to build (no project), question (detected question), or edit.
// images is an optional list of absolute paths to image files to include.
func (s *Service) Send(ctx context.Context, prompt string, images []string) error {
	if !s.config.HasProject() {
		if err := s.build(ctx, prompt, images); err != nil {
			return err
		}
		// Auto-run on simulator after successful build
		fmt.Println()
		return s.Run(ctx)
	}
	return s.edit(ctx, prompt, images)
}

// SetModel changes the model at runtime.
func (s *Service) SetModel(model string) {
	s.model = model
}

// CurrentModel returns the current model name.
func (s *Service) CurrentModel() string {
	if s.model == "" {
		return "sonnet"
	}
	return s.model
}

// ClearSession resets the session ID so the next request starts fresh.
func (s *Service) ClearSession() {
	project, err := s.projectStore.Load()
	if err != nil || project == nil {
		return
	}
	project.SessionID = ""
	s.projectStore.Save(project)
	s.historyStore.Clear()
	s.usageStore.Reset()
}

// Usage returns the current session usage stats.
func (s *Service) Usage() *storage.SessionUsage {
	return s.usageStore.Current()
}

// UpdateConfig updates the service config (e.g., after build creates a project).
func (s *Service) UpdateConfig(cfg *config.Config) {
	s.config = cfg
	s.projectStore = storage.NewProjectStore(cfg.NanowaveDir)
	s.historyStore = storage.NewHistoryStore(cfg.NanowaveDir)
	s.usageStore = storage.NewUsageStore(cfg.NanowaveDir)
}

// SetSimulator sets the simulator device name and persists it.
func (s *Service) SetSimulator(name string) {
	project, err := s.projectStore.Load()
	if err != nil || project == nil {
		return
	}
	project.Simulator = name
	s.projectStore.Save(project)
}

// CurrentSimulator returns the selected simulator name.
// If none is set, auto-detects the best available iPhone simulator.
func (s *Service) CurrentSimulator() string {
	project, err := s.projectStore.Load()
	if err != nil || project == nil || project.Simulator == "" {
		return s.detectDefaultSimulator()
	}
	return project.Simulator
}

// currentDeviceFamily reads the device family from the current project, defaulting to "iphone".
func (s *Service) currentDeviceFamily() string {
	project, err := s.projectStore.Load()
	if err != nil || project == nil || project.DeviceFamily == "" {
		return "iphone"
	}
	return project.DeviceFamily
}

// currentPlatform reads the platform from the current project, defaulting to "ios".
func (s *Service) currentPlatform() string {
	project, err := s.projectStore.Load()
	if err != nil || project == nil || project.Platform == "" {
		return "ios"
	}
	return project.Platform
}

// platformBundleIDSuffix returns the bundle ID suffix for a platform.
func platformBundleIDSuffix(platform string) string {
	switch platform {
	case "tvos":
		return ".tv"
	case "visionos":
		return ".vision"
	case "macos":
		return ".mac"
	default:
		return ""
	}
}

// build creates a new app from a prompt using the multi-phase pipeline.
func (s *Service) build(ctx context.Context, prompt string, images []string) error {
	terminal.Header("Nanowave Build")

	pipeline := orchestration.NewPipeline(s.claude, s.config, s.model)
	pipeline.SetManager(s.manager)
	result, err := pipeline.Build(ctx, prompt, images)
	if err != nil {
		terminal.Error(fmt.Sprintf("Build failed: %v", err))
		return err
	}

	// Switch config to the newly created project directory so state is saved there
	s.config.SetProject(result.ProjectDir)
	s.projectStore = storage.NewProjectStore(s.config.NanowaveDir)
	s.historyStore = storage.NewHistoryStore(s.config.NanowaveDir)
	s.usageStore = storage.NewUsageStore(s.config.NanowaveDir)

	// Record usage
	s.usageStore.RecordUsage(result.TotalCostUSD, result.InputTokens, result.OutputTokens, result.CacheRead, result.CacheCreated)

	// Save state
	if err := s.config.EnsureNanowaveDir(); err == nil {
		appName := result.AppName
		proj := &storage.Project{
			ID:           1,
			Name:         &appName,
			Status:       "active",
			ProjectPath:  result.ProjectDir,
			BundleID:     result.BundleID,
			Platform:     result.Platform,
			Platforms:     result.Platforms,
			DeviceFamily: result.DeviceFamily,
			SessionID:    result.SessionID,
		}
		s.projectStore.Save(proj)
		s.historyStore.Append(storage.HistoryMessage{Role: "user", Content: prompt})
		buildSummary := fmt.Sprintf("Built %s (%d files)", result.AppName, result.CompletedFiles)
		if result.Description != "" {
			buildSummary += " — " + result.Description
		}
		s.historyStore.Append(storage.HistoryMessage{
			Role:    "assistant",
			Content: buildSummary,
		})
	}

	// Print results
	fmt.Println()
	terminal.Success(fmt.Sprintf("%s is ready!", result.AppName))
	if result.Description != "" {
		fmt.Printf("  %s%s%s\n", terminal.Dim, result.Description, terminal.Reset)
	}
	fmt.Println()
	if len(result.Features) > 0 {
		for _, f := range result.Features {
			fmt.Printf("  %s•%s %s%s%s", terminal.Bold, terminal.Reset, terminal.Bold, f.Name, terminal.Reset)
			if f.Description != "" {
				fmt.Printf(" %s— %s%s", terminal.Dim, f.Description, terminal.Reset)
			}
			fmt.Println()
		}
		fmt.Println()
	}
	terminal.Detail("Files", fmt.Sprintf("%d", result.CompletedFiles))
	terminal.Detail("Location", result.ProjectDir)

	appNamePascal := SanitizeToPascalCase(result.AppName)
	xcodeproj := filepath.Join(result.ProjectDir, appNamePascal+".xcodeproj")
	if _, err := os.Stat(xcodeproj); err == nil {
		terminal.Detail("Open in Xcode", fmt.Sprintf("open %s", xcodeproj))
	} else {
		terminal.Detail("Open folder", fmt.Sprintf("open %s", result.ProjectDir))
	}

	return nil
}

// edit modifies an existing project.
func (s *Service) edit(ctx context.Context, prompt string, images []string) error {
	project, err := s.projectStore.Load()
	if err != nil || project == nil {
		return fmt.Errorf("no active project found")
	}

	terminal.Header("Nanowave Edit")
	terminal.Detail("Project", projectName(project))

	pipeline := orchestration.NewPipeline(s.claude, s.config, s.model)
	pipeline.SetManager(s.manager)
	result, err := pipeline.Edit(ctx, prompt, project.ProjectPath, project.SessionID, images)
	if err != nil {
		terminal.Error(fmt.Sprintf("Edit failed: %v", err))
		return err
	}

	// Record usage
	s.usageStore.RecordUsage(result.TotalCostUSD, result.InputTokens, result.OutputTokens, result.CacheRead, result.CacheCreated)

	// Update session ID for conversation continuity
	if result.SessionID != "" {
		project.SessionID = result.SessionID
		s.projectStore.Save(project)
	}

	// Show summary of what was done
	printSummary(result.Summary)

	s.historyStore.Append(storage.HistoryMessage{Role: "user", Content: prompt})
	summary := truncateStr(result.Summary, 200)
	if summary == "" {
		summary = fmt.Sprintf("Applied edit: %s", truncateStr(prompt, 50))
	}
	s.historyStore.Append(storage.HistoryMessage{
		Role:    "assistant",
		Content: summary,
	})

	return nil
}

// Fix auto-fixes build errors in the current project.
func (s *Service) Fix(ctx context.Context) error {
	project, err := s.projectStore.Load()
	if err != nil || project == nil {
		return fmt.Errorf("no active project found. Run `nanowave` first")
	}

	terminal.Header("Nanowave Fix")
	terminal.Detail("Project", projectName(project))

	pipeline := orchestration.NewPipeline(s.claude, s.config, s.model)
	pipeline.SetManager(s.manager)
	result, err := pipeline.Fix(ctx, project.ProjectPath, project.SessionID)
	if err != nil {
		terminal.Error(fmt.Sprintf("Fix failed: %v", err))
		return err
	}

	// Record usage
	s.usageStore.RecordUsage(result.TotalCostUSD, result.InputTokens, result.OutputTokens, result.CacheRead, result.CacheCreated)

	// Update session ID for conversation continuity
	if result.SessionID != "" {
		project.SessionID = result.SessionID
		s.projectStore.Save(project)
	}

	return nil
}

// Run builds and launches the project in the Simulator (or natively on macOS).
func (s *Service) Run(ctx context.Context) error {
	project, err := s.projectStore.Load()
	if err != nil || project == nil {
		return fmt.Errorf("no active project found. Run `nanowave` first")
	}

	terminal.Header("Nanowave Run")
	terminal.Detail("Project", projectName(project))

	// Find the .xcodeproj
	entries, err := os.ReadDir(project.ProjectPath)
	if err != nil {
		return fmt.Errorf("failed to read project directory: %w", err)
	}

	var xcodeprojName string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".xcodeproj") {
			xcodeprojName = entry.Name()
			break
		}
	}

	if xcodeprojName == "" {
		return fmt.Errorf("no .xcodeproj found in %s", project.ProjectPath)
	}

	appName := strings.TrimSuffix(xcodeprojName, ".xcodeproj")
	platform := s.currentPlatform()

	// Multi-platform: let user pick which target to run
	if len(project.Platforms) > 1 {
		options := make([]terminal.PickerOption, len(project.Platforms))
		for i, p := range project.Platforms {
			options[i] = terminal.PickerOption{
				Label: p,
				Desc:  orchestration.PlatformDisplayName(p),
			}
		}
		picked := terminal.Pick("Select platform to run", options, platform)
		if picked == "" {
			return nil // user cancelled
		}
		platform = picked
	}

	// Single-platform: scheme is just the app name (e.g. "FaveFoods")
	// Multi-platform: scheme includes platform suffix (e.g. "FaveFoodsVision")
	var scheme string
	if len(project.Platforms) > 1 {
		scheme = appName + orchestration.PlatformSourceDirSuffix(platform)
	} else {
		scheme = appName
	}
	isMacOS := platform == "macos"

	derivedDataPath := projectDerivedDataPath(project.ProjectPath)
	if err := os.MkdirAll(derivedDataPath, 0o755); err != nil {
		return fmt.Errorf("failed to prepare derived data path %s: %w", derivedDataPath, err)
	}

	var destination string
	if isMacOS {
		// macOS builds natively — no simulator
		destination = "generic/platform=macOS"
		terminal.Detail("Target", "macOS (native)")
	} else {
		simulator := s.CurrentSimulator()

		// Platform-aware simulator destination
		simPlatform := "iOS Simulator"
		switch platform {
		case "watchos":
			simPlatform = "watchOS Simulator"
		case "tvos":
			simPlatform = "tvOS Simulator"
		case "visionos":
			simPlatform = "visionOS Simulator"
		}

		// Resolve simulator name to UDID for a precise destination match,
		// avoiding OS version mismatch when xcodebuild defaults to OS:latest.
		simUDID := s.resolveSimulatorUDID(simulator)
		if simUDID != "" {
			destination = fmt.Sprintf("platform=%s,id=%s", simPlatform, simUDID)
		} else {
			destination = fmt.Sprintf("platform=%s,name=%s", simPlatform, simulator)
		}

		terminal.Detail("Simulator", simulator)
	}

	// Build
	buildMsg := "Building for simulator..."
	if isMacOS {
		buildMsg = "Building for macOS..."
	}
	spinner := terminal.NewSpinner(buildMsg)
	spinner.Start()

	buildCmd := exec.CommandContext(ctx, "xcodebuild",
		"-project", xcodeprojName,
		"-scheme", scheme,
		"-derivedDataPath", derivedDataPath,
		"-destination", destination,
		"-quiet",
		"build",
	)
	buildCmd.Dir = project.ProjectPath
	buildOutput, err := buildCmd.CombinedOutput()

	if err == nil {
		spinner.Stop()
	} else {
		spinner.StopWithMessage(fmt.Sprintf("%s%s!%s Build failed — auto-fixing...", terminal.Bold, terminal.Yellow, terminal.Reset))

		// Auto-fix: use Claude to diagnose and repair
		pipeline := orchestration.NewPipeline(s.claude, s.config, s.model)
		pipeline.SetManager(s.manager)
		fixResult, fixErr := pipeline.Fix(ctx, project.ProjectPath, project.SessionID)
		if fixErr != nil {
			terminal.Error("Auto-fix failed")
			return fmt.Errorf("xcodebuild failed: %w\n%s", err, string(buildOutput))
		}

		// Record fix usage
		s.usageStore.RecordUsage(fixResult.TotalCostUSD, fixResult.InputTokens, fixResult.OutputTokens, fixResult.CacheRead, fixResult.CacheCreated)
		if fixResult.SessionID != "" {
			project.SessionID = fixResult.SessionID
			s.projectStore.Save(project)
		}

		// Retry the build
		spinner = terminal.NewSpinner("Verifying build...")
		spinner.Start()

		retryCmd := exec.CommandContext(ctx, "xcodebuild",
			"-project", xcodeprojName,
			"-scheme", scheme,
			"-derivedDataPath", derivedDataPath,
			"-destination", destination,
			"-quiet",
			"build",
		)
		retryCmd.Dir = project.ProjectPath
		retryOutput, retryErr := retryCmd.CombinedOutput()

		if retryErr != nil {
			spinner.StopWithMessage(fmt.Sprintf("%s%s✗%s Build still failing after auto-fix", terminal.Bold, terminal.Red, terminal.Reset))
			terminal.Info("Run `nanowave fix` to try again")
			return fmt.Errorf("xcodebuild failed after fix: %w\n%s", retryErr, string(retryOutput))
		}
		spinner.StopWithMessage(fmt.Sprintf("%s%s✓%s Build fixed!", terminal.Bold, terminal.Green, terminal.Reset))
	}

	if err == nil {
		terminal.Success("Build succeeded")
	}

	bundleID := project.BundleID
	if bundleID == "" {
		bundleID = fmt.Sprintf("com.%s.%s", sanitizeBundleID(currentUsername()), strings.ToLower(appName))
	}
	if len(project.Platforms) > 1 {
		bundleID += platformBundleIDSuffix(platform)
	}

	// Find the built .app in the per-project derived data path.
	appPath, appErr := findBuiltAppInDerivedData(derivedDataPath, scheme, platform)
	if appErr != nil {
		return appErr
	}

	if isMacOS {
		// macOS: launch the app natively via `open`
		spinner = terminal.NewSpinner("Launching macOS app...")
		spinner.Start()

		openCmd := exec.CommandContext(ctx, "open", appPath)
		if openOutput, openErr := openCmd.CombinedOutput(); openErr != nil {
			spinner.Stop()
			return fmt.Errorf("failed to launch macOS app: %w%s", openErr, commandOutputSuffix(openOutput))
		}

		spinner.Stop()
		terminal.Success("Launched macOS app")

		// Stream native macOS logs
		watchDuration := runLogWatchDuration()
		if watchDuration > 0 {
			terminal.Info(fmt.Sprintf("Streaming macOS logs for %s...", watchDuration.Truncate(time.Second)))
			terminal.Detail("Tip", "Set NANOWAVE_RUN_LOG_WATCH_SECONDS=0 to disable log watching")
			if streamErr := streamMacOSLogs(ctx, scheme, bundleID, watchDuration); streamErr != nil {
				terminal.Warning(fmt.Sprintf("Log streaming unavailable: %v", streamErr))
			}
		} else if watchDuration < 0 {
			terminal.Info("Streaming macOS logs until interrupted...")
			terminal.Detail("Tip", "Set NANOWAVE_RUN_LOG_WATCH_SECONDS=0 to disable or a positive value for timed log watching")
			if streamErr := streamMacOSLogs(ctx, scheme, bundleID, watchDuration); streamErr != nil {
				terminal.Warning(fmt.Sprintf("Log streaming unavailable: %v", streamErr))
			}
		}
	} else {
		// Simulator path: boot, install, launch
		simulator := s.CurrentSimulator()
		spinner = terminal.NewSpinner(fmt.Sprintf("Launching %s...", simulator))
		spinner.Start()

		// Boot the simulator by UDID (falls back to name)
		simUDID := s.resolveSimulatorUDID(simulator)
		bootTarget := simulator
		if simUDID != "" {
			bootTarget = simUDID
		}
		bootCmd := exec.CommandContext(ctx, "xcrun", "simctl", "boot", bootTarget)
		if bootOutput, bootErr := bootCmd.CombinedOutput(); bootErr != nil && !isAlreadyBootedSimError(bootErr, bootOutput) {
			spinner.Stop()
			return fmt.Errorf("failed to boot simulator %s: %w%s", simulator, bootErr, commandOutputSuffix(bootOutput))
		}

		// Open Simulator.app
		openCmd := exec.CommandContext(ctx, "open", "-a", "Simulator")
		if openOutput, openErr := openCmd.CombinedOutput(); openErr != nil {
			terminal.Warning(fmt.Sprintf("Could not open Simulator.app: %v%s", openErr, commandOutputSuffix(openOutput)))
		}

		installCmd := exec.CommandContext(ctx, "xcrun", "simctl", "install", "booted", appPath)
		if installOutput, installErr := installCmd.CombinedOutput(); installErr != nil {
			spinner.Stop()
			return fmt.Errorf("failed to install app on simulator: %w%s", installErr, commandOutputSuffix(installOutput))
		}

		launchCmd := exec.CommandContext(ctx, "xcrun", "simctl", "launch", "booted", bundleID)
		if launchOutput, launchErr := launchCmd.CombinedOutput(); launchErr != nil {
			spinner.Stop()
			return fmt.Errorf("failed to launch app %s on simulator: %w%s", bundleID, launchErr, commandOutputSuffix(launchOutput))
		}

		spinner.Stop()
		terminal.Success(fmt.Sprintf("Launched on %s", simulator))

		watchDuration := runLogWatchDuration()
		if watchDuration > 0 {
			terminal.Info(fmt.Sprintf("Streaming simulator logs for %s...", watchDuration.Truncate(time.Second)))
			terminal.Detail("Tip", "Set NANOWAVE_RUN_LOG_WATCH_SECONDS=0 to disable log watching")
			if streamErr := streamSimulatorLogs(ctx, scheme, bundleID, watchDuration); streamErr != nil {
				terminal.Warning(fmt.Sprintf("Log streaming unavailable: %v", streamErr))
			}
		} else if watchDuration < 0 {
			terminal.Info("Streaming simulator logs until interrupted...")
			terminal.Detail("Tip", "Set NANOWAVE_RUN_LOG_WATCH_SECONDS=0 to disable or a positive value for timed log watching")
			if streamErr := streamSimulatorLogs(ctx, scheme, bundleID, watchDuration); streamErr != nil {
				terminal.Warning(fmt.Sprintf("Log streaming unavailable: %v", streamErr))
			}
		}
	}

	return nil
}

// Info shows the current project status.
func (s *Service) Info() error {
	project, err := s.projectStore.Load()
	if err != nil || project == nil {
		terminal.Info("No active project. Describe the app you want to build.")
		return nil
	}

	terminal.Header("Project Info")
	terminal.Detail("Name", projectName(project))
	terminal.Detail("Status", project.Status)
	terminal.Detail("Path", project.ProjectPath)
	terminal.Detail("Bundle ID", project.BundleID)
	if len(project.Platforms) > 1 {
		terminal.Detail("Platforms", strings.Join(project.Platforms, ", "))
	} else if project.Platform != "" {
		terminal.Detail("Platform", project.Platform)
	}
	terminal.Detail("Simulator", s.CurrentSimulator())

	history, _ := s.historyStore.List()
	terminal.Detail("Messages", fmt.Sprintf("%d", len(history)))

	// Show usage summary
	if today := s.usageStore.TodayUsage(); today != nil {
		terminal.Detail("Today", fmt.Sprintf("$%.4f (%d requests)", today.TotalCostUSD, today.Requests))
	}
	weekHistory := s.usageStore.History(7)
	if len(weekHistory) > 0 {
		var weekCost float64
		var weekRequests int
		for _, d := range weekHistory {
			weekCost += d.TotalCostUSD
			weekRequests += d.Requests
		}
		terminal.Detail("Week", fmt.Sprintf("$%.4f (%d requests, %d days)", weekCost, weekRequests, len(weekHistory)))
	}

	return nil
}

// Open opens the current project in Xcode.
func (s *Service) Open() error {
	project, err := s.projectStore.Load()
	if err != nil || project == nil {
		return fmt.Errorf("no active project found")
	}

	entries, err := os.ReadDir(project.ProjectPath)
	if err != nil {
		return fmt.Errorf("failed to read project directory: %w", err)
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".xcodeproj") {
			xcodeprojPath := filepath.Join(project.ProjectPath, entry.Name())
			terminal.Info(fmt.Sprintf("Opening %s...", entry.Name()))
			return exec.Command("open", xcodeprojPath).Run()
		}
	}

	terminal.Info(fmt.Sprintf("Opening %s...", project.ProjectPath))
	return exec.Command("open", project.ProjectPath).Run()
}

// HasProject returns whether the service has a loaded project.
func (s *Service) HasProject() bool {
	return s.config.HasProject()
}

// isQuestion returns true if the prompt looks like a pure question rather than an edit request.
// Conservative: only matches clear questions. Ambiguous prompts go through the edit pipeline.
func isQuestion(prompt string) bool {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return false
	}

	lower := strings.ToLower(trimmed)

	// If it contains action words, it's an edit request even if phrased as a question
	actionWords := []string{
		"fix ", "add ", "change ", "update ", "remove ", "delete ",
		"make ", "create ", "implement ", "replace ", "move ",
		"refactor ", "please ", "let us ", "let's ",
	}
	for _, a := range actionWords {
		if strings.Contains(lower, a) {
			return false
		}
	}

	// Must end with ? to be detected as a question via prefix matching
	if !strings.HasSuffix(trimmed, "?") {
		return false
	}

	// Only match clear question-word prefixes (with trailing ?)
	prefixes := []string{
		"what ", "how ", "why ", "where ", "which ",
		"is ", "are ", "does ", "do ", "can ", "could ",
		"should ", "would ", "tell me ", "explain ", "describe ",
		"show me ", "list ", "how many ", "how much ",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// question runs a cheap Q&A path using haiku with read-only tools.
func (s *Service) question(ctx context.Context, prompt, projectDir, sessionID string) (*claude.Response, error) {
	systemPrompt := `You are a helpful assistant answering questions about an iOS app project.
You have read-only access to the project files. Browse the codebase to answer accurately.
Be concise and direct. Do not modify any files.`

	readOnlyTools := []string{"Read", "Glob", "Grep"}

	var resp *claude.Response
	var err error

	resp, err = s.claude.GenerateStreaming(ctx, prompt, claude.GenerateOpts{
		SystemPrompt: systemPrompt,
		MaxTurns:     5,
		Model:        "haiku",
		WorkDir:      projectDir,
		AllowedTools: readOnlyTools,
		SessionID:    sessionID,
	}, func(ev claude.StreamEvent) {
		if ev.Type == "content_block_delta" && ev.Text != "" {
			fmt.Print(ev.Text)
		}
	})

	// End the streamed output with a newline
	fmt.Println()

	return resp, err
}

// ask is the internal method for answering questions with usage/history recording.
func (s *Service) ask(ctx context.Context, prompt string) error {
	project, err := s.projectStore.Load()
	if err != nil || project == nil {
		return fmt.Errorf("no active project found")
	}

	fmt.Println()

	resp, err := s.question(ctx, prompt, project.ProjectPath, project.SessionID)
	if err != nil {
		return fmt.Errorf("question failed: %w", err)
	}

	if resp != nil {
		s.usageStore.RecordUsage(resp.TotalCostUSD, resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CacheReadInputTokens, resp.Usage.CacheCreationInputTokens)
		if resp.SessionID != "" {
			project.SessionID = resp.SessionID
			s.projectStore.Save(project)
		}
	}

	s.historyStore.Append(storage.HistoryMessage{Role: "user", Content: prompt})
	answer := ""
	if resp != nil {
		answer = truncateStr(resp.Result, 200)
	}
	s.historyStore.Append(storage.HistoryMessage{Role: "assistant", Content: answer})

	return nil
}

// Ask is the public method for the /ask command.
func (s *Service) Ask(ctx context.Context, prompt string) error {
	return s.ask(ctx, prompt)
}

// printSummary prints a short dimmed summary of what Claude did.
// Extracts the first meaningful sentence, skipping noise.
func printSummary(summary string) {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return
	}
	// Take just the first line that isn't a markdown header, bullet, or empty
	var line string
	for _, l := range strings.Split(summary, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, "```") || strings.HasPrefix(l, "---") {
			continue
		}
		// Strip leading markdown bullets/numbers
		l = strings.TrimLeft(l, "-*•0123456789. ")
		l = strings.TrimPrefix(l, "**")
		l = strings.TrimSuffix(l, "**")
		if l != "" {
			line = l
			break
		}
	}
	if line == "" {
		return
	}
	if len(line) > 120 {
		line = line[:120] + "..."
	}
	fmt.Printf("\n  %s%s%s\n", terminal.Dim, line, terminal.Reset)
}

// ---- Helpers ----

// SanitizeToPascalCase converts a string to PascalCase.
func SanitizeToPascalCase(name string) string {
	var result strings.Builder
	capitalizeNext := true

	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if capitalizeNext {
				result.WriteRune(unicode.ToUpper(r))
				capitalizeNext = false
			} else {
				result.WriteRune(r)
			}
		} else {
			capitalizeNext = true
		}
	}
	return result.String()
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func projectName(p *storage.Project) string {
	if p != nil && p.Name != nil {
		return *p.Name
	}
	return "Unknown"
}

func projectDerivedDataPath(projectPath string) string {
	return filepath.Join(projectPath, ".nanowave", "DerivedData")
}

// findBuiltAppInDerivedData looks for the expected .app bundle in a specific DerivedData path.
func findBuiltAppInDerivedData(derivedDataPath, scheme, platform string) (string, error) {
	productsSubdir := "Debug-iphonesimulator"
	switch platform {
	case "watchos":
		productsSubdir = "Debug-watchsimulator"
	case "tvos":
		productsSubdir = "Debug-appletvsimulator"
	case "visionos":
		productsSubdir = "Debug-xrsimulator"
	case "macos":
		productsSubdir = "Debug"
	}

	productsDir := filepath.Join(derivedDataPath, "Build", "Products", productsSubdir)
	entries, err := os.ReadDir(productsDir)
	if err != nil {
		return "", fmt.Errorf("failed to read build products in %s: %w", productsDir, err)
	}

	expectedApp := scheme + ".app"
	var foundApps []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".app") {
			foundApps = append(foundApps, entry.Name())
			if entry.Name() == expectedApp {
				return filepath.Join(productsDir, entry.Name()), nil
			}
		}
	}

	if len(foundApps) == 0 {
		return "", fmt.Errorf("no .app bundle found in %s (derived data path: %s)", productsDir, derivedDataPath)
	}

	sort.Strings(foundApps)
	return "", fmt.Errorf("expected %s in %s but found %d app bundle(s): %s", expectedApp, productsDir, len(foundApps), strings.Join(foundApps, ", "))
}

func commandOutputSuffix(output []byte) string {
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return ""
	}
	return "\n" + trimmed
}

func currentUsername() string {
	u, err := user.Current()
	if err != nil || u.Username == "" {
		return "app"
	}
	return u.Username
}

func sanitizeBundleID(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	result := b.String()
	if result == "" {
		return "app"
	}
	return result
}
