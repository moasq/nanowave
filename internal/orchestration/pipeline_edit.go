package orchestration

import (
	"context"
	"fmt"
	"strings"

	"github.com/moasq/nanowave/internal/claude"
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

// Edit modifies an existing project using Claude Code.
// images is an optional list of image file paths to include in the edit prompt.
func (p *Pipeline) Edit(ctx context.Context, prompt, projectDir, sessionID string, images []string) (*EditResult, error) {
	appName := readProjectAppName(projectDir)
	ensureProjectConfigs(projectDir)

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
		AllowedTools:       append([]string(nil), baseAgenticTools...),
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
