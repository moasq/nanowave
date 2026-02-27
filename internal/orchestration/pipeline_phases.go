package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moasq/nanowave/internal/claude"
	"github.com/moasq/nanowave/internal/terminal"
)

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
func (p *Pipeline) buildStreaming(ctx context.Context, prompt, appName, projectDir string, analysis *AnalysisResult, plan *PlannerResult, sessionID string, progress *terminal.ProgressDisplay, images []string, backendProvisioned bool) (*claude.Response, error) {
	appendPrompt, userMsg, err := p.buildPrompts(prompt, appName, projectDir, analysis, plan, backendProvisioned)
	if err != nil {
		return nil, err
	}

	tools := make([]string, len(baseAgenticTools))
	copy(tools, baseAgenticTools)
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

	tools := make([]string, len(baseAgenticTools))
	copy(tools, baseAgenticTools)

	return p.claude.GenerateStreaming(ctx, userMsg, claude.GenerateOpts{
		AppendSystemPrompt: appendPrompt,
		MaxTurns:           20,
		Model:              p.buildModel(),
		WorkDir:            projectDir,
		AllowedTools:       tools,
		SessionID:          sessionID,
	}, newProgressCallback(progress))
}
