package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moasq/nanowave/internal/agentruntime"
	"github.com/moasq/nanowave/internal/terminal"
)

// analyze runs Phase 2: prompt → AnalysisResult.
// No tools allowed — pure JSON generation.
func (p *Pipeline) analyze(ctx context.Context, prompt string, intent *IntentDecision, ac ActionContext, progress *terminal.ProgressDisplay) (*AnalysisResult, error) {
	systemPrompt, err := composeAnalyzerSystemPrompt(intent)
	if err != nil {
		return nil, err
	}

	userMsg := prompt
	if ac.IsEdit() {
		userMsg = fmt.Sprintf("Existing project: %s (platform: %s)\nEdit request: %s", ac.AppName, ac.Platform, prompt)
	}

	progress.AddActivity("Analyzing requirements...")

	resp, err := p.runtime.GenerateStreaming(ctx, userMsg, agentruntime.GenerateOpts{
		SystemPrompt: systemPrompt,
		MaxTurns:     3,
		Model:        p.modelForPhase(agentruntime.PhaseAnalyze),
		AllowedTools: []string{},
	}, func(ev agentruntime.StreamEvent) {
		if ev.Type == "content_block_delta" && ev.Text != "" {
			progress.OnStreamingText(ev.Text)
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
// Pure JSON generation — no tools, single turn.
// For edits, the existing project file list is injected into the prompt
// so the planner doesn't need to browse the filesystem.
func (p *Pipeline) plan(ctx context.Context, analysis *AnalysisResult, intent *IntentDecision, ac ActionContext, progress *terminal.ProgressDisplay) (*PlannerResult, error) {
	systemPrompt, err := composePlannerSystemPrompt(intent, intent.PlatformHint)
	if err != nil {
		return nil, err
	}

	analysisJSON, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal analysis: %w", err)
	}

	var userMsg string
	if ac.IsEdit() {
		// For edits: inject existing file list so the planner knows the codebase
		fileList := listProjectSwiftFiles(ac.ProjectDir, ac.AppName)
		userMsg = fmt.Sprintf(`Plan ONLY the new/modified files for this edit to an existing project.

Existing project: %s (platform: %s)
Existing Swift files:
%s

Analysis of changes needed:
%s

Return PlannerResult JSON with ONLY files that need to be created or modified. Do NOT include unchanged files.`,
			ac.AppName, ac.Platform, fileList, string(analysisJSON))
	} else {
		userMsg = fmt.Sprintf("Create a file-level build plan for this app spec:\n\n%s", string(analysisJSON))
	}

	progress.AddActivity("Planning file structure...")

	resp, err := p.runtime.GenerateStreaming(ctx, userMsg, agentruntime.GenerateOpts{
		SystemPrompt: systemPrompt,
		MaxTurns:     3,
		Model:        p.modelForPhase(agentruntime.PhasePlan),
		AllowedTools: []string{},
	}, func(ev agentruntime.StreamEvent) {
		if ev.Type == "content_block_delta" && ev.Text != "" {
			progress.OnStreamingText(ev.Text)
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
func (p *Pipeline) buildStreaming(ctx context.Context, prompt, appName, projectDir string, analysis *AnalysisResult, plan *PlannerResult, sessionID string, progress *terminal.ProgressDisplay, images []string, backendProvisioned bool, ac ActionContext) (*agentruntime.Response, error) {
	appendPrompt, userMsg, err := p.buildPrompts(prompt, appName, projectDir, analysis, plan, backendProvisioned, ac)
	if err != nil {
		return nil, err
	}

	tools := p.baseAgenticTools()
	if p.manager != nil {
		tools = append(tools, p.manager.AgentTools(p.activeProviders)...)
	}

	opts := agentruntime.GenerateOpts{
		SystemPrompt: coderPromptForPlatform(plan.GetPlatform()),
		MaxTurns:     30,
		Model:        p.modelForPhase(agentruntime.PhaseBuild),
		WorkDir:      projectDir,
		AllowedTools: tools,
		SessionID:    sessionID,
		Images:       images,
	}
	opts = p.mergedOpts(opts, appendPrompt)

	return p.runtime.GenerateStreaming(ctx, userMsg, opts, p.makeStreamCallback(progress))
}

// completeMissingFilesStreaming runs targeted completion passes for unresolved planned files.
func (p *Pipeline) completeMissingFilesStreaming(ctx context.Context, appName, projectDir string, plan *PlannerResult, report *FileCompletionReport, sessionID string, progress *terminal.ProgressDisplay) (*agentruntime.Response, error) {
	appendPrompt, userMsg, err := p.completionPrompts(appName, projectDir, plan, report)
	if err != nil {
		return nil, err
	}

	tools := p.baseAgenticTools()

	opts := agentruntime.GenerateOpts{
		SystemPrompt: coderPromptForPlatform(plan.GetPlatform()),
		MaxTurns:     20,
		Model:        p.modelForPhase(agentruntime.PhaseBuild),
		WorkDir:      projectDir,
		AllowedTools: tools,
		SessionID:    sessionID,
	}
	opts = p.mergedOpts(opts, appendPrompt)

	return p.runtime.GenerateStreaming(ctx, userMsg, opts, p.makeStreamCallback(progress))
}

// listProjectSwiftFiles returns a formatted list of Swift files in a project directory.
func listProjectSwiftFiles(projectDir, appName string) string {
	var files []string
	root := filepath.Join(projectDir, appName)
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".swift") {
			rel, _ := filepath.Rel(projectDir, path)
			files = append(files, rel)
		}
		return nil
	})
	// Also check Shared/ and Targets/
	for _, extra := range []string{"Shared", "Targets"} {
		extraDir := filepath.Join(projectDir, extra)
		_ = filepath.WalkDir(extraDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if strings.HasSuffix(d.Name(), ".swift") {
				rel, _ := filepath.Rel(projectDir, path)
				files = append(files, rel)
			}
			return nil
		})
	}
	if len(files) == 0 {
		return "(no existing Swift files found)"
	}
	var b strings.Builder
	for _, f := range files {
		b.WriteString("- ")
		b.WriteString(f)
		b.WriteString("\n")
	}
	return b.String()
}
