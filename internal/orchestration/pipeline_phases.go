package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moasq/nanowave/internal/agentruntime"
	"github.com/moasq/nanowave/internal/terminal"
)

// analyze runs Phase 2: prompt → AnalysisResult.
func (p *Pipeline) analyze(ctx context.Context, prompt string, intent *IntentDecision, ac ActionContext, progress *terminal.ProgressDisplay) (*AnalysisResult, error) {
	systemPrompt, err := composeAnalyzerSystemPrompt(intent)
	if err != nil {
		return nil, err
	}

	// For edits, prepend existing project context so the analyzer focuses on NEW capabilities
	userMsg := prompt
	if ac.IsEdit() {
		userMsg = fmt.Sprintf("Existing project: %s (platform: %s)\nEdit request: %s", ac.AppName, ac.Platform, prompt)
	}

	progress.AddActivity("Starting analysis")

	gotFirstDelta := false
	var progressCb func(agentruntime.StreamEvent)
	if progress != nil {
		progressCb = newProgressCallback(progress, p.progressRuntimeLabel())
	}
	resp, err := p.runtime.GenerateStreaming(ctx, userMsg, agentruntime.GenerateOpts{
		SystemPrompt: systemPrompt,
		MaxTurns:     3,
		Model:        p.modelForPhase(agentruntime.PhaseAnalyze),
	}, func(ev agentruntime.StreamEvent) {
		if progressCb != nil {
			progressCb(ev)
		}
		switch ev.Type {
		case "content_block_delta":
			if ev.Text != "" {
				if !gotFirstDelta {
					gotFirstDelta = true
					progress.AddActivity("Identifying features and requirements")
				}
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
func (p *Pipeline) plan(ctx context.Context, analysis *AnalysisResult, intent *IntentDecision, ac ActionContext, progress *terminal.ProgressDisplay) (*PlannerResult, error) {
	systemPrompt, err := composePlannerSystemPrompt(intent, intent.PlatformHint)
	if err != nil {
		return nil, err
	}

	// Marshal the analysis as the user message
	analysisJSON, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal analysis: %w", err)
	}

	var userMsg string
	if ac.IsEdit() {
		userMsg = fmt.Sprintf("Plan ONLY the new/modified files for this edit to an existing project:\n\n%s", string(analysisJSON))
	} else {
		userMsg = fmt.Sprintf("Create a file-level build plan for this app spec:\n\n%s", string(analysisJSON))
	}

	progress.AddActivity("Starting plan")

	gotFirstDelta := false
	var progressCb func(agentruntime.StreamEvent)
	if progress != nil {
		progressCb = newProgressCallback(progress, p.progressRuntimeLabel())
	}
	resp, err := p.runtime.GenerateStreaming(ctx, userMsg, agentruntime.GenerateOpts{
		SystemPrompt: systemPrompt,
		MaxTurns:     3,
		Model:        p.modelForPhase(agentruntime.PhasePlan),
	}, func(ev agentruntime.StreamEvent) {
		if progressCb != nil {
			progressCb(ev)
		}
		switch ev.Type {
		case "content_block_delta":
			if ev.Text != "" {
				if !gotFirstDelta {
					gotFirstDelta = true
					progress.AddActivity("Drafting file structure and models")
				}
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
func (p *Pipeline) buildStreaming(ctx context.Context, prompt, appName, projectDir string, analysis *AnalysisResult, plan *PlannerResult, sessionID string, progress *terminal.ProgressDisplay, images []string, backendProvisioned bool, ac ActionContext) (*agentruntime.Response, error) {
	appendPrompt, userMsg, err := p.buildPrompts(prompt, appName, projectDir, analysis, plan, backendProvisioned, ac)
	if err != nil {
		return nil, err
	}

	tools := p.baseAgenticTools()
	if p.manager != nil {
		tools = append(tools, p.manager.AgentTools(p.activeProviders)...)
	}
	terminal.Detail("Build prompt", fmt.Sprintf("system_append=%d chars, user_msg=%d chars, tools=%d",
		len(appendPrompt), len(userMsg), len(tools)))

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

	return p.runtime.GenerateStreaming(ctx, userMsg, agentruntime.GenerateOpts{
		AppendSystemPrompt: appendPrompt,
		MaxTurns:           30,
		Model:              p.modelForPhase(agentruntime.PhaseBuild),
		WorkDir:            projectDir,
		AllowedTools:       tools,
		SessionID:          sessionID,
		Images:             images,
	}, p.makeStreamCallback(progress))
}

// completeMissingFilesStreaming runs targeted completion passes for unresolved planned files.
func (p *Pipeline) completeMissingFilesStreaming(ctx context.Context, appName, projectDir string, plan *PlannerResult, report *FileCompletionReport, sessionID string, progress *terminal.ProgressDisplay) (*agentruntime.Response, error) {
	appendPrompt, userMsg, err := p.completionPrompts(appName, projectDir, plan, report)
	if err != nil {
		return nil, err
	}

	tools := p.baseAgenticTools()

	return p.runtime.GenerateStreaming(ctx, userMsg, agentruntime.GenerateOpts{
		AppendSystemPrompt: appendPrompt,
		MaxTurns:           20,
		Model:              p.modelForPhase(agentruntime.PhaseBuild),
		WorkDir:            projectDir,
		AllowedTools:       tools,
		SessionID:          sessionID,
	}, p.makeStreamCallback(progress))
}
