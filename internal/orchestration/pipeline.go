package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moasq/nanowave/internal/claude"
	"github.com/moasq/nanowave/internal/config"
	"github.com/moasq/nanowave/internal/integrations"
	"github.com/moasq/nanowave/internal/mcpregistry"
	"github.com/moasq/nanowave/internal/terminal"
)

const maxBuildCompletionPasses = 6
const maxPhaseRetries = 2 // retry analyze/plan up to 2 times on transient failures

// retryPhase retries a phase function up to maxRetries times on transient errors.
// Permanent errors (context cancellation) are not retried.
func retryPhase[T any](ctx context.Context, maxRetries int, fn func() (T, error)) (T, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			var zero T
			return zero, ctx.Err()
		}
		result, err := fn()
		if err == nil {
			return result, nil
		}
		lastErr = err
		// Don't retry if the parent context was cancelled
		if ctx.Err() != nil {
			break
		}
	}
	var zero T
	return zero, lastErr
}

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

// coreAgenticTools is the set of non-MCP tools Claude Code always needs.
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
	claude          claude.ClaudeAgent
	config          *config.Config
	model           string                         // user-selected model for code generation (empty = "sonnet")
	manager         *integrations.Manager          // provider-based integration manager (nil = no integrations)
	registry        *mcpregistry.Registry          // internal MCP server registry (apple-docs, xcodegen)
	activeProviders []integrations.ActiveProvider   // resolved providers for current build (transient)
	onStreamEvent   func(claude.StreamEvent)       // optional hook for web UI streaming (nil = CLI-only)
}

// SetManager sets the integration manager for provider-based integrations.
func (p *Pipeline) SetManager(m *integrations.Manager) {
	p.manager = m
}

// SetStreamHook sets an optional callback invoked for every streaming event.
// Used by the web UI to mirror CLI progress in the browser.
func (p *Pipeline) SetStreamHook(hook func(claude.StreamEvent)) {
	p.onStreamEvent = hook
}

// makeStreamCallback wraps the terminal progress callback and the optional web hook.
func (p *Pipeline) makeStreamCallback(progress *terminal.ProgressDisplay) func(claude.StreamEvent) {
	termCb := newProgressCallback(progress)
	if p.onStreamEvent == nil {
		return termCb
	}
	hook := p.onStreamEvent
	return func(ev claude.StreamEvent) {
		termCb(ev)
		hook(ev)
	}
}

// NewPipeline creates a new pipeline orchestrator.
// model overrides the default "sonnet" model for build/edit/fix phases.
func NewPipeline(claudeClient claude.ClaudeAgent, cfg *config.Config, model string) *Pipeline {
	reg := mcpregistry.New()
	mcpregistry.RegisterAll(reg)
	return &Pipeline{
		claude:   claudeClient,
		config:   cfg,
		model:    model,
		registry: reg,
	}
}

// baseAgenticTools returns core tools plus all MCP tools from the registry.
func (p *Pipeline) baseAgenticTools() []string {
	tools := make([]string, len(coreAgenticTools))
	copy(tools, coreAgenticTools)
	tools = append(tools, p.registry.AllTools()...)
	return tools
}

// buildModel returns the model to use for code generation phases.
func (p *Pipeline) buildModel() string {
	if p.model != "" {
		return p.model
	}
	return "sonnet"
}

// QuickIntentCheck runs the intent router and returns the decision.
// Used by Service.Send() to detect ASC intent before entering build/edit.
func (p *Pipeline) QuickIntentCheck(ctx context.Context, prompt string) (*IntentDecision, error) {
	progress := terminal.NewProgressDisplay("intent", 0)
	progress.Start()
	defer progress.Stop()
	return p.decideBuildIntent(ctx, prompt, progress)
}

// Action runs the unified pipeline: intent → analyze → plan → integrations → code.
// For new builds (ac.ProjectDir == ""), it creates a workspace and scaffolds the project.
// For edits (ac.ProjectDir != ""), it runs in the existing workspace with session continuity.
// images is an optional list of image file paths to include in the build prompt.
func (p *Pipeline) Action(ctx context.Context, prompt string, ac ActionContext, images []string) (*BuildResult, error) {
	isEdit := ac.IsEdit()

	// Phase 0: Intent decision (advisory hints for analyzer/planner)
	intentProgress := terminal.NewProgressDisplay("intent", 0)
	intentProgress.Start()

	intentDecision, intentErr := p.decideBuildIntent(ctx, prompt, intentProgress)
	if intentErr != nil {
		intentProgress.StopWithSuccess("Intent hints unavailable — using defaults")
		terminal.Detail("Intent", fmt.Sprintf("Router fallback failed (%v); continuing with defaults", intentErr))
		op := "build"
		if isEdit {
			op = "edit"
		}
		intentDecision = &IntentDecision{
			Operation:        op,
			PlatformHint:     PlatformIOS,
			DeviceFamilyHint: "iphone",
			Confidence:       0.1,
			Reason:           "Router unavailable; using default iOS/iPhone assumptions",
		}
	} else {
		intentProgress.StopWithSuccess("Intent decided")
		if hints := formatIntentHintsForPrompt(intentDecision); hints != "" {
			terminal.Detail("Intent", strings.ReplaceAll(strings.TrimPrefix(hints, "Intent hints (advisory only; explicit user request wins):\n"), "\n", " | "))
		}
	}

	// For edits, override platform hint from existing project config
	if isEdit && ac.Platform != "" {
		intentDecision.PlatformHint = ac.Platform
	}

	// Phase 1: Setup (new builds only)
	if !isEdit {
		spinner := terminal.NewSpinner("Setting up workspace...")
		spinner.Start()
		spinner.StopWithMessage(fmt.Sprintf("%s%s✓%s Workspace ready", terminal.Bold, terminal.Green, terminal.Reset))
	}

	// Phase 2: Analyze (with retry for transient failures)
	analyzeProgress := terminal.NewProgressDisplay("analyze", 0)
	analyzeProgress.Start()

	analysis, err := retryPhase(ctx, maxPhaseRetries, func() (*AnalysisResult, error) {
		return p.analyze(ctx, prompt, intentDecision, ac, analyzeProgress)
	})
	if err != nil {
		analyzeProgress.StopWithError("Analysis failed")
		return nil, fmt.Errorf("analysis failed: %w", err)
	}

	var appName, projectDir string
	if isEdit {
		appName = ac.AppName
		projectDir = ac.ProjectDir
	} else {
		appName = sanitizeToPascalCase(analysis.AppName)
		projectDir = uniqueProjectDir(p.config.ProjectDir, appName)
	}

	analyzeProgress.StopWithSuccess(fmt.Sprintf("Analyzed: %s", analysis.AppName))

	var featureNames []string
	for _, f := range analysis.Features {
		featureNames = append(featureNames, f.Name)
	}
	terminal.Detail("App", analysis.AppName)
	terminal.Detail("Features", strings.Join(featureNames, ", "))
	terminal.Detail("Flow", analysis.CoreFlow)
	if len(analysis.Deferred) > 0 {
		terminal.Detail("Deferred", strings.Join(analysis.Deferred, ", "))
	}

	// Phase 3: Plan (with retry for transient failures)
	planProgress := terminal.NewProgressDisplay("plan", 0)
	planProgress.Start()

	plan, err := retryPhase(ctx, maxPhaseRetries, func() (*PlannerResult, error) {
		return p.plan(ctx, analysis, intentDecision, ac, planProgress)
	})
	if err != nil {
		planProgress.StopWithError("Planning failed")
		return nil, fmt.Errorf("planning failed: %w", err)
	}

	planProgress.StopWithSuccess(fmt.Sprintf("Plan ready (%d files, %d models)", len(plan.Files), len(plan.Models)))

	terminal.Detail("Design", fmt.Sprintf("%s palette, %s font, %s mood",
		plan.Design.Palette.Primary, plan.Design.FontDesign, plan.Design.AppMood))
	if len(plan.Permissions) > 0 {
		var permNames []string
		for _, p := range plan.Permissions {
			permNames = append(permNames, p.Framework)
		}
		terminal.Detail("Permissions", strings.Join(permNames, ", "))
	}

	if isEdit {
		// For edits, ensure project configs are up to date
		p.ensureProjectConfigs(projectDir)
	} else {
		// Set up the actual workspace with the real app name
		if err := p.setupBuildWorkspace(projectDir, appName, plan); err != nil {
			return nil, err
		}
	}

	// Resolve and provision integrations
	provState, err := p.provisionIntegrations(ctx, projectDir, appName, plan, analysis, ac)
	if err != nil {
		return nil, err
	}

	if !isEdit {
		// Scaffold project files and run XcodeGen (new builds only)
		if err := p.scaffoldProject(projectDir, appName, plan, provState.needsAppleSignIn); err != nil {
			return nil, err
		}
	}
	backendProvisioned := provState.backendProvisioned

	// Phase 4: Code + deterministic completion gate
	var (
		report            *FileCompletionReport
		sessionID         = ac.SessionID
		completionPasses  int
		totalCostUSD      float64
		totalInputTokens  int
		totalOutputTokens int
		totalCacheRead    int
		totalCacheCreate  int
	)

	progress := terminal.NewProgressDisplay("build", len(plan.Files))
	progress.Start()

	prevValidCount := 0
	for pass := 1; pass <= maxBuildCompletionPasses; pass++ {
		passLabel := fmt.Sprintf("Generation pass %d", pass)

		var resp *claude.Response
		if pass == 1 {
			resp, err = p.buildStreaming(ctx, prompt, appName, projectDir, analysis, plan, sessionID, progress, images, backendProvisioned, ac)
		} else {
			resp, err = p.completeMissingFilesStreaming(ctx, appName, projectDir, plan, report, sessionID, progress)
		}
		if err != nil {
			progress.StopWithError(passLabel + " failed")
			return nil, fmt.Errorf("%s failed: %w", strings.ToLower(passLabel), err)
		}

		totalCostUSD += resp.TotalCostUSD
		totalInputTokens += resp.Usage.InputTokens
		totalOutputTokens += resp.Usage.OutputTokens
		totalCacheRead += resp.Usage.CacheReadInputTokens
		totalCacheCreate += resp.Usage.CacheCreationInputTokens
		if resp.SessionID != "" {
			sessionID = resp.SessionID
		}

		if !isEdit {
			// Clean up scaffold placeholders now that real code has been written.
			cleanupScaffoldPlaceholders(projectDir, appName, plan)
		}

		report, err = verifyPlannedFiles(projectDir, appName, plan)
		if err != nil {
			progress.StopWithError("File verification failed")
			return nil, fmt.Errorf("file completion check failed: %w", err)
		}

		retry, retryErr := shouldRetryCompletion(report, pass, maxBuildCompletionPasses)
		if retryErr != nil {
			progress.StopWithError(passLabel + " incomplete")
			return nil, retryErr
		}
		if !retry {
			completionPasses = pass
			break
		}

		// Plateau detection: if valid file count hasn't increased, fail early
		if pass > 1 && report.ValidCount <= prevValidCount {
			progress.StopWithError(fmt.Sprintf("No progress after pass %d (%d/%d files valid)", pass, report.ValidCount, report.TotalPlanned))
			return nil, fmt.Errorf("file completion stalled after %d passes — %d/%d files valid, no improvement:\n%s",
				pass, report.ValidCount, report.TotalPlanned, formatIncompleteReport(report))
		}
		prevValidCount = report.ValidCount

		remaining := len(report.Missing) + len(report.Invalid)
		progress.SetTotalFiles(report.TotalPlanned)
		progress.ResetForRetry()
		progress.SetStatus(fmt.Sprintf("%d planned files unresolved — pass %d...", remaining, pass+1))
	}

	if completionPasses == 0 {
		progress.StopWithError("Build did not reach a terminal state")
		return nil, fmt.Errorf("file completion check failed: build did not reach a terminal state")
	}

	progress.StopWithSuccess(fmt.Sprintf("Build complete — %d files", report.ValidCount))
	terminal.Detail("Cost", fmt.Sprintf("$%.4f (total across %d passes)", totalCostUSD, completionPasses))

	// Phase 5: Finalize (git init + commit — new builds only)
	if !isEdit {
		p.finalize(ctx, projectDir, appName)
	}

	var resultPlatforms []string
	if plan.IsMultiPlatform() {
		resultPlatforms = plan.GetPlatforms()
	}

	return &BuildResult{
		AppName:           analysis.AppName,
		Description:       analysis.Description,
		ProjectDir:        projectDir,
		BundleID:          fmt.Sprintf("%s.%s", bundleIDPrefix(), strings.ToLower(appName)),
		DeviceFamily:      plan.GetDeviceFamily(),
		Platform:          plan.GetPlatform(),
		Platforms:         resultPlatforms,
		WatchProjectShape: plan.GetWatchProjectShape(),
		Features:          analysis.Features,
		FileCount:         len(plan.Files),
		PlannedFiles:      len(plan.Files),
		CompletedFiles:    report.ValidCount,
		CompletionPasses:  completionPasses,
		SessionID:         sessionID,
		TotalCostUSD:      totalCostUSD,
		InputTokens:       totalInputTokens,
		OutputTokens:      totalOutputTokens,
		CacheRead:         totalCacheRead,
		CacheCreated:      totalCacheCreate,
	}, nil
}
