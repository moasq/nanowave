package orchestration

import (
	"context"
	"fmt"
	"strings"

	"github.com/moasq/nanowave/internal/claude"
	"github.com/moasq/nanowave/internal/terminal"
)

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
	ensureProjectConfigs(projectDir)

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
		AllowedTools:       append([]string(nil), baseAgenticTools...),
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
