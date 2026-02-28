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

// canonicalBuildDestination returns the Xcode build destination for the given platform.
func canonicalBuildDestination(platform string) string {
	return canonicalBuildDestinationForShape(platform, "")
}

// detectProjectPlatform reads project_config.json and extracts the platform field.
// Returns "ios" if missing or unreadable (backward compat).
func detectProjectPlatform(projectDir string) string {
	platform, _, _ := detectProjectBuildHints(projectDir)
	return platform
}

// detectProjectBuildHints reads project_config.json and extracts build-relevant hints.
// Returns ("ios", nil, "") when missing or unreadable (backward compat).
func detectProjectBuildHints(projectDir string) (platform string, platforms []string, watchProjectShape string) {
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

// readProjectAppName returns the Xcode app name for an existing project.
// It reads app_name from project_config.json (the canonical source of truth written at build time).
// Falls back to filepath.Base(projectDir) for projects predating the suffixed-dir feature.
func readProjectAppName(projectDir string) string {
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

// baseAgenticTools is the set of tools Claude Code needs for writing code, building, and fixing.
var baseAgenticTools = []string{
	"Write", "Edit", "Read", "Bash", "Glob", "Grep",
	"WebFetch", "WebSearch",
	// Apple docs
	"mcp__apple-docs__search_apple_docs",
	"mcp__apple-docs__get_apple_doc_content",
	"mcp__apple-docs__search_framework_symbols",
	"mcp__apple-docs__get_sample_code",
	"mcp__apple-docs__get_related_apis",
	"mcp__apple-docs__find_similar_apis",
	"mcp__apple-docs__get_platform_compatibility",
	// XcodeGen project config
	"mcp__xcodegen__add_permission",
	"mcp__xcodegen__add_extension",
	"mcp__xcodegen__add_entitlement",
	"mcp__xcodegen__add_localization",
	"mcp__xcodegen__set_build_setting",
	"mcp__xcodegen__get_project_config",
	"mcp__xcodegen__add_package",
	"mcp__xcodegen__regenerate_project",
}

// Pipeline orchestrates the multi-phase app generation process.
type Pipeline struct {
	claude          *claude.Client
	config          *config.Config
	model           string                    // user-selected model for code generation (empty = "sonnet")
	manager         *integrations.Manager          // provider-based integration manager (nil = no integrations)
	activeProviders []integrations.ActiveProvider // resolved providers for current build (transient)
}

// SetManager sets the integration manager for provider-based integrations.
func (p *Pipeline) SetManager(m *integrations.Manager) {
	p.manager = m
}

// NewPipeline creates a new pipeline orchestrator.
// model overrides the default "sonnet" model for build/edit/fix phases.
func NewPipeline(claudeClient *claude.Client, cfg *config.Config, model string) *Pipeline {
	return &Pipeline{
		claude: claudeClient,
		config: cfg,
		model:  model,
	}
}

// buildModel returns the model to use for code generation phases.
func (p *Pipeline) buildModel() string {
	if p.model != "" {
		return p.model
	}
	return "sonnet"
}

// Build runs the full pipeline: intent → setup → analyze → plan → build+fix → finalize.
// images is an optional list of image file paths to include in the build prompt.
func (p *Pipeline) Build(ctx context.Context, prompt string, images []string) (*BuildResult, error) {
	// Phase 0: Intent decision (advisory hints for analyzer/planner)
	intentProgress := terminal.NewProgressDisplay("intent", 0)
	intentProgress.Start()

	intentDecision, intentErr := p.decideBuildIntent(ctx, prompt, intentProgress)
	if intentErr != nil {
		intentProgress.StopWithSuccess("Intent hints unavailable — using defaults")
		terminal.Detail("Intent", fmt.Sprintf("Router fallback failed (%v); continuing with defaults", intentErr))
		intentDecision = &IntentDecision{
			Operation:        "build",
			PlatformHint:     PlatformIOS,
			DeviceFamilyHint: "iphone",
			Confidence:       0.1,
			Reason:           "Router unavailable; using default iOS/iPhone build assumptions",
		}
	} else {
		intentProgress.StopWithSuccess("Intent decided")
		if hints := formatIntentHintsForPrompt(intentDecision); hints != "" {
			terminal.Detail("Intent", strings.ReplaceAll(strings.TrimPrefix(hints, "Intent hints (advisory only; explicit user request wins):\n"), "\n", " | "))
		}
	}

	// Phase 1: Setup workspace
	spinner := terminal.NewSpinner("Setting up workspace...")
	spinner.Start()

	appName := "MyApp" // placeholder until analyzer names it
	projectDir := filepath.Join(p.config.ProjectDir, appName)

	// We'll rename after analysis — for now create a temp structure
	// Phase 2 will give us the real name

	spinner.StopWithMessage(fmt.Sprintf("%s%s✓%s Workspace ready", terminal.Bold, terminal.Green, terminal.Reset))

	// Phase 2: Analyze (with retry for transient failures)
	analyzeProgress := terminal.NewProgressDisplay("analyze", 0)
	analyzeProgress.Start()

	analysis, err := retryPhase(ctx, maxPhaseRetries, func() (*AnalysisResult, error) {
		return p.analyze(ctx, prompt, intentDecision, analyzeProgress)
	})
	if err != nil {
		analyzeProgress.StopWithError("Analysis failed")
		return nil, fmt.Errorf("analysis failed: %w", err)
	}

	appName = sanitizeToPascalCase(analysis.AppName)
	projectDir = uniqueProjectDir(p.config.ProjectDir, appName)
	// appName stays clean (e.g. "Skies") for Xcode project name, bundle ID, scheme, source dirs.
	// projectDir may have a numeric suffix (e.g. ".../Skies2") to avoid collisions on disk.

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
		return p.plan(ctx, analysis, intentDecision, planProgress)
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

	// Now set up the actual workspace with the real app name
	if err := setupWorkspace(projectDir); err != nil {
		return nil, fmt.Errorf("workspace setup failed: %w", err)
	}

	if err := writeInitialCLAUDEMD(projectDir, appName, plan.GetPlatform(), plan.GetDeviceFamily()); err != nil {
		return nil, fmt.Errorf("failed to write CLAUDE.md: %w", err)
	}

	if err := enrichCLAUDEMD(projectDir, plan, appName); err != nil {
		return nil, fmt.Errorf("failed to enrich CLAUDE.md: %w", err)
	}

	if err := writeCoreRules(projectDir, plan.GetPlatform(), plan.Packages); err != nil {
		return nil, fmt.Errorf("failed to write core rules: %w", err)
	}

	if plan.IsMultiPlatform() {
		platforms := plan.GetPlatforms()
		if err := writeAlwaysSkills(projectDir, platforms[0], platforms[1:]...); err != nil {
			return nil, fmt.Errorf("failed to write always skills: %w", err)
		}
	} else {
		if err := writeAlwaysSkills(projectDir, plan.GetPlatform()); err != nil {
			return nil, fmt.Errorf("failed to write always skills: %w", err)
		}
	}

	// Auto-inject adaptive-layout skill for iPad/universal apps (iOS only)
	if plan.GetPlatform() == PlatformIOS {
		if family := plan.GetDeviceFamily(); family == "ipad" || family == "universal" {
			hasAdaptive := false
			for _, k := range plan.RuleKeys {
				if k == "adaptive-layout" {
					hasAdaptive = true
					break
				}
			}
			if !hasAdaptive {
				plan.RuleKeys = append(plan.RuleKeys, "adaptive-layout")
			}
		}
	}

	if err := writeConditionalSkills(projectDir, plan.RuleKeys, plan.GetPlatform()); err != nil {
		return nil, fmt.Errorf("failed to write conditional skills: %w", err)
	}

	scaffoldPlatform := plan.GetPlatform()
	scaffoldShape := plan.GetWatchProjectShape()
	if plan.IsMultiPlatform() {
		// For multi-platform, scaffold uses the primary platform (iOS)
		scaffoldPlatform = PlatformIOS
		scaffoldShape = ""
	}
	if err := writeClaudeProjectScaffoldWithShape(projectDir, appName, scaffoldPlatform, scaffoldShape); err != nil {
		return nil, fmt.Errorf("failed to write Claude project scaffold: %w", err)
	}

	// Resolve active integrations from planner output
	var activeIntegrationIDs []string
	var activeProviders []integrations.ActiveProvider
	backendProvisioned := false
	needsAppleSignIn := false

	if p.manager != nil && len(plan.Integrations) > 0 {
		terminal.Info(fmt.Sprintf("Resolving %d integration(s): %s", len(plan.Integrations), strings.Join(plan.Integrations, ", ")))
		ui := &pipelineSetupUI{}
		resolved, err := p.manager.Resolve(ctx, appName, plan.Integrations, ui)
		if err != nil {
			terminal.Warning(fmt.Sprintf("Integration resolution failed: %v", err))
		}
		activeProviders = resolved
		p.activeProviders = resolved // store for buildStreaming access
		for _, ap := range activeProviders {
			activeIntegrationIDs = append(activeIntegrationIDs, string(ap.Provider.ID()))
		}
		terminal.Detail("Active integrations", fmt.Sprintf("%d: %s", len(activeIntegrationIDs), strings.Join(activeIntegrationIDs, ", ")))

		// Provision via Manager
		if len(activeProviders) > 0 && (analysis.BackendNeeds != nil && analysis.BackendNeeds.NeedsBackend() || plan.MonetizationPlan != nil) {
			var authMethods []string
			if analysis.BackendNeeds != nil {
				authMethods = analysis.BackendNeeds.AuthMethods
				if analysis.BackendNeeds.Auth && len(authMethods) == 0 {
					authMethods = []string{"email", "anonymous"}
				}
			}
			provReq := integrations.ProvisionRequest{
				AppName:       appName,
				BundleID:      fmt.Sprintf("%s.%s", bundleIDPrefix(), strings.ToLower(appName)),
				Models:        modelsToModelRefs(plan.Models),
				AuthMethods:   authMethods,
				NeedsAuth:     analysis.BackendNeeds != nil && analysis.BackendNeeds.Auth,
				NeedsDB:       analysis.BackendNeeds != nil && (analysis.BackendNeeds.DB || len(plan.Models) > 0),
				NeedsStorage:  analysis.BackendNeeds != nil && analysis.BackendNeeds.Storage,
				NeedsRealtime: analysis.BackendNeeds != nil && analysis.BackendNeeds.Realtime,
			}
			// Wire monetization plan for RevenueCat provisioning
			if plan.MonetizationPlan != nil {
				provReq.NeedsMonetization = true
				provReq.MonetizationType = plan.MonetizationPlan.Model
				provReq.MonetizationPlan = monetizationPlanToRef(plan.MonetizationPlan)
			}
			provResult, err := p.manager.Provision(ctx, provReq, activeProviders)
			if err != nil {
				terminal.Warning(fmt.Sprintf("Provisioning failed: %v", err))
			} else if provResult != nil {
				backendProvisioned = provResult.BackendProvisioned
				needsAppleSignIn = provResult.NeedsAppleSignIn
				for _, w := range provResult.Warnings {
					terminal.Warning(w)
				}
				if len(provResult.TablesCreated) > 0 {
					terminal.Success(fmt.Sprintf("Tables created: %s", strings.Join(provResult.TablesCreated, ", ")))
				}
			}
		}

		// Generate StoreKit configuration file for local testing
		if plan.MonetizationPlan != nil {
			if err := writeStoreKitConfig(projectDir, appName, plan.MonetizationPlan); err != nil {
				terminal.Warning(fmt.Sprintf("StoreKit config generation failed: %v", err))
			} else {
				terminal.Success("StoreKit configuration file generated")
			}
		}

		// Write MCP config via Manager
		mcpConfigs, err := p.manager.MCPConfigs(ctx, activeProviders)
		if err != nil {
			terminal.Warning(fmt.Sprintf("MCP config generation failed: %v", err))
		}
		if err := writeMCPConfig(projectDir, mcpConfigs); err != nil {
			return nil, fmt.Errorf("failed to write MCP config: %w", err)
		}
		terminal.Detail("MCP config", fmt.Sprintf("written to %s/.mcp.json (%d integrations)", projectDir, len(mcpConfigs)))

		// Write settings with Manager tool allowlist
		mcpTools := p.manager.MCPToolAllowlist(activeProviders)
		if err := writeSettingsShared(projectDir, mcpTools); err != nil {
			return nil, fmt.Errorf("failed to update settings with integration permissions: %w", err)
		}
	} else {
		terminal.Detail("Integrations", "none in plan")
	}

	if err := writeSettingsLocal(projectDir); err != nil {
		return nil, fmt.Errorf("failed to write settings: %w", err)
	}

	// Write project_config.json first (source of truth for XcodeGen MCP server).
	if err := writeProjectConfig(projectDir, plan, appName); err != nil {
		return nil, fmt.Errorf("failed to write project_config.json: %w", err)
	}

	// Auto-add Apple Sign-In entitlement to project_config.json when apple auth is detected.
	// This ensures the XcodeGen MCP server preserves it on future regenerations.
	if needsAppleSignIn {
		if err := addAutoEntitlement(projectDir, "com.apple.developer.applesignin", []any{"Default"}, ""); err != nil {
			terminal.Warning(fmt.Sprintf("Could not add Apple Sign-In entitlement: %v", err))
		} else {
			terminal.Success("Apple Sign-In entitlement added to project config")
		}
	}

	// Read back entitlements from project_config.json so project.yml includes them
	// in the entitlements properties section. This ensures XcodeGen writes the correct
	// .entitlements plist, and regenerate_project preserves them.
	mainEntitlements := readConfigEntitlements(projectDir, "")

	if err := writeProjectYML(projectDir, plan, appName, mainEntitlements); err != nil {
		return nil, fmt.Errorf("failed to write project.yml: %w", err)
	}

	if err := writeGitignore(projectDir); err != nil {
		return nil, fmt.Errorf("failed to write .gitignore: %w", err)
	}

	if plan.IsMultiPlatform() {
		// Multi-platform: one asset catalog per platform source dir
		for _, plat := range plan.GetPlatforms() {
			suffix := PlatformSourceDirSuffix(plat)
			dirName := appName + suffix
			if err := writeAssetCatalog(projectDir, dirName, plat); err != nil {
				return nil, fmt.Errorf("failed to write %s asset catalog: %w", PlatformDisplayName(plat), err)
			}
		}
	} else if IsWatchOS(plan.GetPlatform()) && plan.GetWatchProjectShape() == WatchShapePaired {
		// Paired: iOS asset catalog for the main app, watchOS for the watch app
		if err := writeAssetCatalog(projectDir, appName, PlatformIOS); err != nil {
			return nil, fmt.Errorf("failed to write asset catalog: %w", err)
		}
		if err := writeAssetCatalog(projectDir, appName+"Watch", PlatformWatchOS); err != nil {
			return nil, fmt.Errorf("failed to write watch asset catalog: %w", err)
		}
	} else {
		if err := writeAssetCatalog(projectDir, appName, plan.GetPlatform()); err != nil {
			return nil, fmt.Errorf("failed to write asset catalog: %w", err)
		}
	}

	if err := scaffoldSourceDirs(projectDir, appName, plan); err != nil {
		return nil, fmt.Errorf("failed to scaffold source dirs: %w", err)
	}

	if err := runXcodeGen(projectDir); err != nil {
		return nil, fmt.Errorf("failed to run xcodegen: %w", err)
	}

	// Phase 4: Build + deterministic completion gate
	var (
		report            *FileCompletionReport
		sessionID         string
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
			resp, err = p.buildStreaming(ctx, prompt, appName, projectDir, analysis, plan, sessionID, progress, images, backendProvisioned)
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

		// Clean up scaffold placeholders now that real code has been written.
		// These trip the quality-gate hook and confuse the coding agent.
		cleanupScaffoldPlaceholders(projectDir, appName, plan)

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
		// to avoid wasting turns on unresolvable files.
		if pass > 1 && report.ValidCount <= prevValidCount {
			progress.StopWithError(fmt.Sprintf("No progress after pass %d (%d/%d files valid)", pass, report.ValidCount, report.TotalPlanned))
			return nil, fmt.Errorf("file completion stalled after %d passes — %d/%d files valid, no improvement:\n%s",
				pass, report.ValidCount, report.TotalPlanned, formatIncompleteReport(report))
		}
		prevValidCount = report.ValidCount

		// Prepare progress display for next pass — keep cumulative file count,
		// update total to include any newly discovered files, reset transient state.
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

	// Phase 5: Finalize (git init + commit)
	p.finalize(ctx, projectDir, appName)

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
