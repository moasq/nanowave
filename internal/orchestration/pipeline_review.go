package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/moasq/nanowave/internal/agentruntime"
	"github.com/moasq/nanowave/internal/skills"
	"github.com/moasq/nanowave/internal/terminal"
)

const maxReviewIterations = 5

// reviewAndIterate captures simulator screenshots, evaluates the UI via LLM,
// and iterates fixes until the UI passes or max iterations are reached.
// If the build fails during screenshot capture, it sends the errors to the LLM to fix
// and retries. Returns an error if the app cannot be built after all attempts.
func (p *Pipeline) reviewAndIterate(ctx context.Context, appName, projectDir string, plan *PlannerResult, sessionID string) (string, error) {
	reviewContent := skills.LoadRuleContent("review")
	tools := p.baseAgenticTools()

	for iteration := 1; iteration <= maxReviewIterations; iteration++ {
		if ctx.Err() != nil {
			return sessionID, ctx.Err()
		}

		progress := terminal.NewProgressDisplay("build", 0)
		progress.Start()
		progress.AddActivity(fmt.Sprintf("Capturing screenshots (pass %d/%d)...", iteration, maxReviewIterations))

		screenshotPath, simUDID, buildErrors, err := p.captureOrCollectErrors(ctx, appName, projectDir)

		if err != nil && buildErrors == "" {
			// Infrastructure failure (no simulator, etc.) — not a code issue, skip review
			progress.StopWithError("Screenshot capture failed")
			terminal.Warning(fmt.Sprintf("Screenshot infrastructure error: %v", err))
			return sessionID, nil
		}

		if buildErrors != "" {
			// Build failed — send errors to LLM to fix
			progress.StopWithError(fmt.Sprintf("Build failed (pass %d) — fixing...", iteration))

			fixProgress := terminal.NewProgressDisplay("build", 0)
			fixProgress.Start()
			fixProgress.AddActivity(fmt.Sprintf("Fixing build errors (pass %d/%d)...", iteration, maxReviewIterations))

			systemPrompt := p.composeReviewSystemPrompt(plan, reviewContent)
			userMsg := fmt.Sprintf(`The %s app failed to compile. Fix ALL errors below, then rebuild.

BUILD ERRORS:
%s

Fix process:
1. Read the error messages carefully.
2. Edit the Swift source files to fix each error.
3. After fixing all errors, rebuild: %s
4. If the rebuild fails, fix the new errors and rebuild again.
5. Keep going until the build succeeds.

IMPORTANT: Do not skip or ignore any errors. Every error must be fixed.`,
				appName, buildErrors,
				canonicalSimulatorBuildCommand(appName, plan.GetPlatform(), plan.GetWatchProjectShape()))

			opts := agentruntime.GenerateOpts{
				SystemPrompt: systemPrompt,
				MaxTurns:     20,
				Model:        p.modelForPhase(agentruntime.PhaseBuild),
				WorkDir:      projectDir,
				AllowedTools: tools,
				SessionID:    sessionID,
			}

			resp, fixErr := p.runtime.GenerateStreaming(ctx, userMsg, opts, p.makeStreamCallback(fixProgress))
			if fixErr != nil {
				fixProgress.StopWithError("Fix attempt failed")
				if iteration == maxReviewIterations {
					return sessionID, fmt.Errorf("build still broken after %d fix attempts: %w", iteration, fixErr)
				}
				continue
			}
			if resp != nil && resp.SessionID != "" {
				sessionID = resp.SessionID
			}
			fixProgress.StopWithSuccess(fmt.Sprintf("Fix pass %d complete", iteration))
			continue // retry capture on next iteration
		}

		// Build succeeded — now review the screenshot
		progress.StopWithSuccess("Screenshot captured")

		reviewProgress := terminal.NewProgressDisplay("build", 0)
		reviewProgress.Start()
		reviewProgress.AddActivity(fmt.Sprintf("Reviewing UI (pass %d/%d)...", iteration, maxReviewIterations))

		systemPrompt := p.composeReviewSystemPrompt(plan, reviewContent)
		userMsg := fmt.Sprintf(`Review this screenshot of the %s app.

Read the screenshot image at: %s

Evaluate the UI against these criteria:
1. **Layout** — Are views properly aligned? No overlapping content? Correct safe area usage?
2. **Text** — Is all text readable? No truncation? Appropriate font sizes?
3. **Colors** — Does the palette match the design spec? Good contrast?
4. **Sample data** — Does the app show meaningful sample data, not empty/placeholder screens?
5. **Components** — Are standard iOS components used correctly (NavigationStack, List, etc.)?

If there are issues:
- Fix them ONE AT A TIME — edit the Swift file, then rebuild.
- After fixing, rebuild: %s
- After rebuilding, capture a new screenshot: xcrun simctl io %s screenshot %s

If the UI looks good with no issues, respond with: REVIEW_PASSED

IMPORTANT: Fix only real visual issues. Minor cosmetic preferences are not issues.`,
			appName, screenshotPath,
			canonicalSimulatorBuildCommand(appName, plan.GetPlatform(), plan.GetWatchProjectShape()),
			simUDID, screenshotPath)

		opts := agentruntime.GenerateOpts{
			SystemPrompt: systemPrompt,
			MaxTurns:     15,
			Model:        p.modelForPhase(agentruntime.PhaseBuild),
			WorkDir:      projectDir,
			AllowedTools: tools,
			SessionID:    sessionID,
			Images:       []string{screenshotPath},
		}

		resp, reviewErr := p.runtime.GenerateStreaming(ctx, userMsg, opts, p.makeStreamCallback(reviewProgress))
		if reviewErr != nil {
			reviewProgress.StopWithError("Review failed")
			return sessionID, fmt.Errorf("review iteration %d: %w", iteration, reviewErr)
		}

		if resp != nil && resp.SessionID != "" {
			sessionID = resp.SessionID
		}

		if resp != nil && strings.Contains(resp.Result, "REVIEW_PASSED") {
			reviewProgress.StopWithSuccess("UI review passed")
			return sessionID, nil
		}

		reviewProgress.StopWithSuccess(fmt.Sprintf("Review pass %d — fixes applied", iteration))
	}

	// After all iterations, do a final build check to confirm the app compiles
	_, _, buildErrors, _ := p.captureOrCollectErrors(ctx, appName, projectDir)
	if buildErrors != "" {
		return sessionID, fmt.Errorf("app still has build errors after %d review passes:\n%s", maxReviewIterations, truncateBuildErrors(buildErrors, 500))
	}

	terminal.Info(fmt.Sprintf("Review completed after %d iterations", maxReviewIterations))
	return sessionID, nil
}

// captureOrCollectErrors attempts to build for simulator, install, launch, and capture.
// If the build fails, it returns the build errors as a string (not as an error) so the
// caller can send them to the LLM for fixing.
// Returns (screenshotPath, simUDID, buildErrors, infraError).
func (p *Pipeline) captureOrCollectErrors(ctx context.Context, appName, projectDir string) (string, string, string, error) {
	screenshotDir := filepath.Join(projectDir, "screenshots", "review")
	if err := os.MkdirAll(screenshotDir, 0o755); err != nil {
		return "", "", "", fmt.Errorf("create screenshot dir: %w", err)
	}

	// Find .xcodeproj
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return "", "", "", fmt.Errorf("read project dir: %w", err)
	}
	var xcodeprojName string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".xcodeproj") {
			xcodeprojName = e.Name()
			break
		}
	}
	if xcodeprojName == "" {
		return "", "", "", fmt.Errorf("no .xcodeproj found")
	}

	// Find a suitable iPhone simulator
	udid, _, err := findBestSimulator()
	if err != nil {
		return "", "", "", fmt.Errorf("find simulator: %w", err)
	}

	// Boot simulator
	bootCmd := exec.CommandContext(ctx, "xcrun", "simctl", "boot", udid)
	bootOut, bootErr := bootCmd.CombinedOutput()
	if bootErr != nil {
		text := strings.ToLower(string(bootOut) + " " + bootErr.Error())
		if !strings.Contains(text, "already booted") && !strings.Contains(text, "current state: booted") {
			return "", "", "", fmt.Errorf("boot simulator: %s: %s", bootErr, bootOut)
		}
	}

	// Build for simulator
	derivedData := filepath.Join(projectDir, ".derivedData-review")
	buildCmd := exec.CommandContext(ctx, "xcodebuild",
		"-project", xcodeprojName,
		"-scheme", appName,
		"-destination", fmt.Sprintf("platform=iOS Simulator,id=%s", udid),
		"-derivedDataPath", derivedData,
		"-quiet", "build",
	)
	buildCmd.Dir = projectDir
	buildOut, buildErr := buildCmd.CombinedOutput()
	if buildErr != nil {
		os.RemoveAll(derivedData)
		outStr := extractBuildErrors(string(buildOut))
		// Return build errors as data, not as an error — caller will send to LLM
		return "", udid, outStr, nil
	}

	// Find the built .app
	appPath, err := findBuiltApp(derivedData)
	if err != nil {
		os.RemoveAll(derivedData)
		return "", "", "", fmt.Errorf("find built app: %w", err)
	}

	// Read bundle ID
	configData, _ := os.ReadFile(filepath.Join(projectDir, "project_config.json"))
	var cfg struct {
		BundleID string `json:"bundle_id"`
	}
	_ = json.Unmarshal(configData, &cfg)
	bundleID := cfg.BundleID
	if bundleID == "" {
		bundleID = "com.app." + strings.ToLower(appName)
	}

	// Install and launch
	installCmd := exec.CommandContext(ctx, "xcrun", "simctl", "install", udid, appPath)
	if out, err := installCmd.CombinedOutput(); err != nil {
		os.RemoveAll(derivedData)
		return "", "", "", fmt.Errorf("install app: %s: %s", err, out)
	}

	launchCmd := exec.CommandContext(ctx, "xcrun", "simctl", "launch", udid, bundleID)
	if out, err := launchCmd.CombinedOutput(); err != nil {
		os.RemoveAll(derivedData)
		return "", "", "", fmt.Errorf("launch app: %s: %s", err, out)
	}

	// Wait for app to render
	select {
	case <-ctx.Done():
		os.RemoveAll(derivedData)
		return "", "", "", ctx.Err()
	case <-time.After(4 * time.Second):
	}

	// Capture screenshot
	screenshotPath := filepath.Join(screenshotDir, fmt.Sprintf("review_%d.png", time.Now().Unix()))
	captureCmd := exec.CommandContext(ctx, "xcrun", "simctl", "io", udid, "screenshot", screenshotPath)
	if out, err := captureCmd.CombinedOutput(); err != nil {
		os.RemoveAll(derivedData)
		return "", "", "", fmt.Errorf("capture screenshot: %s: %s", err, out)
	}

	os.RemoveAll(derivedData)
	return screenshotPath, udid, "", nil
}

// extractBuildErrors extracts just the error lines from xcodebuild output.
func extractBuildErrors(output string) string {
	var errors []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "error:") || strings.Contains(trimmed, "** BUILD FAILED **") {
			errors = append(errors, trimmed)
		}
	}
	if len(errors) == 0 {
		// Fallback: return last 2000 chars
		if len(output) > 2000 {
			return output[len(output)-2000:]
		}
		return output
	}
	return strings.Join(errors, "\n")
}

func truncateBuildErrors(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// composeReviewSystemPrompt builds the system prompt for the UI review / fix phase.
func (p *Pipeline) composeReviewSystemPrompt(plan *PlannerResult, reviewContent string) string {
	var b strings.Builder

	appendPromptSection(&b, "Role", "You are a senior iOS developer. Fix build errors and evaluate simulator screenshots against the build plan.")
	appendPromptSection(&b, "Coder", coderPromptForPlatform(plan.GetPlatform()))
	appendXMLSection(&b, "constraints", sharedConstraints)
	if reviewContent != "" {
		appendPromptSection(&b, "Review Guidelines", reviewContent)
	}
	appendXMLSection(&b, "verification", composeSelfCheck(plan.GetPlatform()))

	return b.String()
}

// findBestSimulator locates the best available iPhone simulator.
func findBestSimulator() (udid, name string, err error) {
	out, err := exec.Command("xcrun", "simctl", "list", "devices", "available", "-j").Output()
	if err != nil {
		return "", "", fmt.Errorf("list simulators: %w", err)
	}

	type simDevice struct {
		Name                 string `json:"name"`
		UDID                 string `json:"udid"`
		IsAvailable          bool   `json:"isAvailable"`
		DeviceTypeIdentifier string `json:"deviceTypeIdentifier"`
	}
	var result struct {
		Devices map[string][]simDevice `json:"devices"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", "", fmt.Errorf("parse simulator list: %w", err)
	}

	type candidate struct {
		name, udid, runtime string
	}
	var candidates []candidate
	for runtime, devs := range result.Devices {
		if !strings.Contains(runtime, "iOS") {
			continue
		}
		for _, d := range devs {
			if !d.IsAvailable {
				continue
			}
			if strings.Contains(strings.ToLower(d.DeviceTypeIdentifier), "iphone") {
				candidates = append(candidates, candidate{d.Name, d.UDID, runtime})
			}
		}
	}

	if len(candidates) == 0 {
		return "", "", fmt.Errorf("no iPhone simulator found")
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if simScore(c.name) > simScore(best.name) || (simScore(c.name) == simScore(best.name) && c.runtime > best.runtime) {
			best = c
		}
	}

	return best.udid, best.name, nil
}

func simScore(name string) int {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "pro max") {
		return 3
	}
	if strings.Contains(lower, "pro") {
		return 2
	}
	return 1
}

// findBuiltApp locates the built .app inside derivedData.
func findBuiltApp(derivedData string) (string, error) {
	productsDir := filepath.Join(derivedData, "Build", "Products")
	entries, err := os.ReadDir(productsDir)
	if err != nil {
		return "", fmt.Errorf("read products dir: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		subDir := filepath.Join(productsDir, e.Name())
		subEntries, err := os.ReadDir(subDir)
		if err != nil {
			continue
		}
		for _, se := range subEntries {
			if strings.HasSuffix(se.Name(), ".app") {
				return filepath.Join(subDir, se.Name()), nil
			}
		}
	}
	return "", fmt.Errorf("no .app found in %s", productsDir)
}

// canonicalSimulatorBuildCommand returns the build command string for simulator builds.
func canonicalSimulatorBuildCommand(appName, platform, watchProjectShape string) string {
	destination := canonicalSimulatorBuildDestination(platform, watchProjectShape)
	return fmt.Sprintf("xcodebuild -project %s.xcodeproj -scheme %s -destination '%s' -quiet build", appName, appName, destination)
}
