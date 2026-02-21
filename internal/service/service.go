package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/moasq/nanowave/internal/claude"
	"github.com/moasq/nanowave/internal/config"
	"github.com/moasq/nanowave/internal/orchestration"
	"github.com/moasq/nanowave/internal/storage"
	"github.com/moasq/nanowave/internal/terminal"
)

// defaultSimulatorPreference is the preferred simulator name when auto-detecting.
var defaultSimulatorPreference = []string{"iPhone 17 Pro", "iPhone 17", "iPhone 16 Pro", "iPhone 16", "iPhone Air"}

const defaultRunLogWatchSeconds = 30

// SimulatorDevice represents an available iOS simulator.
type SimulatorDevice struct {
	Name    string
	UDID    string
	Runtime string // e.g. "iOS 18.1"
}

// Service coordinates app generation for CLI usage.
type Service struct {
	config       *config.Config
	claude       *claude.Client
	projectStore *storage.ProjectStore
	historyStore *storage.HistoryStore
	usageStore   *storage.UsageStore
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

	return &Service{
		config:       cfg,
		claude:       claudeClient,
		projectStore: storage.NewProjectStore(cfg.NanowaveDir),
		historyStore: storage.NewHistoryStore(cfg.NanowaveDir),
		usageStore:   storage.NewUsageStore(cfg.NanowaveDir),
		model:        model,
	}, nil
}

// Send auto-routes to build (no project) or edit (project exists).
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

// detectDefaultSimulator picks the best available iPhone simulator.
func (s *Service) detectDefaultSimulator() string {
	devices, err := s.ListSimulators()
	if err != nil || len(devices) == 0 {
		return "iPhone 17 Pro"
	}

	// Try preferred names in order
	available := make(map[string]bool)
	for _, d := range devices {
		available[d.Name] = true
	}
	for _, name := range defaultSimulatorPreference {
		if available[name] {
			return name
		}
	}

	// Fall back to first available iPhone
	return devices[0].Name
}

// ListSimulators returns available iOS simulator devices.
func (s *Service) ListSimulators() ([]SimulatorDevice, error) {
	out, err := exec.Command("xcrun", "simctl", "list", "devices", "available", "-j").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list simulators: %w", err)
	}

	var result struct {
		Devices map[string][]struct {
			Name        string `json:"name"`
			UDID        string `json:"udid"`
			IsAvailable bool   `json:"isAvailable"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("failed to parse simulator list: %w", err)
	}

	var devices []SimulatorDevice
	for runtime, devs := range result.Devices {
		if !strings.Contains(runtime, "iOS") {
			continue
		}
		// Extract readable runtime: "com.apple.CoreSimulator.SimRuntime.iOS-18-1" → "iOS 18.1"
		runtimeName := parseRuntimeName(runtime)
		for _, d := range devs {
			if !d.IsAvailable {
				continue
			}
			// Only include iPhones for simplicity
			if !strings.HasPrefix(d.Name, "iPhone") {
				continue
			}
			devices = append(devices, SimulatorDevice{
				Name:    d.Name,
				UDID:    d.UDID,
				Runtime: runtimeName,
			})
		}
	}

	// Sort: newest runtime first, then by name
	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Runtime != devices[j].Runtime {
			return devices[i].Runtime > devices[j].Runtime
		}
		return devices[i].Name < devices[j].Name
	})

	// Deduplicate by name — keep only the newest runtime version
	seen := map[string]bool{}
	var unique []SimulatorDevice
	for _, d := range devices {
		if seen[d.Name] {
			continue
		}
		seen[d.Name] = true
		unique = append(unique, d)
	}

	return unique, nil
}

// build creates a new app from a prompt using the multi-phase pipeline.
func (s *Service) build(ctx context.Context, prompt string, images []string) error {
	terminal.Header("Nanowave Build")

	pipeline := orchestration.NewPipeline(s.claude, s.config, s.model)
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
			ID:          1,
			Name:        &appName,
			Status:      "active",
			ProjectPath: result.ProjectDir,
			BundleID:    result.BundleID,
			SessionID:   result.SessionID,
		}
		s.projectStore.Save(proj)
		s.historyStore.Append(storage.HistoryMessage{Role: "user", Content: prompt})
		s.historyStore.Append(storage.HistoryMessage{
			Role:    "assistant",
			Content: fmt.Sprintf("Built %s with %d files", result.AppName, result.FileCount),
		})
	}

	// Print results
	fmt.Println()
	terminal.Success(fmt.Sprintf("%s is ready!", result.AppName))
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

	s.historyStore.Append(storage.HistoryMessage{Role: "user", Content: prompt})
	s.historyStore.Append(storage.HistoryMessage{
		Role:    "assistant",
		Content: fmt.Sprintf("Applied edit: %s", truncateStr(prompt, 50)),
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

// Run builds and launches the project in the iOS Simulator.
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

	scheme := strings.TrimSuffix(xcodeprojName, ".xcodeproj")
	simulator := s.CurrentSimulator()
	destination := fmt.Sprintf("platform=iOS Simulator,name=%s", simulator)

	terminal.Detail("Simulator", simulator)

	// Build for simulator
	spinner := terminal.NewSpinner("Building for simulator...")
	spinner.Start()

	buildCmd := exec.CommandContext(ctx, "xcodebuild",
		"-project", xcodeprojName,
		"-scheme", scheme,
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

	// Boot simulator
	spinner = terminal.NewSpinner(fmt.Sprintf("Launching %s...", simulator))
	spinner.Start()

	// Boot the simulator by name
	_ = exec.CommandContext(ctx, "xcrun", "simctl", "boot", simulator).Run()

	// Open Simulator.app
	_ = exec.CommandContext(ctx, "open", "-a", "Simulator").Run()

	// Install and launch the app
	bundleID := project.BundleID
	if bundleID == "" {
		bundleID = fmt.Sprintf("com.nanowave.%s", strings.ToLower(scheme))
	}

	// Find the built .app in DerivedData
	appPath := findBuiltApp(project.ProjectPath, scheme)
	if appPath != "" {
		_ = exec.CommandContext(ctx, "xcrun", "simctl", "install", "booted", appPath).Run()
		_ = exec.CommandContext(ctx, "xcrun", "simctl", "launch", "booted", bundleID).Run()
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

// parseRuntimeName converts "com.apple.CoreSimulator.SimRuntime.iOS-18-1" to "iOS 18.1".
func parseRuntimeName(runtime string) string {
	// Extract the part after the last "SimRuntime."
	parts := strings.Split(runtime, "SimRuntime.")
	if len(parts) < 2 {
		return runtime
	}
	name := parts[1]
	// "iOS-18-1" → "iOS 18.1"
	name = strings.Replace(name, "-", " ", 1)
	name = strings.ReplaceAll(name, "-", ".")
	return name
}

// findBuiltApp looks for the built .app bundle in DerivedData.
func findBuiltApp(projectDir, scheme string) string {
	// xcodebuild typically puts builds in DerivedData
	home, _ := os.UserHomeDir()
	derivedData := filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")

	entries, err := os.ReadDir(derivedData)
	if err != nil {
		return ""
	}

	// Find the matching DerivedData directory
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), scheme+"-") {
			continue
		}
		// Look for the .app in Build/Products/Debug-iphonesimulator/
		appDir := filepath.Join(derivedData, entry.Name(), "Build", "Products", "Debug-iphonesimulator")
		appEntries, err := os.ReadDir(appDir)
		if err != nil {
			continue
		}
		for _, ae := range appEntries {
			if strings.HasSuffix(ae.Name(), ".app") {
				return filepath.Join(appDir, ae.Name())
			}
		}
	}

	return ""
}

func runLogWatchDuration() time.Duration {
	raw := strings.TrimSpace(os.Getenv("NANOWAVE_RUN_LOG_WATCH_SECONDS"))
	if raw == "" {
		return time.Duration(defaultRunLogWatchSeconds) * time.Second
	}

	if strings.EqualFold(raw, "follow") {
		return -1
	}

	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < -1 {
		return time.Duration(defaultRunLogWatchSeconds) * time.Second
	}

	if seconds == -1 {
		return -1
	}

	return time.Duration(seconds) * time.Second
}

func streamSimulatorLogs(ctx context.Context, processName, bundleID string, duration time.Duration) error {
	if duration <= 0 {
		if duration == 0 {
			return nil
		}
	}

	watchCtx := ctx
	cancel := func() {}
	if duration > 0 {
		watchCtx, cancel = context.WithTimeout(ctx, duration)
	}
	defer cancel()

	predicate := fmt.Sprintf(`process == "%s"`, processName)
	if strings.TrimSpace(bundleID) != "" {
		predicate = fmt.Sprintf(`process == "%s" OR subsystem CONTAINS[c] "%s"`, processName, bundleID)
	}

	cmd := exec.CommandContext(watchCtx, "xcrun", "simctl", "spawn", "booted", "log", "stream",
		"--style", "compact",
		"--level", "debug",
		"--predicate", predicate,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to read log stream stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to read log stream stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start simulator log stream: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go streamLogReader(stdout, false, &wg)
	go streamLogReader(stderr, true, &wg)

	waitErr := cmd.Wait()
	wg.Wait()

	if watchCtx.Err() == context.DeadlineExceeded {
		terminal.Info("Stopped log streaming.")
		return nil
	}
	if watchCtx.Err() == context.Canceled {
		return nil
	}
	if waitErr != nil {
		return fmt.Errorf("simulator log stream failed: %w", waitErr)
	}

	return nil
}

func streamLogReader(r io.Reader, isErr bool, wg *sync.WaitGroup) {
	defer wg.Done()

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if isErr {
			fmt.Printf("  %s[sim-log]%s %s\n", terminal.Dim, terminal.Reset, line)
			continue
		}
		fmt.Printf("  %s[sim-log]%s %s\n", terminal.Dim, terminal.Reset, line)
	}
}
