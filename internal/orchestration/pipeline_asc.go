package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/moasq/nanowave/internal/appleauth"
	"github.com/moasq/nanowave/internal/asc"
	"github.com/moasq/nanowave/internal/claude"
	"github.com/moasq/nanowave/internal/terminal"
	"golang.org/x/term"
)

// ASCFull runs a full App Store Connect operation with pre-flight checks,
// guided auth setup, agreement verification, app selection,
// and HITL confirmations for destructive actions.
func (p *Pipeline) ASCFull(ctx context.Context, prompt, projectDir, sessionID string) (*asc.Result, error) {
	restore := redirectLogsToFile(projectDir)
	defer restore()

	log.Printf("[asc] starting ASCFull prompt=%q projectDir=%s sessionID=%s", prompt, projectDir, sessionID)

	appName := readProjectAppName(projectDir)
	fmt.Printf("\n%sNanowave Connect%s\n", terminal.Bold, terminal.Reset)
	if appName != "" {
		terminal.Detail("Project", appName)
	}
	fmt.Printf("\n  %sPreflight%s\n", terminal.Bold, terminal.Reset)

	cl := terminal.NewChecklist()

	// 1. asc CLI
	cl.StartItem("Checking asc CLI")
	if _, err := exec.LookPath("asc"); err != nil {
		log.Printf("[asc] asc CLI not found in PATH, attempting install")
		cl.CompleteItem(terminal.ChecklistWarning, "asc CLI not found — installing...")

		installed := p.installASCCLI(ctx)
		if !installed {
			log.Printf("[asc] all installation methods failed")
			return nil, fmt.Errorf("asc CLI installation failed")
		}

		if _, err := exec.LookPath("asc"); err != nil {
			log.Printf("[asc] asc installed but not in PATH")
			terminal.Warning("asc was installed but is not in your PATH.")
			terminal.Info("You may need to restart your terminal or add it to your PATH.")
			return nil, fmt.Errorf("asc CLI not in PATH after installation")
		}
		cl.StartItem("Verifying asc CLI")
	}
	log.Printf("[asc] asc CLI available")
	cl.CompleteItem(terminal.ChecklistSuccess, "asc CLI ready")

	// 2. Authentication
	cl.StartItem("Checking authentication")
	if !p.checkASCAuth(ctx) {
		log.Printf("[asc] not authenticated, starting guided setup")
		cl.CompleteItem(terminal.ChecklistError, "Not authenticated with App Store Connect")
		fmt.Println()
		if !p.setupASCAuth(ctx) {
			log.Printf("[asc] auth setup failed or skipped by user")
			return nil, fmt.Errorf("not authenticated with App Store Connect")
		}
		log.Printf("[asc] auth setup completed successfully")
		cl.StartItem("Verifying authentication")
	}
	cl.CompleteItem(terminal.ChecklistSuccess, "Authenticated with App Store Connect")

	// 3. App context
	cl.StartItem("Verifying app in App Store Connect")
	preflight := p.gatherASCContext(ctx, projectDir)
	log.Printf("[asc] preflight result: appID=%s appName=%q bundleID=%s localizations=%v",
		preflight.AppID, preflight.AppName, preflight.BundleID, preflight.Localizations)
	if preflight.AppID != "" {
		detail := preflight.BundleID
		if preflight.AppName != "" {
			detail = fmt.Sprintf("%s (%s)", preflight.AppName, preflight.BundleID)
		}
		cl.CompleteItem(terminal.ChecklistSuccess, detail)
	} else {
		cl.CompleteItem(terminal.ChecklistError, "No app found")
		log.Printf("[asc] GATE FAILED: no app ID — cannot proceed without an app in ASC")
		return nil, fmt.Errorf("no App Store Connect app found for %s. Create the app in App Store Connect first, then try again", preflight.BundleID)
	}

	// 6. Icon
	platform := "ios"
	if configData, err := os.ReadFile(filepath.Join(projectDir, "project_config.json")); err == nil {
		var cfg struct {
			Platform string `json:"platform"`
		}
		if json.Unmarshal(configData, &cfg) == nil && cfg.Platform != "" {
			platform = cfg.Platform
		}
	}

	cl.StartItem("Checking app icon")
	iconFound, iconCount := p.checkAppIcon(projectDir, platform)
	if iconFound {
		cl.CompleteItem(terminal.ChecklistSuccess, fmt.Sprintf("App icon ready (%d sizes)", iconCount))
	} else {
		cl.CompleteItem(terminal.ChecklistWarning, "No app icon")
		p.offerIconUpload(ctx, projectDir, platform)
	}

	// 7. Xcode project
	cl.StartItem("Regenerating Xcode project")
	if err := p.regenerateXcodeProject(ctx, projectDir); err != nil {
		cl.CompleteItem(terminal.ChecklistWarning, "Xcode project regeneration skipped")
	} else {
		cl.CompleteItem(terminal.ChecklistSuccess, "Xcode project regenerated")
	}

	// Load API credentials silently (no checklist item — not a meaningful gate)
	var apiKeySection string
	if cred, credErr := asc.LoadCredential(); credErr == nil {
		log.Printf("[asc] credential loaded: keyID=%s issuerID=%s", cred.KeyID, cred.IssuerID)
		if keyPath, writeErr := asc.WriteKeyFile(cred, projectDir); writeErr == nil {
			defer os.Remove(keyPath)
			apiKeySection = fmt.Sprintf(`
## API Key Authentication
An API key is available for xcodebuild authentication (no keychain prompts):
- Key path: %s
- Key ID: %s
- Issuer ID: %s
Use these flags with xcodebuild: -authenticationKeyPath %s -authenticationKeyID %s -authenticationKeyIssuerID %s
`, keyPath, cred.KeyID, cred.IssuerID, keyPath, cred.KeyID, cred.IssuerID)
			log.Printf("[asc] API key written to %s", keyPath)
		} else {
			log.Printf("[asc] failed to write API key file: %v", writeErr)
		}
	} else {
		log.Printf("[asc] no API key credential available: %v", credErr)
	}

	cl.Finish()

	// --- Publishing phase (ProgressDisplay for Claude streaming) ---
	systemPrompt := p.ComposeASCSystemPrompt(projectDir, preflight)
	if apiKeySection != "" {
		systemPrompt += apiKeySection
	}

	allowedTools := asc.AgentTools()
	log.Printf("[asc] starting Claude streaming with %d allowed tools, maxTurns=30", len(allowedTools))

	fmt.Printf("\n  %sRunning%s\n", terminal.Bold, terminal.Reset)

	// progressRouter provides thread-safe access to the current progress display
	// and its callback. Both may be replaced during HITL interactions.
	var progressMu sync.Mutex
	progress := terminal.NewProgressDisplay("asc", 0)
	progress.Start()
	terminalCallback := newProgressCallback(progress)

	toolsUsed := map[string]bool{}

	// HITL (human-in-the-loop) state: detect when Claude asks a question
	// and is waiting for user input. We track the last assistant message
	// and use a timer to detect idle periods.
	var (
		hitlMu          sync.Mutex
		hitlTimer       *time.Timer
		lastAssistant   string
		pendingQuestion = make(chan string, 1)
	)

	const hitlIdleTimeout = 3 * time.Second

	resetHITLTimer := func() {
		hitlMu.Lock()
		defer hitlMu.Unlock()
		if hitlTimer != nil {
			hitlTimer.Stop()
		}
		hitlTimer = nil
		lastAssistant = ""
	}

	eventCount := 0
	streamCallback := func(ev claude.StreamEvent) {
		eventCount++
		progressMu.Lock()
		cb := terminalCallback
		progressMu.Unlock()
		cb(ev)

		// Log events for debugging
		switch ev.Type {
		case "tool_use_start":
			log.Printf("[asc] tool starting: %s", ev.ToolName)
		case "tool_use":
			if !toolsUsed[ev.ToolName] {
				log.Printf("[asc] tool first use: %s", ev.ToolName)
			}
			toolsUsed[ev.ToolName] = true
		case "assistant":
			if ev.Text != "" {
				truncated := ev.Text
				if len(truncated) > 200 {
					truncated = truncated[:200] + "..."
				}
				log.Printf("[asc] assistant: %s", truncated)
			}
		case "result":
			log.Printf("[asc] result event received")
		case "system":
			log.Printf("[asc] system event: subtype=%s sessionID=%s", ev.Subtype, ev.SessionID)
		}

		if ev.Type == "tool_use" && ev.ToolName != "" {
			resetHITLTimer()
		}

		if ev.Type == "result" {
			resetHITLTimer()
		}

		// When we get a full assistant message, start the idle timer.
		// If no tool_use or result follows within the timeout, Claude is
		// likely waiting for user input.
		if ev.Type == "assistant" && ev.Text != "" {
			hitlMu.Lock()
			lastAssistant = ev.Text
			if hitlTimer != nil {
				hitlTimer.Stop()
			}
			msg := ev.Text
			hitlTimer = time.AfterFunc(hitlIdleTimeout, func() {
				hitlMu.Lock()
				stillPending := lastAssistant == msg
				hitlMu.Unlock()
				if stillPending {
					select {
					case pendingQuestion <- msg:
					default:
					}
				}
			})
			hitlMu.Unlock()
		}
	}

	// Wrap user prompt with a reminder about confirmation requirements
	wrappedPrompt := prompt + "\n\nReminder: Before submitting for Beta App Review, ask me for confirmation first."

	session, err := p.claude.StartInteractiveStreaming(ctx, wrappedPrompt, claude.GenerateOpts{
		AppendSystemPrompt: systemPrompt,
		MaxTurns:           30,
		Model:              p.buildModel(),
		WorkDir:            projectDir,
		AllowedTools:       allowedTools,
	}, streamCallback)

	if err != nil {
		log.Printf("[asc] interactive session start failed: %v", err)
		progress.StopWithError("ASC operation failed")
		return nil, fmt.Errorf("asc operation failed: %w", err)
	}

	// HITL input loop: runs concurrently with stream processing.
	// When an idle timeout fires, we pause the progress display,
	// show Claude's question, collect user input, and send it back.
	hitlDone := make(chan struct{})
	go func() {
		defer close(hitlDone)
		for {
			select {
			case question, ok := <-pendingQuestion:
				if !ok {
					return
				}
				log.Printf("[asc] HITL: Claude is asking for input: %s", question)

				// Pause progress display to show the question cleanly
				progressMu.Lock()
				progress.Stop()
				progressMu.Unlock()

				fmt.Println()
				fmt.Printf("  %s%s%s\n\n", terminal.Dim, question, terminal.Reset)

				// Read user input
				userInput := terminal.ReadSimpleLine()
				log.Printf("[asc] HITL: user responded: %s", userInput)

				if userInput == "" {
					userInput = "(no response)"
				}

				// Send the reply and resume progress
				if sendErr := session.SendUserMessage(userInput); sendErr != nil {
					log.Printf("[asc] HITL: failed to send user message: %v", sendErr)
					return
				}

				fmt.Println()
				fmt.Printf("  %sRunning%s\n", terminal.Bold, terminal.Reset)
				newProgress := terminal.NewProgressDisplay("asc", 0)
				newProgress.Start()
				newCb := newProgressCallback(newProgress)

				progressMu.Lock()
				progress = newProgress
				terminalCallback = newCb
				progressMu.Unlock()

			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for the session to complete.
	resp, err := session.Wait()

	// Check if HITL is actively prompting the user (session ended while waiting for input).
	// This happens when Claude asks a question and then the session completes.
	// We need to wait for the user's response and resume the session.
	hitlMu.Lock()
	hitlActive := lastAssistant != ""
	hitlMu.Unlock()

	if hitlActive && err == nil && resp != nil && resp.SessionID != "" {
		// Drain the pending question if it hasn't fired yet
		select {
		case <-pendingQuestion:
		default:
		}

		// The session ended while Claude was asking the user. Wait for input
		// and resume the session with the user's response.
		log.Printf("[asc] session ended while HITL is active, waiting for user input to resume")

		progressMu.Lock()
		progress.Stop()
		progressMu.Unlock()

		// Show the question if not already shown
		if resp.Result != "" {
			fmt.Println()
			fmt.Printf("  %s%s%s\n\n", terminal.Dim, resp.Result, terminal.Reset)
		}

		userInput := terminal.ReadSimpleLine()
		log.Printf("[asc] HITL resume: user responded: %s", userInput)
		if userInput == "" {
			userInput = "(no response)"
		}

		fmt.Println()
		fmt.Printf("  %sRunning%s\n", terminal.Bold, terminal.Reset)
		newProgress := terminal.NewProgressDisplay("asc", 0)
		newProgress.Start()
		newCb := newProgressCallback(newProgress)

		progressMu.Lock()
		progress = newProgress
		terminalCallback = newCb
		progressMu.Unlock()

		// Resume session with user's response
		resumeSession, resumeErr := p.claude.StartInteractiveStreaming(ctx, userInput, claude.GenerateOpts{
			AppendSystemPrompt: systemPrompt,
			MaxTurns:           30,
			Model:              p.buildModel(),
			WorkDir:            projectDir,
			AllowedTools:       allowedTools,
			SessionID:          resp.SessionID,
		}, func(ev claude.StreamEvent) {
			progressMu.Lock()
			cb := terminalCallback
			progressMu.Unlock()
			cb(ev)

			switch ev.Type {
			case "tool_use_start":
				log.Printf("[asc] [resume] tool starting: %s", ev.ToolName)
			case "tool_use":
				toolsUsed[ev.ToolName] = true
			case "assistant":
				if ev.Text != "" {
					truncated := ev.Text
					if len(truncated) > 200 {
						truncated = truncated[:200] + "..."
					}
					log.Printf("[asc] [resume] assistant: %s", truncated)
				}
			case "result":
				log.Printf("[asc] [resume] result event received")
			}
		})

		if resumeErr != nil {
			log.Printf("[asc] resume session failed: %v", resumeErr)
		} else {
			resumeResp, resumeWaitErr := resumeSession.Wait()
			resumeSession.CloseInput()
			if resumeWaitErr == nil && resumeResp != nil {
				resp = resumeResp
				err = nil
			} else if resumeWaitErr != nil {
				log.Printf("[asc] resume wait failed: %v", resumeWaitErr)
			}
		}
	}

	session.CloseInput()

	// Clean up HITL
	resetHITLTimer()
	close(pendingQuestion)
	<-hitlDone

	// Log stderr for debugging
	if stderr := session.Stderr(); stderr != "" {
		log.Printf("[asc] stderr: %s", stderr)
	}
	log.Printf("[asc] total stream events received: %d", eventCount)

	if err != nil {
		log.Printf("[asc] streaming failed: %v", err)
		progress.StopWithError("ASC operation failed")
		return nil, fmt.Errorf("asc operation failed: %w", err)
	}

	log.Printf("[asc] streaming complete: sessionID=%s cost=$%.4f input=%d output=%d cacheRead=%d cacheCreated=%d",
		resp.SessionID, resp.TotalCostUSD, resp.Usage.InputTokens, resp.Usage.OutputTokens,
		resp.Usage.CacheReadInputTokens, resp.Usage.CacheCreationInputTokens)

	progress.StopWithSuccess("Done")

	// Show Claude's summary of what was accomplished
	if summary := extractASCSummary(resp.Result); summary != "" {
		fmt.Printf("\n  %s%s%s\n", terminal.Dim, summary, terminal.Reset)
	}

	showCost(resp)

	return &asc.Result{
		Summary:      resp.Result,
		SessionID:    resp.SessionID,
		TotalCostUSD: resp.TotalCostUSD,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
		CacheRead:    resp.Usage.CacheReadInputTokens,
		CacheCreated: resp.Usage.CacheCreationInputTokens,
		ToolsUsed:    toolsUsed,
	}, nil
}

// extractASCSummary extracts a concise summary from Claude's response text.
// Returns up to 3 lines of meaningful output, skipping code blocks and noise.
func extractASCSummary(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	var result []string
	inCodeBlock := false
	const maxLines = 4
	const maxWidth = 90

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock || trimmed == "" {
			continue
		}
		// Skip JSON-looking lines
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			continue
		}
		// Truncate long lines
		if len(trimmed) > maxWidth {
			trimmed = trimmed[:maxWidth] + "..."
		}
		result = append(result, trimmed)
		if len(result) >= maxLines {
			break
		}
	}

	return strings.Join(result, "\n  ")
}

// redirectLogsToFile redirects log output to a file in the project's .nanowave
// directory. Returns a function to restore the previous log output.
// In non-interactive mode, logs stay on stdout.
func redirectLogsToFile(projectDir string) func() {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return func() {}
	}
	logDir := filepath.Join(projectDir, ".nanowave")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return func() {}
	}
	f, err := os.Create(filepath.Join(logDir, "publish.log"))
	if err != nil {
		return func() {}
	}
	prev := log.Writer()
	log.SetOutput(f)
	return func() {
		log.SetOutput(prev)
		f.Close()
	}
}

// installASCCLI attempts to install the asc CLI via Homebrew or install script.
func (p *Pipeline) installASCCLI(ctx context.Context) bool {
	// Try Homebrew first
	if _, brewErr := exec.LookPath("brew"); brewErr == nil {
		installSpinner := terminal.NewSpinner("Installing asc via Homebrew...")
		installSpinner.Start()
		installCmd := exec.CommandContext(ctx, "brew", "install", "asc")
		if _, installErr := installCmd.CombinedOutput(); installErr == nil {
			installSpinner.StopWithMessage(fmt.Sprintf("  %s%s\u2713%s asc installed via Homebrew", terminal.Bold, terminal.Green, terminal.Reset))
			log.Printf("[asc] installed via Homebrew")
			return true
		}
		log.Printf("[asc] Homebrew install failed")
		installSpinner.StopWithMessage(fmt.Sprintf("  %s%s!%s Homebrew install failed, trying install script...", terminal.Bold, terminal.Yellow, terminal.Reset))
	}

	// Fallback to install script
	installSpinner := terminal.NewSpinner("Installing asc via install script...")
	installSpinner.Start()
	installCmd := exec.CommandContext(ctx, "bash", "-c", "curl -fsSL https://asccli.sh/install | bash")
	if _, installErr := installCmd.CombinedOutput(); installErr == nil {
		installSpinner.StopWithMessage(fmt.Sprintf("  %s%s\u2713%s asc installed via install script", terminal.Bold, terminal.Green, terminal.Reset))
		log.Printf("[asc] installed via install script")
		return true
	}
	log.Printf("[asc] install script failed")
	installSpinner.StopWithMessage(fmt.Sprintf("  %s%s\u2717%s asc installation failed", terminal.Bold, terminal.Red, terminal.Reset))
	return false
}

// regenerateXcodeProject runs xcodegen generate if project.yml exists.
func (p *Pipeline) regenerateXcodeProject(ctx context.Context, projectDir string) error {
	projectYMLPath := filepath.Join(projectDir, "project.yml")
	if _, err := os.Stat(projectYMLPath); err != nil {
		log.Printf("[asc] no project.yml found, skipping xcodegen regeneration")
		return err
	}

	log.Printf("[asc] regenerating Xcode project from project.yml")
	genCmd := exec.CommandContext(ctx, "xcodegen", "generate")
	genCmd.Dir = projectDir
	if out, genErr := genCmd.CombinedOutput(); genErr != nil {
		log.Printf("[asc][xcodegen] xcodegen generate FAILED: %s: %v", string(out), genErr)
		return genErr
	}
	log.Printf("[asc][xcodegen] xcodegen generate succeeded")
	return nil
}

// checkASCAuth verifies authentication by making a real API call.
// If `asc apps list --output json` succeeds and returns a valid ASC envelope,
// the user has working credentials. No string matching on CLI output.
func (p *Pipeline) checkASCAuth(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "asc", "apps", "list", "--output", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[asc] auth check failed: asc apps list returned error: %v", err)
		return false
	}
	// Valid auth returns an ASC envelope with a "data" array
	var env asc.Envelope
	ok := json.Unmarshal(output, &env) == nil && env.Data != nil
	log.Printf("[asc] auth check: valid=%v apps=%d", ok, len(env.Data))
	return ok
}

// setupASCAuth guides the user through App Store Connect authentication.
// Primary flow: Apple ID + password + OTP (fully automated API key creation).
// Fallback: manual API key entry. Returns true on success.
func (p *Pipeline) setupASCAuth(ctx context.Context) bool {
	log.Printf("[asc] starting guided auth setup")
	terminal.Info("You need to authenticate with App Store Connect to continue.")
	fmt.Println()

	picked := terminal.Pick("", []terminal.PickerOption{
		{Label: "Sign in with Apple ID", Desc: "Automatic setup — enter email, password, and OTP"},
		{Label: "Enter API key manually", Desc: "I already have a Key ID, Issuer ID, and .p8 file"},
		{Label: "Skip", Desc: "Return to main prompt"},
	}, "")

	switch picked {
	case "Sign in with Apple ID":
		return p.setupASCAuthAppleID(ctx)
	case "Enter API key manually":
		return p.setupASCAuthManual(ctx)
	default:
		log.Printf("[asc] auth setup: user skipped")
		return false
	}
}

// setupASCAuthAppleID performs fully automated Apple ID authentication:
// SRP sign-in → 2FA verification → API key creation → asc auth login.
func (p *Pipeline) setupASCAuthAppleID(ctx context.Context) bool {
	log.Printf("[asc] starting Apple ID auth flow")

	// Collect Apple ID
	appleID := strings.TrimSpace(pipelineReadLineFn("Apple ID (email)"))
	if appleID == "" {
		terminal.Error("Apple ID is required.")
		return false
	}

	// Collect password (masked)
	fmt.Print("  Password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println() // newline after masked input
	if err != nil {
		log.Printf("[asc] failed to read password: %v", err)
		terminal.Error("Failed to read password.")
		return false
	}
	password := string(passwordBytes)
	if password == "" {
		terminal.Error("Password is required.")
		return false
	}

	// Create auth client and fetch service key
	spinner := terminal.NewSpinner("Connecting to Apple...")
	spinner.Start()

	client, err := appleauth.NewClient()
	if err != nil {
		log.Printf("[asc] failed to create auth client: %v", err)
		spinner.StopWithMessage(fmt.Sprintf("  %s%s✗%s Failed to initialize", terminal.Bold, terminal.Red, terminal.Reset))
		return false
	}

	if err := client.FetchServiceKey(ctx); err != nil {
		log.Printf("[asc] failed to fetch service key: %v", err)
		spinner.StopWithMessage(fmt.Sprintf("  %s%s✗%s Failed to connect to Apple", terminal.Bold, terminal.Red, terminal.Reset))
		terminal.Error(err.Error())
		return false
	}
	spinner.StopWithMessage(fmt.Sprintf("  %s%s✓%s Connected to Apple", terminal.Bold, terminal.Green, terminal.Reset))

	// SRP sign-in
	spinner = terminal.NewSpinner("Signing in...")
	spinner.Start()

	authState, err := client.SignIn(ctx, appleID, password)
	if err != nil {
		log.Printf("[asc] sign-in failed: %v", err)
		spinner.StopWithMessage(fmt.Sprintf("  %s%s✗%s Sign-in failed", terminal.Bold, terminal.Red, terminal.Reset))
		terminal.Error(err.Error())
		return false
	}
	spinner.StopWithMessage(fmt.Sprintf("  %s%s✓%s Signed in", terminal.Bold, terminal.Green, terminal.Reset))

	// Handle 2FA if required
	if authState != nil {
		log.Printf("[asc] 2FA required: trustedDevices=%v phones=%d codeLen=%d",
			authState.HasTrustedDevices, len(authState.TrustedPhones), authState.CodeLength)

		if !p.handleApple2FA(ctx, client, authState) {
			return false
		}
	}

	// Trust session
	spinner = terminal.NewSpinner("Establishing session...")
	spinner.Start()
	if err := client.TrustSession(ctx); err != nil {
		log.Printf("[asc] trust session failed: %v", err)
		spinner.StopWithMessage(fmt.Sprintf("  %s%s✗%s Failed to establish session", terminal.Bold, terminal.Red, terminal.Reset))
		terminal.Error(err.Error())
		return false
	}
	spinner.StopWithMessage(fmt.Sprintf("  %s%s✓%s Session established", terminal.Bold, terminal.Green, terminal.Reset))

	// Run onboarding: create API key + register with asc CLI
	spinner = terminal.NewSpinner("Creating API key and configuring asc CLI...")
	spinner.Start()
	result, err := appleauth.RunOnboarding(ctx, client, appleID)
	if err != nil {
		log.Printf("[asc] onboarding failed: %v", err)
		spinner.StopWithMessage(fmt.Sprintf("  %s%s✗%s API key setup failed", terminal.Bold, terminal.Red, terminal.Reset))
		terminal.Error(err.Error())
		return false
	}
	spinner.StopWithMessage(fmt.Sprintf("  %s%s✓%s API key created (Key ID: %s)", terminal.Bold, terminal.Green, terminal.Reset, result.KeyID))

	// Validate
	if !p.checkASCAuth(ctx) {
		log.Printf("[asc] auth validation failed after onboarding")
		terminal.Warning("API key was created but validation failed. You may need to wait a moment and retry.")
		return false
	}

	log.Printf("[asc] Apple ID auth complete: keyID=%s issuerID=%s team=%s", result.KeyID, result.IssuerID, result.TeamName)
	if result.TeamName != "" {
		terminal.Success(fmt.Sprintf("Authenticated as %s", result.TeamName))
	} else {
		terminal.Success("Authenticated with App Store Connect")
	}
	return true
}

// handleApple2FA guides the user through two-factor authentication.
func (p *Pipeline) handleApple2FA(ctx context.Context, client *appleauth.Client, state *appleauth.AuthState) bool {
	fmt.Println()

	var isSMS bool
	var selectedPhoneID int

	// When there's only one SMS option and no trusted devices, Apple auto-sends
	// the code — skip the picker and tell the user immediately.
	if !state.HasTrustedDevices && len(state.TrustedPhones) == 1 {
		phone := state.TrustedPhones[0]
		selectedPhoneID = phone.ID
		isSMS = true
		terminal.Info(fmt.Sprintf("A verification code was sent to %s", phone.NumberWithDialCode))
	} else {
		terminal.Info("Two-factor authentication required.")

		// Build verification options
		var options []terminal.PickerOption
		if state.HasTrustedDevices {
			options = append(options, terminal.PickerOption{
				Label: "Trusted device",
				Desc:  "Enter the code shown on your Apple device",
			})
		}
		for _, phone := range state.TrustedPhones {
			options = append(options, terminal.PickerOption{
				Label: fmt.Sprintf("SMS to %s", phone.NumberWithDialCode),
				Desc:  fmt.Sprintf("Send code via SMS (phone ID %d)", phone.ID),
			})
		}

		if len(options) == 0 {
			options = append(options, terminal.PickerOption{
				Label: "Trusted device",
				Desc:  "Enter the code shown on your Apple device",
			})
		}

		picked := terminal.Pick("Verification method", options, "")
		if picked == "" {
			return false
		}

		// Check if SMS was selected
		for _, phone := range state.TrustedPhones {
			label := fmt.Sprintf("SMS to %s", phone.NumberWithDialCode)
			if picked == label {
				isSMS = true
				selectedPhoneID = phone.ID
				break
			}
		}

		if isSMS {
			spinner := terminal.NewSpinner(fmt.Sprintf("Sending SMS code to %s...", state.TrustedPhones[0].NumberWithDialCode))
			spinner.Start()
			if err := client.RequestSMSCode(ctx, selectedPhoneID); err != nil {
				log.Printf("[asc] SMS code request failed: %v", err)
				spinner.StopWithMessage(fmt.Sprintf("  %s%s✗%s Failed to send SMS", terminal.Bold, terminal.Red, terminal.Reset))
				terminal.Error(err.Error())
				return false
			}
			spinner.StopWithMessage(fmt.Sprintf("  %s%s✓%s SMS code sent", terminal.Bold, terminal.Green, terminal.Reset))
		} else {
			terminal.Info("A verification code has been sent to your trusted devices.")
		}
	}

	// Collect the code
	code := strings.TrimSpace(pipelineReadLineFn(fmt.Sprintf("Verification code (%d digits)", state.CodeLength)))
	if code == "" {
		terminal.Error("Verification code is required.")
		return false
	}

	// Verify
	spinner := terminal.NewSpinner("Verifying code...")
	spinner.Start()

	var verifyErr error
	if isSMS {
		verifyErr = client.VerifySMSCode(ctx, code, selectedPhoneID)
	} else {
		verifyErr = client.VerifyDeviceCode(ctx, code)
	}

	if verifyErr != nil {
		log.Printf("[asc] verification failed: %v", verifyErr)
		spinner.StopWithMessage(fmt.Sprintf("  %s%s✗%s Verification failed", terminal.Bold, terminal.Red, terminal.Reset))
		terminal.Error(verifyErr.Error())
		return false
	}

	spinner.StopWithMessage(fmt.Sprintf("  %s%s✓%s Verified", terminal.Bold, terminal.Green, terminal.Reset))
	return true
}

// setupASCAuthManual collects API key credentials manually and runs asc auth login.
func (p *Pipeline) setupASCAuthManual(ctx context.Context) bool {
	log.Printf("[asc] starting manual API key auth flow")
	fmt.Println()
	fmt.Println("  1. Go to App Store Connect > Users and Access > Integrations > API Keys")
	fmt.Println("  2. Generate a new key (Admin role recommended)")
	fmt.Println("  3. Note your Key ID and Issuer ID")
	fmt.Println("  4. Download the .p8 private key file")
	fmt.Println()

	picked := terminal.Pick("", []terminal.PickerOption{
		{Label: "Open browser", Desc: "Open App Store Connect API keys page"},
		{Label: "I have my keys", Desc: "Enter credentials now"},
	}, "")

	if picked == "Open browser" {
		_ = exec.Command("open", "https://appstoreconnect.apple.com/access/integrations/api").Start()
		terminal.Info("Browser opened. Generate your API key, then enter the details below.")
		fmt.Println()
	}

	// Collect credentials
	issuerID := strings.TrimSpace(pipelineReadLineFn("Issuer ID"))
	if issuerID == "" {
		terminal.Error("Issuer ID is required.")
		return false
	}

	keyID := strings.TrimSpace(pipelineReadLineFn("Key ID"))
	if keyID == "" {
		terminal.Error("Key ID is required.")
		return false
	}

	terminal.Info("Drag and drop your .p8 file here, or paste the path:")
	p8Path := strings.TrimSpace(pipelineReadLineFn("Path to .p8 file"))
	if p8Path == "" {
		terminal.Error("Private key path is required.")
		return false
	}
	// Clean up path (drag and drop may add quotes or escapes)
	p8Path = strings.Trim(p8Path, "\"' ")
	p8Path = strings.ReplaceAll(p8Path, "\\ ", " ")

	// Expand ~ if present
	if strings.HasPrefix(p8Path, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			p8Path = filepath.Join(home, p8Path[1:])
		}
	}

	// Validate .p8 file exists
	if _, err := os.Stat(p8Path); err != nil {
		terminal.Error(fmt.Sprintf("Cannot find .p8 file at: %s", p8Path))
		return false
	}

	// Run asc auth login
	log.Printf("[asc] auth setup: running asc auth login keyID=%s issuerID=%s p8Path=%s", keyID, issuerID, p8Path)
	spinner := terminal.NewSpinner("Authenticating with App Store Connect...")
	spinner.Start()

	cmd := exec.CommandContext(ctx, "asc", "auth", "login",
		"--key-id", keyID,
		"--issuer-id", issuerID,
		"--private-key", p8Path,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[asc] auth login failed: %v output=%s", err, strings.TrimSpace(string(output)))
		spinner.StopWithMessage(fmt.Sprintf("  %s%s✗%s Authentication failed", terminal.Bold, terminal.Red, terminal.Reset))
		terminal.Error(fmt.Sprintf("asc auth login failed: %s", strings.TrimSpace(string(output))))
		return false
	}

	// Validate
	if !p.checkASCAuth(ctx) {
		log.Printf("[asc] auth login succeeded but validation failed")
		spinner.StopWithMessage(fmt.Sprintf("  %s%s✗%s Authentication failed", terminal.Bold, terminal.Red, terminal.Reset))
		terminal.Error("Credentials were saved but validation failed. Check your Key ID, Issuer ID, and .p8 file.")
		return false
	}

	log.Printf("[asc] manual auth setup complete")
	spinner.StopWithMessage(fmt.Sprintf("  %s%s✓%s Authenticated with App Store Connect", terminal.Bold, terminal.Green, terminal.Reset))
	return true
}

// gatherASCContext reads project config and matches against ASC apps.
// If asc_app_id is stored in project_config.json, it's used directly (no matching needed).
// Otherwise, matches by bundle ID, and persists the app ID for next time.
func (p *Pipeline) gatherASCContext(ctx context.Context, projectDir string) *asc.PreflightResult {
	log.Printf("[asc] gathering context from %s", projectDir)
	result := &asc.PreflightResult{}

	// Read project_config.json
	configPath := filepath.Join(projectDir, "project_config.json")
	var savedAppID string
	if data, err := os.ReadFile(configPath); err == nil {
		var cfg struct {
			AppName       string   `json:"app_name"`
			BundleID      string   `json:"bundle_id"`
			Platform      string   `json:"platform"`
			Localizations []string `json:"localizations"`
			ASCAppID      string   `json:"asc_app_id"`
		}
		if json.Unmarshal(data, &cfg) == nil {
			result.AppName = cfg.AppName
			result.BundleID = cfg.BundleID
			result.Localizations = cfg.Localizations
			savedAppID = cfg.ASCAppID
			log.Printf("[asc] project_config.json: appName=%q bundleID=%s ascAppID=%s localizations=%v", cfg.AppName, cfg.BundleID, cfg.ASCAppID, cfg.Localizations)
		}
	} else {
		log.Printf("[asc] project_config.json not found: %v", err)
	}

	// Fall back to readProjectAppName if not found
	if result.AppName == "" {
		result.AppName = readProjectAppName(projectDir)
		log.Printf("[asc] app name from xcodeproj: %q", result.AppName)
	}

	// Fast path: if we already have a saved ASC app ID, use it directly
	if savedAppID != "" {
		log.Printf("[asc] using saved asc_app_id=%s", savedAppID)

		cmd := exec.CommandContext(ctx, "asc", "apps", "get", "--id", savedAppID, "--output", "json")
		output, err := cmd.CombinedOutput()
		if err == nil {
			var resp struct {
				Data struct {
					ID         string `json:"id"`
					Attributes struct {
						Name     string `json:"name"`
						BundleID string `json:"bundleId"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if json.Unmarshal(output, &resp) == nil && resp.Data.ID != "" {
				result.AppID = resp.Data.ID
				result.AppName = resp.Data.Attributes.Name
				result.BundleID = resp.Data.Attributes.BundleID
				log.Printf("[asc] verified saved app: id=%s name=%q bundleID=%s", resp.Data.ID, resp.Data.Attributes.Name, resp.Data.Attributes.BundleID)
				return result
			}
		}
		log.Printf("[asc] saved asc_app_id=%s is stale, falling back to matching", savedAppID)
	}

	// Match by bundle ID
	if result.BundleID != "" {
		log.Printf("[asc] attempting app match by bundleID=%s", result.BundleID)

		cmd := exec.CommandContext(ctx, "asc", "apps", "list", "--output", "json")
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("[asc] app list failed: %v", err)
			return result
		}

		type ascAppAttrs struct {
			Name     string `json:"name"`
			BundleID string `json:"bundleId"`
		}
		type ascApp struct {
			ID       string
			Name     string
			BundleID string
		}

		var env asc.Envelope
		if json.Unmarshal(output, &env) == nil {
			apps := make([]ascApp, 0, len(env.Data))
			for _, d := range env.Data {
				var attrs ascAppAttrs
				if json.Unmarshal(d.Attributes, &attrs) == nil {
					apps = append(apps, ascApp{ID: d.ID, Name: attrs.Name, BundleID: attrs.BundleID})
				}
			}
			log.Printf("[asc] found %d apps in ASC account", len(apps))

			for _, app := range apps {
				if app.BundleID == result.BundleID {
					result.AppID = app.ID
					result.AppName = app.Name
					log.Printf("[asc] matched app: id=%s name=%q bundleID=%s", app.ID, app.Name, app.BundleID)
					asc.SaveAppID(projectDir, app.ID)
					return result
				}
			}

			// No match — offer picker
			log.Printf("[asc] no app matched bundleID=%s, offering picker with %d options", result.BundleID, len(apps))

			if len(apps) > 0 {
				options := make([]terminal.PickerOption, 0, len(apps)+2)

				createDesc := "Register a new app"
				if result.BundleID != "" && result.AppName != "" {
					createDesc = fmt.Sprintf("Create %s (%s)", result.AppName, result.BundleID)
				} else if result.BundleID != "" {
					createDesc = fmt.Sprintf("Create app with %s", result.BundleID)
				}
				options = append(options,
					terminal.PickerOption{Label: "Create new", Desc: createDesc},
				)

				for _, app := range apps {
					options = append(options, terminal.PickerOption{
						Label: app.Name,
						Desc:  app.BundleID,
					})
				}
				options = append(options,
					terminal.PickerOption{Label: "Skip", Desc: "Let Claude handle app selection"},
				)

				picked := terminal.Pick("Select your app", options, "")
				if picked == "Skip" || picked == "" {
					return result
				}
				if picked == "Create new" {
					p.createASCApp(ctx, result, projectDir)
					if result.AppID != "" {
						asc.SaveAppID(projectDir, result.AppID)
					}
					return result
				}
				for _, app := range apps {
					if app.Name == picked {
						result.AppID = app.ID
						result.BundleID = app.BundleID
						asc.SaveAppID(projectDir, app.ID)
						return result
					}
				}
			}
		} else {
			log.Printf("[asc] could not parse app list response")
		}
	}

	return result
}

// createASCApp registers a bundle ID and creates an app in App Store Connect.
// This is a guarded flow: each step must succeed before proceeding.
// On name collision, the user is prompted to choose a different name.
func (p *Pipeline) createASCApp(ctx context.Context, preflight *asc.PreflightResult, projectDir string) {
	log.Printf("[asc] creating new app: bundleID=%s appName=%q", preflight.BundleID, preflight.AppName)
	bundleID := preflight.BundleID
	if bundleID == "" {
		bundleID = strings.TrimSpace(pipelineReadLineFn("Bundle ID (e.g. com.example.myapp)"))
		if bundleID == "" {
			terminal.Error("Bundle ID is required to create an app.")
			return
		}
	}

	appName := preflight.AppName
	if appName == "" {
		appName = strings.TrimSpace(pipelineReadLineFn("App name"))
		if appName == "" {
			terminal.Error("App name is required.")
			return
		}
	}

	// GATE 1: Ensure bundle ID is registered
	bundleIDResourceID := p.ensureBundleID(ctx, bundleID, appName)
	if bundleIDResourceID == "" {
		return // error already displayed
	}

	// GATE 2: Ensure iris session is available (required for app creation)
	jar, err := appleauth.LoadIrisCookies()
	if err != nil {
		log.Printf("[asc] no iris session for app creation: %v", err)
		terminal.Error("App creation requires an Apple ID session.")
		terminal.Info("Sign out and sign back in with Apple ID to create a session.")
		return
	}

	// GATE 3: Create the app (with retry on name collision)
	for {
		spinner := terminal.NewSpinner(fmt.Sprintf("Creating app %q in App Store Connect...", appName))
		spinner.Start()

		appID, createErr := asc.CreateAppViaIris(ctx, jar, appName, bundleID, bundleIDResourceID)
		if createErr == nil {
			preflight.AppID = appID
			preflight.BundleID = bundleID
			preflight.AppName = appName
			log.Printf("[asc] app created: id=%s name=%q bundleID=%s", appID, appName, bundleID)
			spinner.StopWithMessage(fmt.Sprintf("  %s%s✓%s App created: %s (%s)", terminal.Bold, terminal.Green, terminal.Reset, appName, bundleID))
			return
		}

		log.Printf("[asc] app creation failed: %v", createErr)
		errMsg := createErr.Error()

		// Check if it's a name collision — offer rename
		if strings.Contains(errMsg, "already being used") || strings.Contains(errMsg, "name") {
			spinner.StopWithMessage(fmt.Sprintf("  %s%s✗%s App name %q is already taken", terminal.Bold, terminal.Red, terminal.Reset, appName))
			fmt.Println()
			terminal.Warning(errMsg)
			fmt.Println()

			newName := strings.TrimSpace(pipelineReadLineFn("Enter a different app name (or leave empty to cancel)"))
			if newName == "" {
				terminal.Info("App creation cancelled.")
				return
			}
			appName = newName
			continue // retry with new name
		}

		// Check if session expired
		if strings.Contains(errMsg, "session expired") || strings.Contains(errMsg, "401") || strings.Contains(errMsg, "403") {
			spinner.StopWithMessage(fmt.Sprintf("  %s%s✗%s Apple ID session expired", terminal.Bold, terminal.Red, terminal.Reset))
			terminal.Error("Your Apple ID session has expired. Sign out and sign back in with Apple ID.")
			return
		}

		// Other error — non-recoverable
		spinner.StopWithMessage(fmt.Sprintf("  %s%s✗%s App creation failed", terminal.Bold, terminal.Red, terminal.Reset))
		terminal.Error(errMsg)
		return
	}
}

// ensureBundleID registers a bundle ID or finds an existing one. Returns the resource ID or empty on failure.
func (p *Pipeline) ensureBundleID(ctx context.Context, bundleID, appName string) string {
	spinner := terminal.NewSpinner(fmt.Sprintf("Registering bundle ID %s...", bundleID))
	spinner.Start()

	cmd := exec.CommandContext(ctx, "asc", "bundle-ids", "create",
		"--identifier", bundleID,
		"--name", appName,
		"--platform", "IOS",
		"--output", "json",
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		var env struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if json.Unmarshal(output, &env) == nil && env.Data.ID != "" {
			log.Printf("[asc] bundle ID registered: %s resourceID=%s", bundleID, env.Data.ID)
			spinner.StopWithMessage(fmt.Sprintf("  %s%s✓%s Bundle ID registered: %s", terminal.Bold, terminal.Green, terminal.Reset, bundleID))
			return env.Data.ID
		}
	}

	// Bundle ID may already exist — look it up
	log.Printf("[asc] bundle-ids create failed, looking up existing: %v", err)
	spinner.StopWithMessage(fmt.Sprintf("  %s%s—%s Checking existing bundle IDs...", terminal.Bold, terminal.Yellow, terminal.Reset))

	lookupCmd := exec.CommandContext(ctx, "asc", "bundle-ids", "list", "--output", "json")
	lookupOut, lookupErr := lookupCmd.CombinedOutput()
	if lookupErr != nil {
		terminal.Error("Failed to list bundle IDs.")
		return ""
	}

	resourceID := asc.FindBundleIDResource(lookupOut, bundleID)
	if resourceID != "" {
		log.Printf("[asc] found existing bundle ID resource: %s", resourceID)
		terminal.Success(fmt.Sprintf("Bundle ID %s already registered", bundleID))
		return resourceID
	}

	terminal.Error(fmt.Sprintf("Bundle ID %s could not be registered or found.", bundleID))
	return ""
}

// ComposeASCSystemPrompt builds the system prompt for ASC operations
// by composing from embedded skill files. Only the core role, safety rules,
// and pre-flight context are inline — all workflow knowledge lives in skills.
func (p *Pipeline) ComposeASCSystemPrompt(projectDir string, preflight *asc.PreflightResult) string {
	log.Printf("[asc] composing system prompt: projectDir=%s", projectDir)
	var sb strings.Builder

	// Core role, tools, and safety rules — minimal inline, everything else from skills
	sb.WriteString(`You are an App Store Connect assistant. You help users manage their app's presence on the App Store, TestFlight, and related services.

## Tools

Use **Bash** to run all ` + "`asc`" + ` CLI commands. The asc CLI is already installed and authenticated.
Use Read/Write/Edit for file operations (ExportOptions.plist, build settings, etc.).
Run ` + "`asc --help`" + ` or ` + "`asc <command> --help`" + ` to discover available commands and flags.

## Safety Rules

Before ANY destructive or externally-visible operation, you MUST:
1. Explain exactly what you are about to do
2. List any irreversible consequences
3. Ask the user to confirm before proceeding

Destructive operations that require confirmation:
- ` + "`asc submit`" + ` — submits the app for App Store review
- ` + "`asc publish`" + ` — releases the app to the App Store
- ` + "`asc publish testflight`" + ` — distributes a build to TestFlight testers
- ` + "`asc bundle-ids create`" + ` — registers a new bundle ID with Apple
- ` + "`asc metadata push`" + ` — pushes metadata changes to App Store Connect
- ` + "`asc age-rating set`" + ` — updates the app's age rating
- ` + "`asc versions create`" + ` — creates a new app version

For read-only operations (` + "`asc apps list`" + `, ` + "`asc status`" + `, ` + "`asc builds list`" + `, ` + "`asc doctor`" + `, etc.), proceed directly without confirmation.

## TestFlight Beta Testing

When adding beta testers, **default to external testing** — it works for any email address.

### Flow:
1. Create an external group: ` + "`asc testflight beta-groups create --app APP_ID --name \"Beta Testers\"`" + `
2. Assign build to the group: ` + "`asc builds add-groups --build BUILD_ID --group GROUP_ID`" + `
3. Add tester to the group: ` + "`asc testflight beta-testers add --app APP_ID --email EMAIL --group \"Beta Testers\"`" + `
4. **STOP and ask the user** before submitting for Beta App Review. Explain that Apple needs to review the build (~24h) before the tester can install.
5. Only after the user confirms: ` + "`asc testflight review submit --build BUILD_ID --confirm`" + `

### Important:
- Do NOT call ` + "`beta-testers invite`" + ` for external testers — it fails before review approval. Testers are auto-invited once approved.
- Do NOT use ` + "`builds add-groups`" + ` for internal groups — they auto-receive builds.
- Internal testing is only for ASC team members. Use only when the user explicitly asks for it.
`)

	// Pre-flight context
	if preflight != nil && (preflight.AppName != "" || preflight.BundleID != "" || preflight.AppID != "") {
		sb.WriteString("\n## Pre-Flight Context\n\n")
		if preflight.AppName != "" {
			sb.WriteString(fmt.Sprintf("- App name: %s\n", preflight.AppName))
		}
		if preflight.BundleID != "" {
			sb.WriteString(fmt.Sprintf("- Bundle ID: %s\n", preflight.BundleID))
		}
		if preflight.AppID != "" {
			sb.WriteString(fmt.Sprintf("- ASC App ID: %s (already matched)\n", preflight.AppID))
		}
		if len(preflight.Localizations) > 0 {
			sb.WriteString(fmt.Sprintf("- Localizations: %s\n", strings.Join(preflight.Localizations, ", ")))
		}
	} else {
		// Fallback: read project config directly
		configPath := filepath.Join(projectDir, "project_config.json")
		if data, err := os.ReadFile(configPath); err == nil {
			var cfg struct {
				AppName  string `json:"app_name"`
				BundleID string `json:"bundle_id"`
				Platform string `json:"platform"`
			}
			if json.Unmarshal(data, &cfg) == nil && cfg.AppName != "" {
				sb.WriteString("\n## Project Context\n\n")
				sb.WriteString(fmt.Sprintf("- App name: %s\n", cfg.AppName))
				if cfg.BundleID != "" {
					sb.WriteString(fmt.Sprintf("- Bundle ID: %s\n", cfg.BundleID))
				}
				if cfg.Platform != "" {
					sb.WriteString(fmt.Sprintf("- Platform: %s\n", cfg.Platform))
				}
			}
		}
	}

	// Load skills — official ASC CLI skills from github.com/rudrankriyam/app-store-connect-cli-skills
	// plus nanowave-specific skills (asset-management, asc-publish)
	ascSkills := []struct {
		dir   string
		label string
	}{
		// Official ASC CLI skills
		{"skills/features/asc-cli-usage", "CLI Usage"},
		{"skills/features/asc-xcode-build", "Xcode Build and Export"},
		{"skills/features/asc-build-lifecycle", "Build Lifecycle"},
		{"skills/features/asc-signing-setup", "Signing Setup"},
		{"skills/features/asc-testflight-orchestration", "Release and TestFlight Orchestration"},
		{"skills/features/asc-id-resolver", "ID Resolution"},
		{"skills/features/asc-submission-health", "Submission Health"},
		{"skills/features/asc-metadata-sync", "Metadata Sync and Localization"},
		{"skills/features/asc-shots-pipeline", "Screenshot Pipeline"},
		{"skills/features/asc-app-create-ui", "App Creation"},
		{"skills/features/asc-crash-triage", "Crash Triage"},
		{"skills/features/asc-notarization", "macOS Notarization"},
		{"skills/features/asc-ppp-pricing", "PPP Pricing"},
		{"skills/features/asc-revenuecat-catalog-sync", "RevenueCat Catalog Sync"},
		{"skills/features/asc-subscription-localization", "Subscription Localization"},
		{"skills/features/asc-wall-submit", "Wall of Apps"},
		{"skills/features/asc-workflow", "Workflow Automation"},
		// Nanowave-specific skills
		{"skills/features/asc-publish", "App Store Publishing"},
		{"skills/features/asset-management", "Asset Management"},
	}
	loadedCount := 0
	for _, skill := range ascSkills {
		if content := readEmbeddedMarkdownDirBodies(skill.dir); content != "" {
			appendPromptSection(&sb, skill.label, content)
			loadedCount++
			log.Printf("[asc] loaded skill: %s (%d chars)", skill.label, len(content))
		} else {
			log.Printf("[asc] skill not found or empty: %s (dir=%s)", skill.label, skill.dir)
		}
	}
	log.Printf("[asc] system prompt composed: %d/%d skills loaded, total=%d chars", loadedCount, len(ascSkills), sb.Len())

	// Final reminder — placed at end for recency bias (models attend most to start + end)
	sb.WriteString(`
## MANDATORY: Confirmation Before Beta App Review

Before submitting a build for Beta App Review, you MUST:
1. Explain to the user that Apple needs to review the build (typically < 24 hours).
2. Ask the user to confirm before running ` + "`asc testflight review submit`" + `.
3. Do NOT call ` + "`beta-testers invite`" + ` for external testers — it fails before review approval.
`)

	return sb.String()
}

// lastLines returns the last n lines of a string.
func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
