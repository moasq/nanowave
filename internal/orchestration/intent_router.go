package orchestration

import (
	"context"
	"fmt"
	"strings"

	"github.com/moasq/nanowave/internal/claude"
	"github.com/moasq/nanowave/internal/terminal"
)

const intentRouterBasePrompt = `You are an intent router for the app generation pipeline.
Return ONLY valid JSON.
Follow the attached phase skill instructions.
Hints are advisory only and must reflect explicit user wording.`

func defaultBuildIntentDecision() *IntentDecision {
	return &IntentDecision{
		Operation:        "build",
		PlatformHint:     PlatformIOS,
		DeviceFamilyHint: "iphone",
		Confidence:       0.25,
		Reason:           "Default build path (iOS/iPhone) until stronger intent signals are found",
	}
}

func finalizeBuildIntentDecision(parsed, fallback *IntentDecision) *IntentDecision {
	if fallback == nil {
		fallback = defaultBuildIntentDecision()
	}
	if parsed == nil {
		out := *fallback
		return &out
	}

	out := *parsed
	if out.Operation == "" || out.Operation == "unknown" {
		out.Operation = "build"
	}
	if out.PlatformHint == "" {
		out.PlatformHint = fallback.PlatformHint
	}

	// When PlatformHints has multiple valid entries, preserve them and set PlatformHint to first
	if len(out.PlatformHints) > 0 {
		out.PlatformHint = out.PlatformHints[0]
	}

	if out.Confidence <= 0 {
		out.Confidence = fallback.Confidence
	}
	if out.Reason == "" {
		out.Reason = fallback.Reason
	}
	if out.PlatformHint != PlatformWatchOS && out.PlatformHint != PlatformTvOS && out.DeviceFamilyHint == "" {
		out.DeviceFamilyHint = fallback.DeviceFamilyHint
	}

	normalizeWatchShapeIntentHints(&out)
	return &out
}

func composeIntentRouterSystemPrompt() (string, error) {
	phaseSkill, err := loadPhaseSkillContent("intent-router")
	if err != nil {
		return "", err
	}
	var b strings.Builder
	appendPromptSection(&b, "Intent Router Base", intentRouterBasePrompt)
	appendPromptSection(&b, "Phase Skill", phaseSkill)
	return b.String(), nil
}

func (p *Pipeline) decideBuildIntent(ctx context.Context, prompt string, progress *terminal.ProgressDisplay) (*IntentDecision, error) {
	fallback := defaultBuildIntentDecision()

	if progress != nil {
		progress.AddActivity("Using AI intent router")
	}

	systemPrompt, err := composeIntentRouterSystemPrompt()
	if err != nil {
		return nil, err
	}

	gotFirstDelta := false
	resp, err := p.claude.GenerateStreaming(ctx, prompt, claude.GenerateOpts{
		SystemPrompt: systemPrompt,
		MaxTurns:     2,
		Model:        "haiku",
	}, func(ev claude.StreamEvent) {
		if progress == nil {
			return
		}
		switch ev.Type {
		case "system":
			progress.AddActivity("Connected to Claude")
		case "content_block_delta":
			if ev.Text != "" {
				if !gotFirstDelta {
					gotFirstDelta = true
					progress.AddActivity("Generating routing hints")
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
		return nil, fmt.Errorf("intent router returned empty response")
	}

	parsed, err := parseIntentDecision(resultText)
	if err != nil {
		return nil, err
	}
	parsed.UsedLLM = true
	return finalizeBuildIntentDecision(parsed, fallback), nil
}
