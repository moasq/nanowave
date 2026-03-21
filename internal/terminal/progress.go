package terminal

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// Phase represents the current build phase.
type Phase int

const (
	PhaseAnalyzing Phase = iota
	PhasePlanning
	PhaseBuildingCode
	PhaseGenerating
	PhaseCompiling
	PhaseFixing
	PhaseEditing
	PhaseASC
)

func (p Phase) label() string {
	switch p {
	case PhaseAnalyzing:
		return "Analyzing request"
	case PhasePlanning:
		return "Planning architecture"
	case PhaseBuildingCode:
		return "Building code"
	case PhaseGenerating:
		return "Generating Xcode project"
	case PhaseCompiling:
		return "Compiling"
	case PhaseFixing:
		return "Fixing errors"
	case PhaseEditing:
		return "Editing"
	case PhaseASC:
		return "Running"
	default:
		return "Working"
	}
}

// activity represents a single logged action (kept for non-interactive fallback).
type activity struct {
	text string
	done bool
}

// ProgressDisplay provides a streaming log display for agent activity.
//
// Design: agent messages and tool calls are printed as permanent log lines.
// Only the current spinner line at the bottom is redrawn in place.
// This means the user sees the full stream of what the agent is doing,
// not a rigid summary that hides the real work.
type ProgressDisplay struct {
	mu         sync.Mutex
	phase      Phase
	totalFiles int
	filesWritten int
	statusText   string          // current spinner status (redrawn in place)
	streamingBuf strings.Builder // accumulates streaming text tokens
	running      bool
	done         chan struct{}
	stopped      chan struct{}
	mode         string
	totalPhases  int
	buildFailed  bool
	fixAttempts  int
	startedAt    time.Time
	interactive  bool

	// Streaming log state
	spinnerDirty      bool   // whether the spinner line needs clearing before next print
	lastLogLine       string // deduplication: last printed log line
	lastToolVerb      string // verb of the last tool call (for collapsing)
	lastToolVerbCount int    // how many consecutive calls share this verb

	// Non-interactive fallback
	lastRenderID    string
	lastRenderLines int
	activities      []activity
	maxActivities   int
}

const (
	defaultMaxActivities = 4
	maxStatusWidth       = 100
)

// NewProgressDisplay creates a progress display for the given mode.
func NewProgressDisplay(mode string, _ int) *ProgressDisplay {
	startPhase := PhaseBuildingCode
	switch mode {
	case "analyze":
		startPhase = PhaseAnalyzing
	case "plan":
		startPhase = PhasePlanning
	case "build", "agentic":
		startPhase = PhaseBuildingCode
	case "edit":
		startPhase = PhaseEditing
	case "fix":
		startPhase = PhaseCompiling
	case "asc":
		startPhase = PhaseASC
	}

	maxAct := defaultMaxActivities
	if mode == "asc" || mode == "agentic" {
		maxAct = 8
	}

	return &ProgressDisplay{
		phase:         startPhase,
		mode:          mode,
		startedAt:     time.Now(),
		interactive:   term.IsTerminal(int(os.Stdout.Fd())),
		done:          make(chan struct{}),
		stopped:       make(chan struct{}),
		maxActivities: maxAct,
	}
}

// Start begins the spinner rendering loop.
func (pd *ProgressDisplay) Start() {
	pd.mu.Lock()
	if pd.running {
		pd.mu.Unlock()
		return
	}
	pd.running = true
	pd.mu.Unlock()
	go pd.spinnerLoop()
}

// Stop stops the display and clears the spinner.
func (pd *ProgressDisplay) Stop() {
	pd.mu.Lock()
	if !pd.running {
		pd.mu.Unlock()
		return
	}
	pd.running = false
	pd.mu.Unlock()

	close(pd.done)
	<-pd.stopped
	if pd.interactive {
		// Clear the spinner line
		fmt.Printf("\r\033[K")
	}
}

// StopWithSuccess stops and prints a success message with trailing spacing.
func (pd *ProgressDisplay) StopWithSuccess(msg string) {
	pd.Stop()
	fmt.Printf("\n  %s%s✓%s %s\n\n", Bold, Green, Reset, msg)
}

// StopWithError stops and prints an error message with trailing spacing.
func (pd *ProgressDisplay) StopWithError(msg string) {
	pd.Stop()
	fmt.Printf("\n  %s%s✗%s %s\n\n", Bold, Red, Reset, msg)
}

// SetPhase explicitly transitions to a new phase.
func (pd *ProgressDisplay) SetPhase(phase Phase) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.phase = phase
}

// AddActivity prints a permanent log line for a nanowave-internal event.
func (pd *ProgressDisplay) AddActivity(text string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.addActivity(text)
	if pd.interactive {
		pd.printLogLine(fmt.Sprintf("  %s●%s %s", Dim, Reset, text))
	}
}

func (pd *ProgressDisplay) addActivity(text string) {
	if len(pd.activities) > 0 {
		pd.activities[len(pd.activities)-1].done = true
	}
	pd.activities = append(pd.activities, activity{text: text, done: false})
	if len(pd.activities) > pd.maxActivities {
		pd.activities = pd.activities[len(pd.activities)-pd.maxActivities:]
	}
}

// UpdateLastActivity updates the text of the most recent activity.
func (pd *ProgressDisplay) UpdateLastActivity(text string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	if len(pd.activities) > 0 && !pd.activities[len(pd.activities)-1].done {
		pd.activities[len(pd.activities)-1].text = text
	} else {
		pd.addActivity(text)
	}
}

// SetStatus sets the spinner status text.
func (pd *ProgressDisplay) SetStatus(text string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.statusText = text
}

// IncrementFiles increments the files written counter.
func (pd *ProgressDisplay) IncrementFiles() {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.filesWritten++
	if pd.filesWritten > pd.totalFiles && pd.totalFiles > 0 {
		pd.totalFiles = pd.filesWritten
	}
}

// ResetForRetry resets transient display state for a new completion pass.
func (pd *ProgressDisplay) ResetForRetry() {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.activities = nil
	pd.statusText = ""
	pd.phase = PhaseBuildingCode
	pd.buildFailed = false
	pd.fixAttempts = 0
}

// SetTotalFiles updates the total expected file count.
func (pd *ProgressDisplay) SetTotalFiles(total int) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.totalFiles = total
	if pd.totalFiles < pd.filesWritten {
		pd.totalFiles = pd.filesWritten
	}
}

// ---------------------------------------------------------------------------
// Agent event handlers — these are the primary display drivers
// ---------------------------------------------------------------------------

// OnToolUse processes a tool call event.
// Consecutive same-verb calls (e.g. "Reading X", "Reading Y") are collapsed:
// only the first is printed as a log line; subsequent ones just update the spinner.
// When the verb changes or agent text arrives, a summary is flushed.
func (pd *ProgressDisplay) OnToolUse(toolName string, inputGetter func(key string) string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.streamingBuf.Reset()

	label := pd.toolActivityLabel(toolName, inputGetter)
	if label == "" {
		return
	}
	pd.addActivity(label)

	if !pd.interactive {
		return
	}

	verb := toolVerb(label)

	if verb != "" && verb == pd.lastToolVerb {
		// Same verb — don't print a new line. Just count and update spinner.
		pd.lastToolVerbCount++
		pd.statusText = fmt.Sprintf("%s (%d files)", verb, pd.lastToolVerbCount)
		return
	}

	// Different verb — flush previous collapsed run, then print this one
	pd.flushToolCollapse()
	pd.lastToolVerb = verb
	pd.lastToolVerbCount = 1
	pd.printLogLine(fmt.Sprintf("  %s↳%s %s%s%s", Dim, Reset, Dim, label, Reset))
}

// UpdateToolActivity refines the spinner status for a tool in progress.
// Unlike OnToolUse, this does NOT print a new log line — it just updates
// the spinner so the user sees what's happening in real time.
func (pd *ProgressDisplay) UpdateToolActivity(toolName string, inputGetter func(key string) string, _ bool) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	label := pd.toolActivityLabel(toolName, inputGetter)
	if label == "" {
		return
	}

	// Update both the activity list and the spinner status
	if len(pd.activities) > 0 && !pd.activities[len(pd.activities)-1].done {
		pd.activities[len(pd.activities)-1].text = label
	}
	pd.statusText = label
}

// OnStreamingText processes token-by-token text deltas.
// Updates the spinner status with a preview — does NOT print raw tokens,
// because the stream often contains tool output (xcodebuild stderr, etc.)
// that would flood the display. Full agent messages are printed by OnAgentMessage.
func (pd *ProgressDisplay) OnStreamingText(text string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.streamingBuf.WriteString(text)

	accumulated := pd.streamingBuf.String()
	status := extractStreamingPreview(accumulated, pd.mode)
	if status != "" {
		pd.statusText = status
	}
}

// OnAssistantText processes full assistant messages.
func (pd *ProgressDisplay) OnAssistantText(text string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.streamingBuf.Reset()

	if isStructuredMode(pd.mode) && strings.TrimSpace(text) != "" {
		pd.statusText = structuredModeLabel(pd.mode)
		return
	}

	status := extractStatus(text)
	if status != "" && !looksLikeCode(status) {
		pd.statusText = status
	}
}

// OnAgentMessage processes agent text and prints it as permanent log lines.
// The full markdown-rendered text is shown so the user sees the complete response.
func (pd *ProgressDisplay) OnAgentMessage(text string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	// Flush any pending tool collapse — agent speaking ends tool runs
	pd.flushToolCollapse()

	text = strings.TrimSpace(text)
	if text == "" {
		pd.streamingBuf.Reset()
		return
	}

	if isStructuredMode(pd.mode) {
		pd.streamingBuf.Reset()
		pd.statusText = structuredModeLabel(pd.mode)
		return
	}

	label := extractStatus(text)
	if label != "" && !looksLikeCode(label) {
		label = truncateActivity(label)
		if len(pd.activities) == 0 || pd.activities[len(pd.activities)-1].text != label {
			pd.addActivity(label)
		}
	}

	pd.streamingBuf.Reset()

	if pd.interactive {
		pd.printAgentText(text)
	} else if label != "" && !looksLikeCode(label) {
		pd.statusText = label
	}
}

// OnAgentCommentary records agent commentary as a log line.
func (pd *ProgressDisplay) OnAgentCommentary(text string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	label := truncateActivity(extractStatus(text))
	if label == "" {
		return
	}
	if len(pd.activities) > 0 && pd.activities[len(pd.activities)-1].text == label {
		return
	}

	pd.statusText = ""
	pd.streamingBuf.Reset()
	pd.addActivity(label)

	if pd.interactive {
		pd.printAgentText(text)
	}
}

// OnRuntimeLog prints a runtime stderr/stdout line when runtime log streaming is enabled.
func (pd *ProgressDisplay) OnRuntimeLog(text string) {
	if !runtimeLogsEnabled() {
		return
	}

	pd.mu.Lock()
	defer pd.mu.Unlock()

	text = strings.TrimSpace(stripAnsi(text))
	if text == "" {
		return
	}

	pd.flushToolCollapse()
	pd.streamingBuf.Reset()
	pd.statusText = ""

	if pd.interactive {
		pd.printLogLine(fmt.Sprintf("  %s[runtime]%s %s", Dim, Reset, text))
		return
	}

	pd.addActivity("Runtime log: " + truncateActivity(text))
}


// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

// printLogLine clears the spinner, prints a permanent line, and marks spinner dirty.
// MUST be called with pd.mu held.
func (pd *ProgressDisplay) printLogLine(line string) {
	if line == pd.lastLogLine {
		return // deduplicate
	}
	pd.lastLogLine = line


	if pd.spinnerDirty {
		fmt.Printf("\r\033[K") // clear the spinner line
	}
	fmt.Printf("%s\n", line)
	pd.spinnerDirty = false
}

// flushToolCollapse prints a summary line if there was a collapsed tool run
// (e.g. "↳ Reading (22 files)"). MUST be called with pd.mu held.
func (pd *ProgressDisplay) flushToolCollapse() {
	if pd.lastToolVerbCount > 1 && pd.lastToolVerb != "" {
		summary := fmt.Sprintf("  %s↳%s %s%s (%d files)%s", Dim, Reset, Dim, pd.lastToolVerb, pd.lastToolVerbCount, Reset)
		pd.doPrintLine(summary)
	}
	pd.lastToolVerb = ""
	pd.lastToolVerbCount = 0
}

// printAgentText renders agent text as permanent log lines with markdown formatting.
// Long runs of dimmed lines (tool output, code) are collapsed for readability.
// Deduplicates against lastLogLine to avoid reprinting spinner preview text.
// MUST be called with pd.mu held.
func (pd *ProgressDisplay) printAgentText(text string) {
	pd.flushToolCollapse()
	rendered := RenderMarkdown(text)
	lines := strings.Split(rendered, "\n")

	const maxDimRun = 3
	var printed bool
	dimRunCount := 0

	for _, line := range lines {
		raw := strings.TrimSpace(stripAnsi(line))
		if raw == "" {
			dimRunCount = 0
			continue
		}

		isDim := strings.Contains(line, Dim)

		if isDim {
			dimRunCount++
			if dimRunCount > maxDimRun {
				continue
			}
		} else {
			if dimRunCount > maxDimRun {
				hidden := dimRunCount - maxDimRun
				pd.doPrintLine(fmt.Sprintf("  %s... %d lines collapsed%s", Dim, hidden, Reset))
				printed = true
			}
			dimRunCount = 0
		}

		if line == pd.lastLogLine {
			continue
		}
		pd.lastLogLine = line
		pd.doPrintLine(line)
		printed = true
	}

	if dimRunCount > maxDimRun {
		hidden := dimRunCount - maxDimRun
		pd.doPrintLine(fmt.Sprintf("  %s... %d lines collapsed%s", Dim, hidden, Reset))
		printed = true
	}

	if printed {
		pd.statusText = ""
	}
}

// doPrintLine prints a single line, clearing spinner if needed.
// MUST be called with pd.mu held.
func (pd *ProgressDisplay) doPrintLine(line string) {
	if pd.spinnerDirty {
		fmt.Printf("\r\033[K")
		pd.spinnerDirty = false
	}
	fmt.Printf("%s\n", line)
}

// stripAnsi removes ANSI escape codes for length/emptiness checks.
func stripAnsi(s string) string {
	// Simple ANSI stripper: remove ESC[...m sequences
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until 'm'
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				i = j + 1
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// spinnerLoop redraws the spinner line in place.
func (pd *ProgressDisplay) spinnerLoop() {
	defer close(pd.stopped)
	frame := 0
	for {
		select {
		case <-pd.done:
			return
		default:
			pd.renderSpinner(frame)
			frame++
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// renderSpinner draws a single spinner line at the current cursor position.
func (pd *ProgressDisplay) renderSpinner(frame int) {
	pd.mu.Lock()
	statusText := pd.statusText
	elapsed := time.Since(pd.startedAt)
	interactive := pd.interactive
	pd.mu.Unlock()

	if !interactive {
		pd.renderNonInteractive()
		return
	}

	spinChar := spinnerFrames[frame%len(spinnerFrames)]

	var sb strings.Builder
	sb.WriteString("  ")
	sb.WriteString(fmt.Sprintf("%s%s%s", Cyan, spinChar, Reset))
	sb.WriteString(fmt.Sprintf("  %s%s%s", Dim, formatElapsed(elapsed), Reset))

	if statusText != "" {
		sb.WriteString(fmt.Sprintf("  %s", statusText))
	}

	line := sb.String()

	fmt.Printf("\r\033[K%s", line)

	pd.mu.Lock()
	pd.spinnerDirty = true
	pd.mu.Unlock()
}

func (pd *ProgressDisplay) renderNonInteractive() {
	pd.mu.Lock()
	activities := make([]activity, len(pd.activities))
	copy(activities, pd.activities)
	statusText := pd.statusText
	pd.mu.Unlock()

	latestActivity := ""
	if len(activities) > 0 {
		act := activities[len(activities)-1]
		if act.done {
			latestActivity = fmt.Sprintf("  ✓ %s", act.text)
		} else {
			latestActivity = fmt.Sprintf("  • %s", act.text)
		}
	}

	parts := []string{latestActivity, statusText}
	renderID := strings.Join(parts, "\n")

	pd.mu.Lock()
	if renderID == pd.lastRenderID {
		pd.mu.Unlock()
		return
	}
	pd.lastRenderID = renderID
	pd.mu.Unlock()

	if latestActivity != "" {
		fmt.Println(latestActivity)
	}
	if statusText != "" {
		fmt.Println("  " + statusText)
	}
}

func formatElapsed(d time.Duration) string {
	s := int(d.Seconds())
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	m := s / 60
	s = s % 60
	return fmt.Sprintf("%dm%02ds", m, s)
}

// ---------------------------------------------------------------------------
// Tool label generation (unchanged)
// ---------------------------------------------------------------------------

func (pd *ProgressDisplay) toolActivityLabel(toolName string, inputGetter func(key string) string) string {
	switch toolName {
	case "Write":
		path := inputGetter("file_path")
		if path != "" {
			return fmt.Sprintf("Writing %s", sanitizeActivityLabel(shortPath(path)))
		}
		return "Writing file"
	case "Edit":
		path := inputGetter("file_path")
		if path != "" {
			return fmt.Sprintf("Editing %s", sanitizeActivityLabel(shortPath(path)))
		}
		return "Editing file"
	case "Read":
		path := inputGetter("file_path")
		if path != "" {
			return fmt.Sprintf("Reading %s", sanitizeActivityLabel(shortPath(path)))
		}
		return "Reading file"
	case "Bash":
		command := unwrapShellCommand(inputGetter("command"))
		if strings.Contains(command, "xcodegen") {
			return "Generating Xcode project"
		} else if strings.Contains(command, "xcodebuild") {
			return "Compiling project"
		} else if strings.Contains(command, "git init") || strings.Contains(command, "git add") || strings.Contains(command, "git commit") {
			return "Updating repository"
		} else if label := ascCommandLabel(command); label != "" {
			return label
		} else if command != "" {
			return truncateActivity(friendlyBashLabel(command))
		}
		return "Running command"
	case "Glob":
		pattern := inputGetter("pattern")
		if pattern != "" {
			return fmt.Sprintf("Searching for %s", sanitizeActivityLabel(shortPath(pattern)))
		}
		return "Searching files"
	case "Grep":
		pattern := inputGetter("pattern")
		if pattern != "" {
			return fmt.Sprintf("Searching for %s", sanitizeActivityLabel(truncateActivity(pattern)))
		}
		return "Searching code"
	case "WebFetch", "WebSearch":
		return "Searching web"
	case "TodoWrite":
		return "Updating task list"
	default:
		if label := friendlyToolName(toolName, inputGetter); label != "" {
			return label
		}
		return ""
	}
}

// ---------------------------------------------------------------------------
// Utility functions (unchanged)
// ---------------------------------------------------------------------------

func shortPath(fullPath string) string {
	parts := strings.Split(fullPath, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	if len(parts) >= 1 {
		return parts[len(parts)-1]
	}
	return fullPath
}

func friendlyToolName(toolName string, inputGetter func(key string) string) string {
	switch toolName {
	case "mcp__apple-docs__search_apple_docs":
		if q := inputGetter("query"); q != "" {
			return truncateActivity("Researching " + q)
		}
		return "Researching Apple docs"
	case "mcp__apple-docs__get_apple_doc_content":
		return "Reading documentation"
	case "mcp__apple-docs__search_framework_symbols":
		if fw := inputGetter("framework"); fw != "" {
			return truncateActivity("Looking up " + fw + " symbols")
		}
		return "Looking up framework symbols"
	case "mcp__apple-docs__get_sample_code":
		return "Reading sample code"
	case "mcp__apple-docs__get_related_apis":
		return "Finding related APIs"
	case "mcp__apple-docs__find_similar_apis":
		return "Finding similar APIs"
	case "mcp__apple-docs__get_platform_compatibility":
		return "Checking platform compatibility"
	case "mcp__xcodegen__add_permission":
		if key := inputGetter("key"); key != "" {
			return truncateActivity("Adding permission: " + key)
		}
		return "Adding permission"
	case "mcp__xcodegen__add_extension":
		if kind := inputGetter("kind"); kind != "" {
			return truncateActivity("Adding " + kind + " extension")
		}
		return "Adding extension"
	case "mcp__xcodegen__add_entitlement":
		return "Adding entitlement"
	case "mcp__xcodegen__add_localization":
		if lang := inputGetter("language"); lang != "" {
			return truncateActivity("Adding " + lang + " localization")
		}
		return "Adding localization"
	case "mcp__xcodegen__set_build_setting":
		return "Updating build settings"
	case "mcp__xcodegen__get_project_config":
		return "Reading project config"
	case "mcp__xcodegen__regenerate_project":
		return "Regenerating Xcode project"
	}
	return ""
}

func unwrapShellCommand(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	for _, prefix := range []string{
		"/bin/zsh -lc ", "/bin/zsh -c ",
		"/bin/bash -lc ", "/bin/bash -c ",
		"/bin/sh -lc ", "/bin/sh -c ",
		"bash -lc ", "bash -c ",
		"sh -lc ", "sh -c ",
		"zsh -lc ", "zsh -c ",
	} {
		if strings.HasPrefix(command, prefix) {
			inner := strings.TrimSpace(command[len(prefix):])
			if len(inner) >= 2 {
				if (inner[0] == '\'' && inner[len(inner)-1] == '\'') ||
					(inner[0] == '"' && inner[len(inner)-1] == '"') {
					inner = inner[1 : len(inner)-1]
				}
			}
			return unwrapShellCommand(strings.TrimSpace(inner))
		}
	}
	return command
}

func friendlyBashLabel(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return "Running command"
	}
	first := command
	for _, sep := range []string{" | ", " && ", " ; "} {
		if idx := strings.Index(first, sep); idx > 0 {
			first = first[:idx]
		}
	}
	fields := strings.Fields(first)
	if len(fields) == 0 {
		return "Running command"
	}
	bin := fields[0]
	if idx := strings.LastIndex(bin, "/"); idx >= 0 {
		bin = bin[idx+1:]
	}
	filePath := extractFileArg(fields)
	switch bin {
	case "cat", "nl", "sed", "head", "tail", "awk", "less", "more", "wc":
		if filePath != "" {
			return "Reading " + shortPath(filePath)
		}
		return "Reading file"
	case "grep", "rg", "ag":
		if filePath != "" {
			return "Searching " + shortPath(filePath)
		}
		return "Searching code"
	case "ls", "find", "fd", "tree":
		if filePath != "" {
			return "Listing " + shortPath(filePath)
		}
		return "Listing files"
	case "mkdir":
		if filePath != "" {
			return "Creating " + shortPath(filePath)
		}
		return "Creating directory"
	case "rm":
		return "Removing files"
	case "cp":
		return "Copying files"
	case "mv":
		return "Moving files"
	case "touch":
		if filePath != "" {
			return "Creating " + shortPath(filePath)
		}
		return "Creating file"
	case "chmod":
		return "Setting permissions"
	case "curl", "wget":
		return "Downloading"
	case "jq":
		return "Processing JSON"
	case "swift":
		return "Running Swift"
	case "pod":
		return "Running CocoaPods"
	case "echo", "printf":
		return "Running command"
	default:
		return bin
	}
}

func extractFileArg(fields []string) string {
	for i := len(fields) - 1; i >= 1; i-- {
		arg := fields[i]
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if strings.HasPrefix(arg, "'") || strings.HasPrefix(arg, "\"") {
			continue
		}
		if strings.Contains(arg, "/") || strings.Contains(arg, ".") {
			return arg
		}
	}
	return ""
}

func ascCommandLabel(command string) string {
	command = strings.TrimSpace(command)
	if !strings.HasPrefix(command, "asc ") && !strings.HasPrefix(command, "asc\t") {
		return ""
	}
	parts := strings.Fields(command)
	if len(parts) < 2 {
		return "Running asc"
	}
	var tokens []string
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "-") {
			break
		}
		tokens = append(tokens, p)
		if len(tokens) >= 4 {
			break
		}
	}
	if len(tokens) == 0 {
		return "Running asc"
	}
	action := tokens[len(tokens)-1]
	var context string
	if len(tokens) >= 3 {
		context = humanizeToken(tokens[len(tokens)-2])
	} else if len(tokens) == 2 {
		context = humanizeToken(tokens[0])
	}
	verb := humanizeVerb(action)
	if context != "" {
		return truncateActivity(verb + " " + context)
	}
	return truncateActivity(verb)
}

func humanizeVerb(action string) string {
	action = strings.ToLower(action)
	switch action {
	case "list":
		return "Listing"
	case "get", "info", "status":
		return "Checking"
	case "create", "register":
		return "Creating"
	case "add", "assign":
		return "Adding"
	case "remove", "delete":
		return "Removing"
	case "update", "set", "push", "upload":
		return "Updating"
	case "submit":
		return "Submitting"
	case "publish":
		return "Publishing"
	case "invite":
		return "Inviting"
	case "pull", "download":
		return "Downloading"
	case "cancel":
		return "Cancelling"
	case "attach-build":
		return "Attaching build"
	case "add-groups":
		return "Assigning to group"
	case "latest":
		return "Checking latest"
	case "login":
		return "Authenticating"
	case "doctor":
		return "Running diagnostics"
	case "help", "--help":
		return "Checking help"
	default:
		return "Running " + humanizeToken(action)
	}
}

func humanizeToken(token string) string {
	s := strings.ReplaceAll(token, "-", " ")
	s = strings.ReplaceAll(s, " ids", " IDs")
	s = strings.ReplaceAll(s, " id", " ID")
	return s
}

func truncateActivity(s string) string {
	s = sanitizeActivityLabel(s)
	const maxWidth = 80
	if len(s) > maxWidth {
		return s[:maxWidth] + "..."
	}
	return s
}

func sanitizeActivityLabel(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\\`, `\`)
	s = strings.ReplaceAll(s, `"`, "")
	s = strings.TrimLeft(s, "{[")
	s = strings.TrimRight(s, "}]")
	s = strings.TrimSpace(s)
	return s
}

// toolVerb extracts the action verb from a tool label.
// e.g. "Reading Models/Foo.swift" → "Reading", "Compiling project" → "Compiling"
func toolVerb(label string) string {
	if idx := strings.IndexByte(label, ' '); idx > 0 {
		return label[:idx]
	}
	return label
}

func isStructuredMode(mode string) bool {
	return mode == "analyze" || mode == "plan"
}

func structuredModeLabel(mode string) string {
	switch mode {
	case "analyze":
		return "Preparing analysis output..."
	case "plan":
		return "Preparing build plan..."
	default:
		return "Preparing output..."
	}
}

func extractStreamingPreview(text, mode string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	switch mode {
	case "analyze":
		return "Preparing analysis output..."
	case "plan":
		return "Preparing build plan..."
	case "agentic":
		return "Composing response..."
	default:
		return extractLastLine(text)
	}
}

func runtimeLogsEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("NANOWAVE_RUNTIME_LOGS"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func extractStatus(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	for i, ch := range text {
		if ch == '.' || ch == '\n' {
			text = text[:i]
			break
		}
	}
	if len(text) > maxStatusWidth {
		text = text[:maxStatusWidth] + "..."
	}
	return text
}

func extractLastLine(text string) string {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if looksLikeCode(line) {
			continue
		}
		if len(line) > maxStatusWidth {
			line = line[:maxStatusWidth] + "..."
		}
		return line
	}
	return ""
}

func looksLikeCode(line string) bool {
	if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "}") ||
		strings.HasPrefix(line, "\"") || strings.HasPrefix(line, "[") ||
		strings.HasPrefix(line, "]") || strings.HasPrefix(line, "```") {
		return true
	}
	if strings.HasPrefix(line, ".") || strings.HasPrefix(line, "//") ||
		strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "func ") ||
		strings.HasPrefix(line, "struct ") || strings.HasPrefix(line, "class ") ||
		strings.HasPrefix(line, "enum ") || strings.HasPrefix(line, "protocol ") ||
		strings.HasPrefix(line, "var ") || strings.HasPrefix(line, "let ") ||
		strings.HasPrefix(line, "case ") || strings.HasPrefix(line, "return ") ||
		strings.HasPrefix(line, "@") || strings.HasPrefix(line, "#") {
		return true
	}
	codeChars := 0
	for _, ch := range line {
		if ch == '(' || ch == ')' || ch == '{' || ch == '}' || ch == ';' || ch == '=' {
			codeChars++
		}
	}
	if codeChars >= 3 {
		return true
	}
	if len(line) > 0 && (line[0] == '\t' || strings.HasPrefix(line, "    ")) {
		return true
	}
	return false
}
