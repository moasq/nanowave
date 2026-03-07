package orchestration

import (
	"context"
	"fmt"
	"strings"

	"github.com/moasq/nanowave/internal/claude"
	"github.com/moasq/nanowave/internal/integrations"
	"github.com/moasq/nanowave/internal/terminal"
)

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

// Edit handles all work on an existing project — edits, fixes, refactors, etc.
// Claude Code determines the right action from the prompt itself.
// images is an optional list of image file paths to include.
func (p *Pipeline) Edit(ctx context.Context, prompt, projectDir, sessionID string, images []string) (*EditResult, error) {
	appName := readProjectAppName(projectDir)
	p.ensureProjectConfigs(projectDir)

	platform, platforms, watchProjectShape := detectProjectBuildHints(projectDir)
	isMulti := len(platforms) > 1

	// Load both editor and fixer skills — Claude decides which context to use.
	appendPrompt, err := composeCoderAppendPrompt("editor", platform)
	if err != nil {
		return nil, err
	}
	fixerSkill, err := composeCoderAppendPrompt("fixer", platform)
	if err == nil {
		appendPrompt += "\n\n" + fixerSkill
	}

	// Resolve existing integrations for this project (no setup prompts — just load stored configs).
	var activeProviders []integrations.ActiveProvider
	if p.manager != nil {
		activeProviders = p.manager.ResolveExisting(appName)
		if len(activeProviders) > 0 {
			var names []string
			for _, ap := range activeProviders {
				names = append(names, string(ap.Provider.ID()))
			}
			terminal.Detail("Integrations", strings.Join(names, ", "))
		}
	}

	// Inject integration prompt contributions (RevenueCat config, API keys, etc.)
	if len(activeProviders) > 0 {
		promptReq := integrations.PromptRequest{
			AppName: appName,
			Store:   p.manager.Store(),
		}
		contributions, err := p.manager.PromptContributions(ctx, promptReq, activeProviders)
		if err != nil {
			terminal.Warning(fmt.Sprintf("Integration prompt contributions failed: %v", err))
		}
		for _, c := range contributions {
			if c.SystemBlock != "" {
				appendPrompt += c.SystemBlock
			}
		}
	}

	// Build commands go into system prompt so Claude always knows how to build.
	if isMulti {
		buildCmds := multiPlatformBuildCommands(appName, platforms)
		var buildCmdStr strings.Builder
		for i, cmd := range buildCmds {
			fmt.Fprintf(&buildCmdStr, "%d. %s\n", i+1, cmd)
		}
		appendPrompt += "\n\nBuild commands (run ALL):\n" + buildCmdStr.String()
	} else {
		destination := canonicalBuildDestinationForShape(platform, watchProjectShape)
		appendPrompt += fmt.Sprintf("\n\nBuild command:\nxcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet build", appName, appName, destination)
	}

	// The user's prompt is the user message — Claude figures out the intent.
	userMsg := prompt

	// Build tool list: base tools + integration MCP tools
	tools := p.baseAgenticTools()
	if p.manager != nil && len(activeProviders) > 0 {
		tools = append(tools, p.manager.AgentTools(activeProviders)...)
	}

	progress := terminal.NewProgressDisplay("working", 0)
	progress.Start()

	resp, err := p.claude.GenerateStreaming(ctx, userMsg, claude.GenerateOpts{
		AppendSystemPrompt: appendPrompt,
		MaxTurns:           30,
		Model:              p.buildModel(),
		WorkDir:            projectDir,
		AllowedTools:       tools,
		SessionID:          sessionID,
		Images:             images,
	}, p.makeStreamCallback(progress))

	if err != nil {
		progress.StopWithError("Failed")
		return nil, fmt.Errorf("operation failed: %w", err)
	}

	progress.StopWithSuccess("Done!")
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
