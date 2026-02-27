package orchestration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
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

// supabaseAgenticTools are Supabase MCP tools appended when Supabase is active.
var supabaseAgenticTools = []string{
	"mcp__supabase__execute_sql",
	"mcp__supabase__list_tables",
	"mcp__supabase__apply_migration",
	"mcp__supabase__list_storage_buckets",
	"mcp__supabase__get_project_url",
	"mcp__supabase__get_anon_key",
	"mcp__supabase__get_logs",
	"mcp__supabase__configure_auth_providers",
	"mcp__supabase__get_auth_config",
}

// buildAgenticTools returns the tool allowlist, optionally including integration-specific tools.
// integrationIDs uses typed ProviderID constants via string values (String Matching Policy compliant).
func buildAgenticTools(integrationIDs []string) []string {
	tools := make([]string, len(baseAgenticTools))
	copy(tools, baseAgenticTools)

	for _, id := range integrationIDs {
		switch id {
		case "supabase":
			tools = append(tools, supabaseAgenticTools...)
		}
	}
	return tools
}

// Pipeline orchestrates the multi-phase app generation process.
type Pipeline struct {
	claude *claude.Client
	config *config.Config
	model  string // user-selected model for code generation (empty = "sonnet")
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
	var mcpIntegrations []mcpIntegrationConfig
	var activeIntegrationIDs []string
	if len(plan.Integrations) > 0 {
		terminal.Info(fmt.Sprintf("Resolving %d integration(s): %s", len(plan.Integrations), strings.Join(plan.Integrations, ", ")))
		integStore := loadGlobalIntegrationStore()
		for _, id := range plan.Integrations {
			cfg := loadIntegrationConfig(integStore, id, appName)
			if cfg == nil {
				terminal.Detail("Integration lookup", fmt.Sprintf("%s: no per-app config for %q", id, appName))
				// No per-app config yet — run auto-setup if Supabase CLI is available.
				// SetupSupabase handles login + PAT detection internally.
				if id == "supabase" && config.CheckSupabaseCLI() {
					terminal.Info(fmt.Sprintf("Setting up Supabase for %s...", appName))
					if err := runInlineIntegrationSetup(integStore, id, appName, false); err != nil {
						terminal.Error(fmt.Sprintf("Auto-setup failed: %v", err))
						cfg = promptIntegrationSetup(integStore, id, appName)
					} else {
						cfg = loadIntegrationConfig(integStore, id, appName)
					}
				} else {
					// Supabase CLI not installed — show picker with setup options
					terminal.Warning(fmt.Sprintf("%s integration needed — let's set it up", id))
					cfg = promptIntegrationSetup(integStore, id, appName)
				}
				if cfg == nil {
					terminal.Info("Continuing without backend — using placeholder config")
					continue
				}
			} else if id == "supabase" {
				// Config found from previous build — always notify the user
				terminal.Success(fmt.Sprintf("Supabase connected (project: %s)", cfg.ProjectRef))
				terminal.Detail("Config details", fmt.Sprintf("URL=%s, has_anon_key=%t, has_PAT=%t",
					cfg.ProjectURL, cfg.AnonKey != "", cfg.PAT != ""))
				// If PAT is missing, the MCP server can't manage the backend
				if cfg.PAT == "" {
					terminal.Warning("Supabase PAT is missing — MCP tools will not work. Re-running setup...")
					if err := runInlineIntegrationSetup(integStore, id, appName, false); err != nil {
						terminal.Error(fmt.Sprintf("Setup failed: %v", err))
					} else {
						cfg = loadIntegrationConfig(integStore, id, appName)
					}
				}
			}
			if cfg == nil {
				terminal.Info("Continuing without backend — using placeholder config")
				continue
			}
			terminal.Detail("Integration active", fmt.Sprintf("%s: ref=%s, PAT=%t", id, cfg.ProjectRef, cfg.PAT != ""))
			mcpIntegrations = append(mcpIntegrations, mcpIntegrationConfig{
				ProviderID: id,
				PAT:        cfg.PAT,
				ProjectRef: cfg.ProjectRef,
			})
			activeIntegrationIDs = append(activeIntegrationIDs, id)
		}
		terminal.Detail("Active integrations", fmt.Sprintf("%d: %s", len(activeIntegrationIDs), strings.Join(activeIntegrationIDs, ", ")))
	} else {
		terminal.Detail("Integrations", "none in plan")
	}

	// Auto-provision Supabase backend: auth providers, tables, RLS, storage
	backendProvisioned := false
	needsAppleSignIn := false
	for _, mc := range mcpIntegrations {
		if mc.ProviderID != "supabase" || mc.PAT == "" {
			continue
		}
		sc := &supabaseAPIClient{pat: mc.PAT, projectRef: mc.ProjectRef}

		// 1. Auth providers
		if analysis.BackendNeeds != nil && analysis.BackendNeeds.Auth {
			terminal.Info(fmt.Sprintf("Auth needed — configuring providers (auth_methods from analysis: %v)", analysis.BackendNeeds.AuthMethods))
			bundleID := fmt.Sprintf("%s.%s", bundleIDPrefix(), strings.ToLower(appName))
			authMethods := analysis.BackendNeeds.AuthMethods
			if len(authMethods) == 0 {
				authMethods = []string{"email", "anonymous"}
			}
			for _, m := range authMethods {
				if m == "apple" {
					needsAppleSignIn = true
					break
				}
			}
			terminal.Detail("Auth config", fmt.Sprintf("bundle_id=%s, methods=%s", bundleID, strings.Join(authMethods, ", ")))
			if err := configureSupabaseAuth(mc.PAT, mc.ProjectRef, bundleID, authMethods); err != nil {
				terminal.Warning(fmt.Sprintf("Could not auto-configure auth providers: %v", err))
			} else {
				terminal.Success(fmt.Sprintf("Auth providers configured: %s", strings.Join(authMethods, ", ")))
			}
		}

		// 2. Create tables from planner models
		if len(plan.Models) > 0 {
			terminal.Info(fmt.Sprintf("Provisioning %d tables on Supabase...", len(plan.Models)))
			sql := generateCreateTablesSQL(plan.Models)
			terminal.Detail("SQL", fmt.Sprintf("%d chars for %d tables", len(sql), len(plan.Models)))
			if err := sc.executeSQL(sql); err != nil {
				terminal.Warning(fmt.Sprintf("Table creation failed: %v", err))
			} else {
				terminal.Success(fmt.Sprintf("Tables created: %s", modelTableNames(plan.Models)))
				backendProvisioned = true
			}

			// 3. Enable RLS on all tables
			rlsSQL := generateEnableRLSSQL(plan.Models)
			if err := sc.executeSQL(rlsSQL); err != nil {
				terminal.Warning(fmt.Sprintf("RLS enable failed: %v", err))
			} else {
				terminal.Success("Row Level Security enabled on all tables")
			}

			// 4. Create basic RLS policies
			policySQL := generateRLSPoliciesSQL(plan.Models)
			if err := sc.executeSQL(policySQL); err != nil {
				terminal.Warning(fmt.Sprintf("RLS policies failed: %v", err))
			} else {
				terminal.Success("RLS policies created")
			}
		}

		// 5. Create storage bucket if app needs file uploads
		if analysis.BackendNeeds != nil && analysis.BackendNeeds.Storage {
			bucketID := strings.ToLower(appName) + "-media"
			terminal.Info("Creating storage bucket for file uploads...")
			bucketSQL := fmt.Sprintf(`INSERT INTO storage.buckets (id, name, public) VALUES ('%s', '%s', true) ON CONFLICT (id) DO NOTHING;`, bucketID, bucketID)
			if err := sc.executeSQL(bucketSQL); err != nil {
				terminal.Warning(fmt.Sprintf("Storage bucket creation failed: %v", err))
			} else {
				terminal.Success(fmt.Sprintf("Storage bucket created: %s", bucketID))

				// 6. Create storage policies for the bucket
				policySQL := generateStoragePoliciesSQL(bucketID)
				if err := sc.executeSQL(policySQL); err != nil {
					terminal.Warning(fmt.Sprintf("Storage policies failed: %v", err))
				} else {
					terminal.Success("Storage bucket policies created (per-user folder access)")
				}
			}
		}

		// 7. Enable Realtime on tables if analysis requests it
		if analysis.BackendNeeds != nil && analysis.BackendNeeds.Realtime && len(plan.Models) > 0 {
			terminal.Info("Enabling Realtime on tables...")
			realtimeSQL := generateRealtimeSQL(plan.Models)
			if err := sc.executeSQL(realtimeSQL); err != nil {
				terminal.Warning(fmt.Sprintf("Realtime enable failed: %v", err))
			} else {
				terminal.Success(fmt.Sprintf("Realtime enabled on %d tables", len(plan.Models)))
			}
		}
	}
	if len(mcpIntegrations) > 0 && !backendProvisioned {
		if analysis.BackendNeeds == nil {
			terminal.Detail("Backend provisioning", "no backend_needs in analysis — skipped")
		} else {
			terminal.Detail("Backend provisioning", "skipped (no PAT or no models)")
		}
	}

	if err := writeMCPConfig(projectDir, mcpIntegrations...); err != nil {
		return nil, fmt.Errorf("failed to write MCP config: %w", err)
	}
	terminal.Detail("MCP config", fmt.Sprintf("written to %s/.mcp.json (%d integrations)", projectDir, len(mcpIntegrations)))

	// Re-write shared settings with integration permissions if any are active
	if len(activeIntegrationIDs) > 0 {
		if err := writeSettingsShared(projectDir, activeIntegrationIDs...); err != nil {
			return nil, fmt.Errorf("failed to update settings with integration permissions: %w", err)
		}
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
			resp, err = p.buildStreaming(ctx, prompt, appName, projectDir, analysis, plan, sessionID, progress, images, activeIntegrationIDs, backendProvisioned)
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

// EditResult holds the output of an Edit operation.
type EditResult struct {
	Summary      string
	SessionID    string
	TotalCostUSD float64
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheCreated int
}

// Edit modifies an existing project using Claude Code.
// images is an optional list of image file paths to include in the edit prompt.
func (p *Pipeline) Edit(ctx context.Context, prompt, projectDir, sessionID string, images []string) (*EditResult, error) {
	appName := readProjectAppName(projectDir)
	ensureMCPConfig(projectDir)

	platform, platforms, watchProjectShape := detectProjectBuildHints(projectDir)
	isMulti := len(platforms) > 1

	appendPrompt, err := composeCoderAppendPrompt("editor", platform)
	if err != nil {
		return nil, err
	}

	var userMsg string
	if isMulti {
		buildCmds := multiPlatformBuildCommands(appName, platforms)
		var buildCmdStr strings.Builder
		for i, cmd := range buildCmds {
			fmt.Fprintf(&buildCmdStr, "%d. %s\n", i+1, cmd)
		}

		appendPrompt += "\n\nBuild commands (run ALL):\n" + buildCmdStr.String()

		userMsg = fmt.Sprintf(`Edit this multi-platform app based on the following request:

%s

This project targets: %s

After making changes:
1. If you need new permissions, extensions, or entitlements, use the xcodegen MCP tools (add_permission, add_extension, etc.)
2. If adding a new platform, create the source directory, write the @main entry point, use xcodegen MCP tools to add the target, then regenerate
3. Build each scheme in sequence:
%s4. If any build fails, read the errors carefully, fix the code, and rebuild
5. If Xcode says a scheme is missing, run: xcodebuild -list -project %s.xcodeproj and use the listed schemes
6. Repeat until all builds succeed`, prompt, strings.Join(platforms, ", "), buildCmdStr.String(), appName)
	} else {
		destination := canonicalBuildDestinationForShape(platform, watchProjectShape)
		appendPrompt += fmt.Sprintf("\n\nBuild command:\nxcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet build", appName, appName, destination)

		userMsg = fmt.Sprintf(`Edit this app based on the following request:

%s

After making changes:
1. If you need new permissions, extensions, or entitlements, use the xcodegen MCP tools (add_permission, add_extension, etc.)
2. Run: xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet build
3. If build fails, read the errors carefully, fix the code, and rebuild
4. If Xcode says the scheme is missing, run: xcodebuild -list -project %s.xcodeproj and use the listed app scheme
5. Repeat until the build succeeds`, prompt, appName, appName, destination, appName)
	}

	progress := terminal.NewProgressDisplay("edit", 0)
	progress.Start()

	resp, err := p.claude.GenerateStreaming(ctx, userMsg, claude.GenerateOpts{
		AppendSystemPrompt: appendPrompt,
		MaxTurns:           30,
		Model:              p.buildModel(),
		WorkDir:            projectDir,
		AllowedTools:       buildAgenticTools(nil),
		SessionID:          sessionID,
		Images:             images,
	}, newProgressCallback(progress))

	if err != nil {
		progress.StopWithError("Edit failed")
		return nil, fmt.Errorf("edit failed: %w", err)
	}

	progress.StopWithSuccess("Changes applied!")
	showCost(resp)

	return &EditResult{
		Summary:      resp.Result,
		SessionID:    resp.SessionID,
		TotalCostUSD: resp.TotalCostUSD,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
		CacheRead:    resp.Usage.CacheReadInputTokens,
		CacheCreated: resp.Usage.CacheCreationInputTokens,
	}, nil
}

// FixResult holds the output of a Fix operation.
type FixResult struct {
	Summary      string
	SessionID    string
	TotalCostUSD float64
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheCreated int
}

// Fix auto-fixes build errors in an existing project.
func (p *Pipeline) Fix(ctx context.Context, projectDir, sessionID string) (*FixResult, error) {
	appName := readProjectAppName(projectDir)
	ensureMCPConfig(projectDir)

	platform, platforms, watchProjectShape := detectProjectBuildHints(projectDir)
	isMulti := len(platforms) > 1

	appendPrompt, err := composeCoderAppendPrompt("fixer", platform)
	if err != nil {
		return nil, err
	}

	var userMsg string
	if isMulti {
		buildCmds := multiPlatformBuildCommands(appName, platforms)
		var buildCmdStr strings.Builder
		for i, cmd := range buildCmds {
			fmt.Fprintf(&buildCmdStr, "%d. %s\n", i+1, cmd)
		}

		userMsg = fmt.Sprintf(`Fix any build errors in this multi-platform project.

This project targets: %s

1. Build each scheme in sequence:
%s2. Read the error output carefully
3. Investigate: read the relevant source files to understand context
4. Fix the errors in the Swift source code
5. If the error is a project configuration issue, use the xcodegen MCP tools (add_permission, add_extension, regenerate_project, etc.)
6. If Xcode says a scheme is missing, run: xcodebuild -list -project %s.xcodeproj and use the listed schemes
7. Rebuild and repeat until all builds succeed`, strings.Join(platforms, ", "), buildCmdStr.String(), appName)
	} else {
		destination := canonicalBuildDestinationForShape(platform, watchProjectShape)
		userMsg = fmt.Sprintf(`Fix any build errors in this project.

1. Run: xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet build
2. Read the error output carefully
3. Investigate: read the relevant source files to understand context
4. Fix the errors in the Swift source code
5. If the error is a project configuration issue, use the xcodegen MCP tools (add_permission, add_extension, regenerate_project, etc.)
6. If Xcode says the scheme is missing, run: xcodebuild -list -project %s.xcodeproj and use the listed app scheme
7. Rebuild and repeat until the build succeeds`, appName, appName, destination, appName)
	}

	progress := terminal.NewProgressDisplay("fix", 0)
	progress.Start()

	resp, err := p.claude.GenerateStreaming(ctx, userMsg, claude.GenerateOpts{
		AppendSystemPrompt: appendPrompt,
		MaxTurns:           30,
		Model:              p.buildModel(),
		WorkDir:            projectDir,
		AllowedTools:       buildAgenticTools(nil),
		SessionID:          sessionID,
	}, newProgressCallback(progress))

	if err != nil {
		progress.StopWithError("Fix failed")
		return nil, fmt.Errorf("fix failed: %w", err)
	}

	progress.StopWithSuccess("Fix applied")
	showCost(resp)

	return &FixResult{
		Summary:      resp.Result,
		SessionID:    resp.SessionID,
		TotalCostUSD: resp.TotalCostUSD,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
		CacheRead:    resp.Usage.CacheReadInputTokens,
		CacheCreated: resp.Usage.CacheCreationInputTokens,
	}, nil
}

// analyze runs Phase 2: prompt → AnalysisResult.
func (p *Pipeline) analyze(ctx context.Context, prompt string, intent *IntentDecision, progress *terminal.ProgressDisplay) (*AnalysisResult, error) {
	systemPrompt, err := composeAnalyzerSystemPrompt(intent)
	if err != nil {
		return nil, err
	}

	progress.AddActivity("Sending request to Claude")

	gotFirstDelta := false
	resp, err := p.claude.GenerateStreaming(ctx, prompt, claude.GenerateOpts{
		SystemPrompt: systemPrompt,
		MaxTurns:     3,
		Model:        "sonnet",
	}, func(ev claude.StreamEvent) {
		switch ev.Type {
		case "system":
			progress.AddActivity("Connected to Claude")
		case "content_block_delta":
			if ev.Text != "" {
				if !gotFirstDelta {
					gotFirstDelta = true
					progress.AddActivity("Identifying features and requirements")
				}
				progress.OnStreamingText(ev.Text)
			}
		case "assistant":
			if ev.Text != "" {
				progress.OnAssistantText(ev.Text)
			}
		case "tool_use":
			if ev.ToolName != "" {
				progress.OnToolUse(ev.ToolName, func(key string) string {
					return extractToolInputString(ev.ToolInput, key)
				})
			}
		}
	})
	if err != nil {
		return nil, err
	}

	resultText := ""
	if resp != nil {
		resultText = resp.Result
	}

	if strings.TrimSpace(resultText) == "" {
		return nil, fmt.Errorf("analysis returned empty response — the model may have failed to generate output")
	}

	return parseAnalysis(resultText)
}

// plan runs Phase 3: analysis → PlannerResult.
func (p *Pipeline) plan(ctx context.Context, analysis *AnalysisResult, intent *IntentDecision, progress *terminal.ProgressDisplay) (*PlannerResult, error) {
	systemPrompt, err := composePlannerSystemPrompt(intent, intent.PlatformHint)
	if err != nil {
		return nil, err
	}

	// Marshal the analysis as the user message
	analysisJSON, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal analysis: %w", err)
	}

	userMsg := fmt.Sprintf("Create a file-level build plan for this app spec:\n\n%s", string(analysisJSON))

	progress.AddActivity("Sending analysis to Claude")

	gotFirstDelta := false
	resp, err := p.claude.GenerateStreaming(ctx, userMsg, claude.GenerateOpts{
		SystemPrompt: systemPrompt,
		MaxTurns:     3,
		Model:        "sonnet",
	}, func(ev claude.StreamEvent) {
		switch ev.Type {
		case "system":
			progress.AddActivity("Connected to Claude")
		case "content_block_delta":
			if ev.Text != "" {
				if !gotFirstDelta {
					gotFirstDelta = true
					progress.AddActivity("Designing file structure and models")
				}
				progress.OnStreamingText(ev.Text)
			}
		case "assistant":
			if ev.Text != "" {
				progress.OnAssistantText(ev.Text)
			}
		case "tool_use":
			if ev.ToolName != "" {
				progress.OnToolUse(ev.ToolName, func(key string) string {
					return extractToolInputString(ev.ToolInput, key)
				})
			}
		}
	})
	if err != nil {
		return nil, err
	}

	resultText := ""
	if resp != nil {
		resultText = resp.Result
	}

	return parsePlan(resultText)
}

// buildStreaming runs Phase 4 with real-time streaming output.
func (p *Pipeline) buildStreaming(ctx context.Context, prompt, appName, projectDir string, analysis *AnalysisResult, plan *PlannerResult, sessionID string, progress *terminal.ProgressDisplay, images []string, integrationIDs []string, backendProvisioned bool) (*claude.Response, error) {
	appendPrompt, userMsg, err := p.buildPrompts(prompt, appName, projectDir, analysis, plan, backendProvisioned)
	if err != nil {
		return nil, err
	}

	tools := buildAgenticTools(integrationIDs)
	terminal.Detail("Build prompt", fmt.Sprintf("system_append=%d chars, user_msg=%d chars, tools=%d, integrations=%s",
		len(appendPrompt), len(userMsg), len(tools), strings.Join(integrationIDs, ",")))

	// Log key prompt sections present
	hasBackendSetup := strings.Contains(appendPrompt, "<backend-setup>")
	hasIntegrationConfig := strings.Contains(appendPrompt, "<integration-config>")
	hasBackendFirst := strings.Contains(userMsg, "BACKEND FIRST")
	terminal.Detail("Prompt sections", fmt.Sprintf("backend-setup=%t, integration-config=%t, backend-first-in-user-msg=%t",
		hasBackendSetup, hasIntegrationConfig, hasBackendFirst))

	// Log if supabase MCP tools are in the allowed list
	hasSupabaseTools := false
	for _, t := range tools {
		if strings.HasPrefix(t, "mcp__supabase__") {
			hasSupabaseTools = true
			break
		}
	}
	terminal.Detail("Supabase MCP tools", fmt.Sprintf("allowed=%t", hasSupabaseTools))

	return p.claude.GenerateStreaming(ctx, userMsg, claude.GenerateOpts{
		AppendSystemPrompt: appendPrompt,
		MaxTurns:           30,
		Model:              p.buildModel(),
		WorkDir:            projectDir,
		AllowedTools:       tools,
		SessionID:          sessionID,
		Images:             images,
	}, newProgressCallback(progress))
}

// completeMissingFilesStreaming runs targeted completion passes for unresolved planned files.
func (p *Pipeline) completeMissingFilesStreaming(ctx context.Context, appName, projectDir string, plan *PlannerResult, report *FileCompletionReport, sessionID string, progress *terminal.ProgressDisplay) (*claude.Response, error) {
	appendPrompt, userMsg, err := p.completionPrompts(appName, projectDir, plan, report)
	if err != nil {
		return nil, err
	}

	return p.claude.GenerateStreaming(ctx, userMsg, claude.GenerateOpts{
		AppendSystemPrompt: appendPrompt,
		MaxTurns:           20,
		Model:              p.buildModel(),
		WorkDir:            projectDir,
		AllowedTools:       buildAgenticTools(nil),
		SessionID:          sessionID,
	}, newProgressCallback(progress))
}

// loadGlobalIntegrationStore creates and loads the global integration store from ~/.nanowave/.
func loadGlobalIntegrationStore() *integrations.IntegrationStore {
	home, err := os.UserHomeDir()
	if err != nil {
		return integrations.NewIntegrationStore("")
	}
	store := integrations.NewIntegrationStore(filepath.Join(home, ".nanowave"))
	_ = store.Load()
	return store
}

// loadIntegrationConfig retrieves a provider's config from the store for a specific app, returning nil if not found.
func loadIntegrationConfig(store *integrations.IntegrationStore, providerID string, appName string) *integrations.IntegrationConfig {
	cfg, err := store.GetProvider(integrations.ProviderID(providerID), appName)
	if err != nil || cfg == nil {
		return nil
	}
	return cfg
}

// inlineSetupChoice represents the user's choice when prompted to set up an integration during build.
type inlineSetupChoice int

const (
	setupChoiceSkip   inlineSetupChoice = iota // Continue without backend
	setupChoiceAuto                            // Automatic setup via CLI
	setupChoiceManual                          // Enter credentials manually
)

// askSetupConfirm prompts the user to set up an integration inline during build.
// Returns setupChoiceSkip if the user cancels (Esc) or picks Skip.
func askSetupConfirm(providerID string) inlineSetupChoice {
	integ := integrations.LookupIntegration(integrations.ProviderID(providerID))
	name := providerID
	if integ != nil {
		name = integ.Name
	}

	options := []terminal.PickerOption{
		{Label: "Automatic", Desc: "Connect via " + name + " CLI (opens browser, ~30 seconds)"},
		{Label: "Manual", Desc: "Enter project URL and anon key manually"},
		{Label: "Skip", Desc: "Continue without backend — use placeholder credentials"},
	}

	picked := terminal.Pick(fmt.Sprintf("Set up %s now?", name), options, "")
	switch picked {
	case "Automatic":
		return setupChoiceAuto
	case "Manual":
		return setupChoiceManual
	default:
		return setupChoiceSkip
	}
}

// promptIntegrationSetup shows the setup picker and retries on failure until the user
// picks Skip (or Esc) or setup succeeds. Returns nil if skipped.
func promptIntegrationSetup(store *integrations.IntegrationStore, providerID string, appName string) *integrations.IntegrationConfig {
	for {
		choice := askSetupConfirm(providerID)
		switch choice {
		case setupChoiceAuto:
			if err := runInlineIntegrationSetup(store, providerID, appName, false); err != nil {
				terminal.Error(fmt.Sprintf("Setup failed: %v", err))
				fmt.Println()
				continue // re-prompt
			}
			return loadIntegrationConfig(store, providerID, appName)
		case setupChoiceManual:
			if err := runInlineIntegrationSetup(store, providerID, appName, true); err != nil {
				terminal.Error(fmt.Sprintf("Setup failed: %v", err))
				fmt.Println()
				continue // re-prompt
			}
			return loadIntegrationConfig(store, providerID, appName)
		default:
			// Skip or Esc
			return nil
		}
	}
}

// runInlineIntegrationSetup runs the setup flow for a provider during the build pipeline.
// For automatic setup, installs the CLI if missing before proceeding.
func runInlineIntegrationSetup(store *integrations.IntegrationStore, providerID string, appName string, manual bool) error {
	switch providerID {
	case "supabase":
		if manual {
			return integrations.SetupSupabaseManual(store, appName, pipelineReadLineFn, pipelinePrintFn)
		}
		if !config.CheckSupabaseCLI() {
			if err := installSupabaseCLI(); err != nil {
				return err
			}
		}
		return integrations.SetupSupabase(store, appName, pipelinePrintFn, pipelinePickFn)
	default:
		return fmt.Errorf("unknown provider: %s", providerID)
	}
}

// installSupabaseCLI installs the Supabase CLI via Homebrew.
func installSupabaseCLI() error {
	terminal.Info("Installing Supabase CLI...")
	brewPath, err := exec.LookPath("brew")
	if err != nil {
		terminal.Error("Homebrew not found — install it from https://brew.sh")
		return fmt.Errorf("homebrew not found")
	}
	installCmd := exec.Command(brewPath, "install", "supabase/tap/supabase")
	installCmd.Env = append(os.Environ(),
		"HOMEBREW_NO_AUTO_UPDATE=1",
		"HOMEBREW_NO_INSTALL_CLEANUP=1",
		"HOMEBREW_NO_ENV_HINTS=1",
	)
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install Supabase CLI: %w", err)
	}
	if !config.CheckSupabaseCLI() {
		return fmt.Errorf("supabase CLI not found after install")
	}
	terminal.Success("Supabase CLI installed")
	return nil
}

// pipelinePrintFn bridges integrations output to terminal for inline setup.
func pipelinePrintFn(level, msg string) {
	switch level {
	case "success":
		terminal.Success(msg)
	case "warning":
		terminal.Warning(msg)
	case "error":
		terminal.Error(msg)
	case "info":
		terminal.Info(msg)
	case "header":
		terminal.Header(msg)
	case "detail":
		fmt.Printf("    %s%s%s\n", terminal.Dim, msg, terminal.Reset)
	}
}

// pipelinePickFn bridges integrations pick to terminal.Pick for inline setup.
func pipelinePickFn(title string, options []string) string {
	pickerOpts := make([]terminal.PickerOption, len(options))
	for i, opt := range options {
		pickerOpts[i] = terminal.PickerOption{Label: opt}
	}
	return terminal.Pick(title, pickerOpts, "")
}


// pipelineReadLineFn reads a line of input with a label prompt.
func pipelineReadLineFn(label string) string {
	fmt.Printf("  %s: ", label)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

// finalize ensures xcodegen has run, then does git init + commit.
func (p *Pipeline) finalize(ctx context.Context, projectDir, appName string) {
	// Safety net: re-run xcodegen if .xcodeproj is missing (shouldn't happen since we run it in scaffold phase)
	xcodeprojPath := filepath.Join(projectDir, appName+".xcodeproj")
	if _, err := os.Stat(xcodeprojPath); os.IsNotExist(err) {
		_ = runXcodeGen(projectDir)
	}

	// Git operations are best-effort
	resp, _ := p.claude.Generate(ctx, fmt.Sprintf(`Run these git commands in order:
1. git init
2. git add -A
3. git commit -m "Initial build: %s"

Just run the commands, no explanation needed.`, appName), claude.GenerateOpts{
		MaxTurns:     3,
		Model:        "haiku",
		WorkDir:      projectDir,
		AllowedTools: []string{"Bash"},
	})
	_ = resp
}

// configureSupabaseAuth calls the Supabase Management API to enable auth providers
// before the build phase starts. This runs from the pipeline (not the MCP server).
// configureSupabaseAuth enables auth providers via the Supabase Management API.
// Field names match the PATCH /v1/projects/{ref}/config/auth schema (lowercase).
func configureSupabaseAuth(pat, projectRef, bundleID string, authMethods []string) error {
	c := &supabaseAPIClient{
		pat:        pat,
		projectRef: projectRef,
	}
	config := make(map[string]any)
	// Auto-confirm emails so signup works instantly without email verification.
	// Apps built by nanowave are prototypes — email verification adds friction.
	config["mailer_autoconfirm"] = true
	for _, method := range authMethods {
		switch method {
		case "email":
			config["external_email_enabled"] = true
		case "anonymous":
			config["external_anonymous_users_enabled"] = true
		case "apple":
			config["external_apple_enabled"] = true
			if bundleID != "" {
				config["external_apple_client_id"] = bundleID
			}
		case "google":
			config["external_google_enabled"] = true
		case "phone":
			config["external_phone_enabled"] = true
		}
	}
	if len(config) == 0 {
		return nil
	}
	return c.updateAuthConfig(config)
}

// supabaseAPIClient is a minimal HTTP client for the Supabase Management API,
// used by the pipeline to provision backend resources before building.
type supabaseAPIClient struct {
	pat        string
	projectRef string
}

func (c *supabaseAPIClient) updateAuthConfig(config map[string]any) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://api.supabase.com/v1/projects/%s/config/auth", c.projectRef)
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.pat)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("auth config update returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *supabaseAPIClient) executeSQL(query string) error {
	data, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://api.supabase.com/v1/projects/%s/database/query", c.projectRef)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.pat)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SQL execution returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// getAPIKeys retrieves all API keys for the project from the Management API.
func (c *supabaseAPIClient) getAPIKeys() ([]supabaseKey, error) {
	url := fmt.Sprintf("https://api.supabase.com/v1/projects/%s/api-keys?reveal=true", c.projectRef)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.pat)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api-keys returned %d: %s", resp.StatusCode, string(body))
	}
	var keys []supabaseKey
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return nil, err
	}
	return keys, nil
}

// supabaseKey represents an API key from the Management API.
type supabaseKey struct {
	Name   string `json:"name"`
	APIKey string `json:"api_key"`
	Type   string `json:"type"`
}

// findAnonKey returns the anon key from the project's API keys.
func (c *supabaseAPIClient) findAnonKey() (string, error) {
	keys, err := c.getAPIKeys()
	if err != nil {
		return "", err
	}
	for _, k := range keys {
		if strings.EqualFold(k.Name, "anon") || k.Type == "publishable" {
			return k.APIKey, nil
		}
	}
	return "", fmt.Errorf("anon key not found in %d keys", len(keys))
}

// setSecrets creates or updates edge function secrets (environment variables).
func (c *supabaseAPIClient) setSecrets(secrets []map[string]string) error {
	data, err := json.Marshal(secrets)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://api.supabase.com/v1/projects/%s/secrets", c.projectRef)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.pat)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("set secrets returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// generateCreateTablesSQL generates CREATE TABLE IF NOT EXISTS statements for all planned models.
func generateCreateTablesSQL(models []ModelPlan) string {
	var b strings.Builder
	for _, m := range models {
		tableName := modelToTableName(m.Name)
		fmt.Fprintf(&b, "CREATE TABLE IF NOT EXISTS public.%s (\n", tableName)
		for i, prop := range m.Properties {
			colName := camelToSnake(prop.Name)
			pgType := swiftTypeToPG(prop.Type)
			constraints := inferConstraints(prop, i == 0, m.Name)
			fmt.Fprintf(&b, "  %s %s%s", colName, pgType, constraints)
			if i < len(m.Properties)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(");\n\n")
	}
	return b.String()
}

// generateEnableRLSSQL generates ALTER TABLE ... ENABLE ROW LEVEL SECURITY for all models.
func generateEnableRLSSQL(models []ModelPlan) string {
	var b strings.Builder
	for _, m := range models {
		fmt.Fprintf(&b, "ALTER TABLE public.%s ENABLE ROW LEVEL SECURITY;\n", modelToTableName(m.Name))
	}
	return b.String()
}

// generateRLSPoliciesSQL generates basic RLS policies for all models.
// Tables with a user_id column get owner-based policies; others get public read + auth write.
// Uses DROP + CREATE (PostgreSQL does not support CREATE POLICY IF NOT EXISTS).
func generateRLSPoliciesSQL(models []ModelPlan) string {
	var b strings.Builder
	for _, m := range models {
		tableName := modelToTableName(m.Name)
		hasUserID := false
		for _, prop := range m.Properties {
			if camelToSnake(prop.Name) == "user_id" {
				hasUserID = true
				break
			}
		}
		policies := []struct{ suffix, op, clause string }{
			{"select", "SELECT", "USING (true)"},
		}
		if hasUserID {
			// Owner-based: users can read all rows, write only their own
			policies = append(policies,
				struct{ suffix, op, clause string }{"insert", "INSERT", "WITH CHECK (auth.uid() = user_id)"},
				struct{ suffix, op, clause string }{"update", "UPDATE", "USING (auth.uid() = user_id)"},
				struct{ suffix, op, clause string }{"delete", "DELETE", "USING (auth.uid() = user_id)"},
			)
		} else {
			// No user_id: public read, authenticated write
			policies = append(policies,
				struct{ suffix, op, clause string }{"insert", "INSERT", "WITH CHECK (auth.role() = 'authenticated')"},
				struct{ suffix, op, clause string }{"update", "UPDATE", "USING (auth.role() = 'authenticated')"},
				struct{ suffix, op, clause string }{"delete", "DELETE", "USING (auth.role() = 'authenticated')"},
			)
		}
		for _, p := range policies {
			policyName := fmt.Sprintf("%s_%s", tableName, p.suffix)
			fmt.Fprintf(&b, "DROP POLICY IF EXISTS \"%s\" ON public.%s;\n", policyName, tableName)
			fmt.Fprintf(&b, "CREATE POLICY \"%s\" ON public.%s FOR %s %s;\n", policyName, tableName, p.op, p.clause)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// modelTableNames returns a comma-separated list of table names for logging.
func modelTableNames(models []ModelPlan) string {
	names := make([]string, len(models))
	for i, m := range models {
		names[i] = modelToTableName(m.Name)
	}
	return strings.Join(names, ", ")
}

// generateRealtimeSQL generates SQL to enable Supabase Realtime on all model tables.
// Adds tables to the supabase_realtime publication and sets REPLICA IDENTITY FULL
// so UPDATE/DELETE events include the full old row data.
func generateRealtimeSQL(models []ModelPlan) string {
	var b strings.Builder
	for _, m := range models {
		tableName := modelToTableName(m.Name)
		fmt.Fprintf(&b, "ALTER PUBLICATION supabase_realtime ADD TABLE public.%s;\n", tableName)
		fmt.Fprintf(&b, "ALTER TABLE public.%s REPLICA IDENTITY FULL;\n", tableName)
	}
	return b.String()
}

// generateStoragePoliciesSQL generates basic storage.objects RLS policies for a bucket.
// Creates per-user folder access: public read, authenticated upload/update/delete in own folder.
func generateStoragePoliciesSQL(bucketID string) string {
	var b strings.Builder

	policies := []struct {
		suffix, op, clause string
	}{
		{
			"select", "SELECT",
			fmt.Sprintf("USING (bucket_id = '%s')", bucketID),
		},
		{
			"insert", "INSERT",
			fmt.Sprintf("WITH CHECK (bucket_id = '%s' AND auth.role() = 'authenticated' AND (storage.foldername(name))[1] = auth.uid()::text)", bucketID),
		},
		{
			"update", "UPDATE",
			fmt.Sprintf("USING (bucket_id = '%s' AND auth.uid()::text = (storage.foldername(name))[1])", bucketID),
		},
		{
			"delete", "DELETE",
			fmt.Sprintf("USING (bucket_id = '%s' AND auth.uid()::text = (storage.foldername(name))[1])", bucketID),
		},
	}

	for _, p := range policies {
		policyName := fmt.Sprintf("%s_%s", bucketID, p.suffix)
		fmt.Fprintf(&b, "DROP POLICY IF EXISTS \"%s\" ON storage.objects;\n", policyName)
		fmt.Fprintf(&b, "CREATE POLICY \"%s\" ON storage.objects FOR %s %s;\n", policyName, p.op, p.clause)
	}
	return b.String()
}
