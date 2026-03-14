package service

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/moasq/nanowave/internal/claude"
	"github.com/moasq/nanowave/internal/orchestration"
	"github.com/moasq/nanowave/internal/storage"
	"github.com/moasq/nanowave/internal/terminal"
)

const maxAutoFixAttempts = 3

type xcodeBuildSpec struct {
	label    string
	platform string
	args     []string
}

func (s xcodeBuildSpec) commandString() string {
	parts := make([]string, 0, len(s.args)+1)
	parts = append(parts, "xcodebuild")
	for _, arg := range s.args {
		parts = append(parts, quoteCommandArg(arg))
	}
	return strings.Join(parts, " ")
}

type buildFailure struct {
	spec   xcodeBuildSpec
	output []byte
	err    error
}

type buildRunner func(context.Context, xcodeBuildSpec) ([]byte, error)

func quoteCommandArg(arg string) string {
	if arg == "" {
		return `""`
	}
	if strings.ContainsAny(arg, " \t\n\"'\\") {
		return strconv.Quote(arg)
	}
	return arg
}

func runXcodeBuildSpec(ctx context.Context, workDir string, spec xcodeBuildSpec) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "xcodebuild", spec.args...)
	cmd.Dir = workDir
	return cmd.CombinedOutput()
}

func verifyBuildSpecs(ctx context.Context, specs []xcodeBuildSpec, run buildRunner) *buildFailure {
	for _, spec := range specs {
		output, err := run(ctx, spec)
		if err != nil {
			return &buildFailure{
				spec:   spec,
				output: output,
				err:    err,
			}
		}
	}
	return nil
}

func runBuildFixLoop(ctx context.Context, initialFailure *buildFailure, specs []xcodeBuildSpec, run buildRunner, maxAttempts int, applyFix func(context.Context, buildFailure, int) error) (*buildFailure, int, error) {
	if initialFailure == nil {
		return nil, 0, nil
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	failure := initialFailure
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := applyFix(ctx, *failure, attempt); err != nil {
			return failure, attempt, err
		}

		failure = verifyBuildSpecs(ctx, specs, run)
		if failure == nil {
			return nil, attempt, nil
		}
	}

	return failure, maxAttempts, nil
}

func buildOutputExcerpt(output []byte) string {
	text := strings.TrimSpace(string(output))
	if text == "" {
		return "(no xcodebuild output captured)"
	}

	const maxChars = 32000
	if len(text) <= maxChars {
		return text
	}

	tail := text[len(text)-maxChars:]
	if idx := strings.Index(tail, "\n"); idx >= 0 && idx+1 < len(tail) {
		tail = tail[idx+1:]
	}

	return fmt.Sprintf("[xcodebuild output truncated to the last %d characters]\n%s", maxChars, strings.TrimSpace(tail))
}

func buildFixPrompt(projectName string, specs []xcodeBuildSpec, failure buildFailure) (string, string) {
	appendPrompt, err := orchestration.ComposeFixerAppendPrompt(failure.spec.platform)
	if err != nil {
		appendPrompt = ""
	}

	var allCommands strings.Builder
	for i, spec := range specs {
		fmt.Fprintf(&allCommands, "%d. %s\n   %s\n", i+1, spec.label, spec.commandString())
	}

	cliMode := `## CLI Fix Mode

You are repairing an existing Xcode project from concrete xcodebuild diagnostics.
Work autonomously: inspect the relevant files, fix the root cause, and rebuild until every validation command succeeds.`
	if strings.TrimSpace(appendPrompt) == "" {
		appendPrompt = cliMode
	} else {
		appendPrompt = appendPrompt + "\n\n" + cliMode
	}

	fence := "```"
	userMsg := fmt.Sprintf(`Fix the current Xcode build failure in this existing project.

Project: %s
Failing validation target: %s
Failure summary: %v

Required process:
1. Read CLAUDE.md and inspect the failing source files before editing.
2. Reproduce with the exact failing xcodebuild command below.
3. Fix the root cause and rebuild until the failing command succeeds.
4. Then run the full validation list and keep every command green.
5. Stop only when all validation commands succeed with zero compiler errors.

Validation commands:
%s
Currently failing command:
%s

Latest xcodebuild output:
%stext
%s
%s

Notes:
- Treat warnings as secondary unless they are the root cause of the non-zero exit status.
- If the failure is caused by project configuration, permissions, or entitlements, use xcodegen tools instead of manually editing the .xcodeproj.
- Focus first on the concrete compiler/build errors that caused exit status 65.`, projectName, failure.spec.label, failure.err, allCommands.String(), failure.spec.commandString(), fence, buildOutputExcerpt(failure.output), fence)

	return appendPrompt, userMsg
}

func (s *Service) runAutoFixLoop(ctx context.Context, project *storage.Project, specs []xcodeBuildSpec, initialFailure *buildFailure) error {
	failure, attempts, err := runBuildFixLoop(ctx, initialFailure, specs, func(ctx context.Context, spec xcodeBuildSpec) ([]byte, error) {
		return runXcodeBuildSpec(ctx, project.ProjectPath, spec)
	}, maxAutoFixAttempts, func(ctx context.Context, failure buildFailure, attempt int) error {
		spinner := terminal.NewSpinner(fmt.Sprintf("Auto-fixing %s (attempt %d/%d)...", failure.spec.label, attempt, maxAutoFixAttempts))
		spinner.Start()

		appendPrompt, userMsg := buildFixPrompt(projectName(project), specs, failure)
		resp, fixErr := s.claude.GenerateStreaming(ctx, userMsg, claude.GenerateOpts{
			AppendSystemPrompt: appendPrompt,
			AllowedTools:       orchestration.DefaultAgenticTools(),
			MaxTurns:           20,
			Model:              s.model,
			WorkDir:            project.ProjectPath,
			SessionID:          project.SessionID,
		}, func(ev claude.StreamEvent) {})
		if fixErr != nil {
			spinner.Stop()
			return fixErr
		}

		if resp != nil {
			s.usageStore.RecordUsage(resp.TotalCostUSD, resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CacheReadInputTokens, resp.Usage.CacheCreationInputTokens)
			if resp.SessionID != "" {
				project.SessionID = resp.SessionID
				s.projectStore.Save(project)
			}
		}

		spinner.Stop()
		return nil
	})
	if err != nil {
		terminal.Error("Auto-fix failed")
		return fmt.Errorf("auto-fix attempt %d failed for %s: %w\n%s", attempts, failure.spec.label, err, string(failure.output))
	}
	if failure != nil {
		terminal.Error(fmt.Sprintf("Build still failing after %d auto-fix attempts (%s)", attempts, failure.spec.label))
		terminal.Info("Try describing the issue to fix it")
		return fmt.Errorf("xcodebuild failed after %d fix attempts: %w\n%s", attempts, failure.err, string(failure.output))
	}

	terminal.Success("Build fixed")
	return nil
}
