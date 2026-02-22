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
	"github.com/moasq/nanowave/internal/terminal"
)

const maxBuildCompletionPasses = 6

// agenticTools is the set of tools Claude Code needs for writing code, building, and fixing.
var agenticTools = []string{
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
	"mcp__xcodegen__regenerate_project",
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

// Build runs the full 5-phase pipeline: setup → analyze → plan → build+fix → finalize.
// images is an optional list of image file paths to include in the build prompt.
func (p *Pipeline) Build(ctx context.Context, prompt string, images []string) (*BuildResult, error) {
	// Phase 1: Setup workspace
	spinner := terminal.NewSpinner("Setting up workspace...")
	spinner.Start()

	appName := "MyApp" // placeholder until analyzer names it
	projectDir := filepath.Join(p.config.ProjectDir, appName)

	// We'll rename after analysis — for now create a temp structure
	// Phase 2 will give us the real name

	spinner.StopWithMessage(fmt.Sprintf("%s%s✓%s Workspace ready", terminal.Bold, terminal.Green, terminal.Reset))

	// Phase 2: Analyze
	analyzeProgress := terminal.NewProgressDisplay("analyze", 0)
	analyzeProgress.Start()

	analysis, err := p.analyze(ctx, prompt, analyzeProgress)
	if err != nil {
		analyzeProgress.StopWithError("Analysis failed")
		return nil, fmt.Errorf("analysis failed: %w", err)
	}

	appName = sanitizeToPascalCase(analysis.AppName)
	projectDir = filepath.Join(p.config.ProjectDir, appName)

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

	// Phase 3: Plan
	planProgress := terminal.NewProgressDisplay("plan", 0)
	planProgress.Start()

	plan, err := p.plan(ctx, analysis, planProgress)
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

	if err := writeInitialCLAUDEMD(projectDir, appName, plan.GetDeviceFamily()); err != nil {
		return nil, fmt.Errorf("failed to write CLAUDE.md: %w", err)
	}

	if err := enrichCLAUDEMD(projectDir, plan, appName); err != nil {
		return nil, fmt.Errorf("failed to enrich CLAUDE.md: %w", err)
	}

	if err := writeCoreRules(projectDir); err != nil {
		return nil, fmt.Errorf("failed to write core rules: %w", err)
	}

	if err := writeAlwaysSkills(projectDir); err != nil {
		return nil, fmt.Errorf("failed to write always skills: %w", err)
	}

	// Auto-inject adaptive_layout skill for iPad/universal apps
	if family := plan.GetDeviceFamily(); family == "ipad" || family == "universal" {
		hasAdaptive := false
		for _, k := range plan.RuleKeys {
			if k == "adaptive_layout" {
				hasAdaptive = true
				break
			}
		}
		if !hasAdaptive {
			plan.RuleKeys = append(plan.RuleKeys, "adaptive_layout")
		}
	}

	if err := writeConditionalSkills(projectDir, plan.RuleKeys); err != nil {
		return nil, fmt.Errorf("failed to write conditional skills: %w", err)
	}

	if err := writeMCPConfig(projectDir); err != nil {
		return nil, fmt.Errorf("failed to write MCP config: %w", err)
	}

	if err := writeSettingsLocal(projectDir); err != nil {
		return nil, fmt.Errorf("failed to write settings: %w", err)
	}

	if err := writeProjectYML(projectDir, plan, appName); err != nil {
		return nil, fmt.Errorf("failed to write project.yml: %w", err)
	}

	if err := writeProjectConfig(projectDir, plan, appName); err != nil {
		return nil, fmt.Errorf("failed to write project_config.json: %w", err)
	}

	if err := writeGitignore(projectDir); err != nil {
		return nil, fmt.Errorf("failed to write .gitignore: %w", err)
	}

	if err := writeAssetCatalog(projectDir, appName); err != nil {
		return nil, fmt.Errorf("failed to write asset catalog: %w", err)
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

	for pass := 1; pass <= maxBuildCompletionPasses; pass++ {
		passLabel := fmt.Sprintf("Generation pass %d", pass)

		var resp *claude.Response
		if pass == 1 {
			resp, err = p.buildStreaming(ctx, prompt, appName, projectDir, analysis, plan, sessionID, progress, images)
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

	return &BuildResult{
		AppName:          analysis.AppName,
		Description:      analysis.Description,
		ProjectDir:       projectDir,
		BundleID:         fmt.Sprintf("%s.%s", bundleIDPrefix(), strings.ToLower(appName)),
		DeviceFamily:     plan.GetDeviceFamily(),
		Features:         analysis.Features,
		FileCount:        len(plan.Files),
		PlannedFiles:     len(plan.Files),
		CompletedFiles:   report.ValidCount,
		CompletionPasses: completionPasses,
		SessionID:        sessionID,
		TotalCostUSD:     totalCostUSD,
		InputTokens:      totalInputTokens,
		OutputTokens:     totalOutputTokens,
		CacheRead:        totalCacheRead,
		CacheCreated:     totalCacheCreate,
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
	appName := filepath.Base(projectDir)
	ensureMCPConfig(projectDir)

	appendPrompt := coderPrompt + "\n\n" + sharedConstraints
	appendPrompt += fmt.Sprintf("\n\nBuild command:\nxcodebuild -project %s.xcodeproj -scheme %s -destination 'generic/platform=iOS Simulator' -quiet build", appName, appName)

	userMsg := fmt.Sprintf(`Edit this app based on the following request:

%s

After making changes:
1. If you need new permissions, extensions, or entitlements, use the xcodegen MCP tools (add_permission, add_extension, etc.)
2. Run: xcodebuild -project %s.xcodeproj -scheme %s -destination 'generic/platform=iOS Simulator' -quiet build
3. If build fails, read the errors carefully, fix the code, and rebuild
4. Repeat until the build succeeds`, prompt, appName, appName)

	progress := terminal.NewProgressDisplay("edit", 0)
	progress.Start()

	resp, err := p.claude.GenerateStreaming(ctx, userMsg, claude.GenerateOpts{
		AppendSystemPrompt: appendPrompt,
		MaxTurns:           30,
		Model:              p.buildModel(),
		WorkDir:            projectDir,
		AllowedTools:       agenticTools,
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
	appName := filepath.Base(projectDir)
	ensureMCPConfig(projectDir)

	appendPrompt := coderPrompt + "\n\n" + sharedConstraints

	userMsg := fmt.Sprintf(`Fix any build errors in this project.

1. Run: xcodebuild -project %s.xcodeproj -scheme %s -destination 'generic/platform=iOS Simulator' -quiet build
2. Read the error output carefully
3. Investigate: read the relevant source files to understand context
4. Fix the errors in the Swift source code
5. If the error is a project configuration issue, use the xcodegen MCP tools (add_permission, add_extension, regenerate_project, etc.)
6. Rebuild and repeat until the build succeeds`, appName, appName)

	progress := terminal.NewProgressDisplay("fix", 0)
	progress.Start()

	resp, err := p.claude.GenerateStreaming(ctx, userMsg, claude.GenerateOpts{
		AppendSystemPrompt: appendPrompt,
		MaxTurns:           30,
		Model:              p.buildModel(),
		WorkDir:            projectDir,
		AllowedTools:       agenticTools,
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
func (p *Pipeline) analyze(ctx context.Context, prompt string, progress *terminal.ProgressDisplay) (*AnalysisResult, error) {
	systemPrompt := analyzerPrompt + "\n\n" + planningConstraints

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
func (p *Pipeline) plan(ctx context.Context, analysis *AnalysisResult, progress *terminal.ProgressDisplay) (*PlannerResult, error) {
	systemPrompt := plannerPrompt + "\n\n" + planningConstraints

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
func (p *Pipeline) buildStreaming(ctx context.Context, prompt, appName, projectDir string, analysis *AnalysisResult, plan *PlannerResult, sessionID string, progress *terminal.ProgressDisplay, images []string) (*claude.Response, error) {
	appendPrompt, userMsg := p.buildPrompts(prompt, appName, projectDir, analysis, plan)

	return p.claude.GenerateStreaming(ctx, userMsg, claude.GenerateOpts{
		AppendSystemPrompt: appendPrompt,
		MaxTurns:           30,
		Model:              p.buildModel(),
		WorkDir:            projectDir,
		AllowedTools:       agenticTools,
		SessionID:          sessionID,
		Images:             images,
	}, newProgressCallback(progress))
}

// completeMissingFilesStreaming runs targeted completion passes for unresolved planned files.
func (p *Pipeline) completeMissingFilesStreaming(ctx context.Context, appName, projectDir string, plan *PlannerResult, report *FileCompletionReport, sessionID string, progress *terminal.ProgressDisplay) (*claude.Response, error) {
	appendPrompt, userMsg := p.completionPrompts(appName, projectDir, plan, report)

	return p.claude.GenerateStreaming(ctx, userMsg, claude.GenerateOpts{
		AppendSystemPrompt: appendPrompt,
		MaxTurns:           20,
		Model:              p.buildModel(),
		WorkDir:            projectDir,
		AllowedTools:       agenticTools,
		SessionID:          sessionID,
	}, newProgressCallback(progress))
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
