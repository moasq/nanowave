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
	default:
		return "Working"
	}
}

func (p Phase) number() int {
	switch p {
	case PhaseAnalyzing:
		return 1
	case PhasePlanning:
		return 2
	case PhaseBuildingCode:
		return 3
	case PhaseGenerating:
		return 4
	case PhaseCompiling:
		return 5
	case PhaseFixing:
		return 6
	case PhaseEditing:
		return 1
	default:
		return 1
	}
}

// activity represents a single logged action.
type activity struct {
	text string
	done bool
}

// ProgressDisplay provides a rich, phase-aware terminal progress UI.
type ProgressDisplay struct {
	mu           sync.Mutex
	phase        Phase
	totalFiles   int
	filesWritten int
	activities   []activity
	statusText   string          // dimmed assistant text
	streamingBuf strings.Builder // accumulates streaming text tokens
	running      bool
	done         chan struct{}
	mode         string // "build", "edit", "fix", "analyze", "plan"
	totalPhases  int
	buildFailed  bool
	fixAttempts  int
	startedAt    time.Time
	interactive  bool
	lastRenderID string
}

const (
	maxActivities                = 4
	maxStatusWidth               = 70
	structuredStreamingTailRunes = 240
)

// NewProgressDisplay creates a progress display for the given mode.
// totalFiles is used for the build progress bar (0 if unknown).
func NewProgressDisplay(mode string, totalFiles int) *ProgressDisplay {
	totalPhases := 5 // build: analyze → plan → code → generate → compile
	startPhase := PhaseBuildingCode

	switch mode {
	case "intent":
		startPhase = PhaseAnalyzing
		totalPhases = 5
	case "analyze":
		startPhase = PhaseAnalyzing
		totalPhases = 5
	case "plan":
		startPhase = PhasePlanning
		totalPhases = 5
	case "build":
		startPhase = PhaseBuildingCode
		totalPhases = 5
	case "edit":
		startPhase = PhaseEditing
		totalPhases = 0
	case "fix":
		startPhase = PhaseCompiling
		totalPhases = 0
	}

	return &ProgressDisplay{
		phase:       startPhase,
		totalFiles:  totalFiles,
		mode:        mode,
		totalPhases: totalPhases,
		startedAt:   time.Now(),
		interactive: term.IsTerminal(int(os.Stdout.Fd())),
		done:        make(chan struct{}),
	}
}

// Start begins the rendering loop.
func (pd *ProgressDisplay) Start() {
	pd.mu.Lock()
	if pd.running {
		pd.mu.Unlock()
		return
	}
	pd.running = true
	pd.mu.Unlock()

	go pd.renderLoop()
}

// Stop stops the progress display and clears the output area.
func (pd *ProgressDisplay) Stop() {
	pd.mu.Lock()
	if !pd.running {
		pd.mu.Unlock()
		return
	}
	pd.running = false
	pd.mu.Unlock()

	close(pd.done)
	if pd.interactive {
		pd.clearDisplay()
	}
}

// StopWithSuccess stops and prints a success message.
func (pd *ProgressDisplay) StopWithSuccess(msg string) {
	pd.Stop()
	fmt.Printf("  %s%s✓%s %s\n", Bold, Green, Reset, msg)
}

// StopWithError stops and prints an error message.
func (pd *ProgressDisplay) StopWithError(msg string) {
	pd.Stop()
	fmt.Printf("  %s%s✗%s %s\n", Bold, Red, Reset, msg)
}

// SetPhase explicitly transitions to a new phase.
func (pd *ProgressDisplay) SetPhase(phase Phase) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.phase = phase
}

// AddActivity adds a new activity line to the display.
func (pd *ProgressDisplay) AddActivity(text string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.addActivity(text)
}

func (pd *ProgressDisplay) addActivity(text string) {
	// Mark previous last activity as done
	if len(pd.activities) > 0 {
		pd.activities[len(pd.activities)-1].done = true
	}
	pd.activities = append(pd.activities, activity{text: text, done: false})
	// Trim to max
	if len(pd.activities) > maxActivities {
		pd.activities = pd.activities[len(pd.activities)-maxActivities:]
	}
}

// SetStatus sets the dimmed status text (from assistant messages).
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

// ResetForRetry resets transient display state for a new completion pass
// while preserving cumulative counters (filesWritten) and totalFiles.
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
// If the new total is less than files already written, it's raised to filesWritten.
func (pd *ProgressDisplay) SetTotalFiles(total int) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.totalFiles = total
	if pd.totalFiles < pd.filesWritten {
		pd.totalFiles = pd.filesWritten
	}
}

// OnToolUse processes a tool_use event and updates the display state.
func (pd *ProgressDisplay) OnToolUse(toolName string, inputGetter func(key string) string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	// Clear stale status and streaming buffer once real tool activity begins.
	pd.statusText = ""
	pd.streamingBuf.Reset()

	switch toolName {
	case "Write":
		path := inputGetter("file_path")
		if path != "" {
			short := shortPath(path)
			if pd.mode == "build" {
				pd.phase = PhaseBuildingCode
			}
			pd.filesWritten++
			if pd.filesWritten > pd.totalFiles && pd.totalFiles > 0 {
				pd.totalFiles = pd.filesWritten
			}
			pd.addActivity(fmt.Sprintf("Writing %s", short))
		}
	case "Edit":
		path := inputGetter("file_path")
		if path != "" {
			short := shortPath(path)
			if pd.buildFailed {
				pd.phase = PhaseFixing
			} else if pd.mode == "edit" {
				pd.phase = PhaseEditing
			}
			pd.addActivity(fmt.Sprintf("Editing %s", short))
		}
	case "Read":
		path := inputGetter("file_path")
		if path != "" {
			short := shortPath(path)
			pd.addActivity(fmt.Sprintf("Reading %s", short))
		}
	case "Bash":
		command := inputGetter("command")
		if strings.Contains(command, "xcodegen") {
			pd.phase = PhaseGenerating
			pd.addActivity("Generating Xcode project")
		} else if strings.Contains(command, "xcodebuild") {
			pd.phase = PhaseCompiling
			pd.addActivity("Compiling project")
		} else if strings.Contains(command, "git") {
			pd.addActivity("Updating repository")
		} else if command != "" {
			short := command
			if len(short) > 35 {
				short = short[:35] + "..."
			}
			pd.addActivity(short)
		}
	case "Glob":
		pd.addActivity("Searching files...")
	case "Grep":
		pd.addActivity("Searching code...")
	case "WebFetch", "WebSearch":
		pd.addActivity("Searching web...")
	default:
		if label := friendlyToolName(toolName, inputGetter); label != "" {
			pd.addActivity(label)
		}
	}
}

// OnStreamingText processes a token-by-token text delta from content_block_delta events.
// It accumulates text and updates the status display in real-time.
func (pd *ProgressDisplay) OnStreamingText(text string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.streamingBuf.WriteString(text)

	// Extract a mode-aware preview from accumulated text for display.
	accumulated := pd.streamingBuf.String()
	status := extractStreamingPreview(accumulated, pd.mode)
	if status != "" {
		pd.statusText = status
	}
}

// OnAssistantText processes assistant text content (full message, not deltas).
func (pd *ProgressDisplay) OnAssistantText(text string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	// Reset streaming buffer since we got the full message
	pd.streamingBuf.Reset()

	// Detect build failure mentions to transition phase
	lower := strings.ToLower(text)
	if strings.Contains(lower, "build failed") || strings.Contains(lower, "compilation error") ||
		strings.Contains(lower, "build error") {
		pd.buildFailed = true
	}
	if pd.buildFailed && (strings.Contains(lower, "fix") || strings.Contains(lower, "correct") ||
		strings.Contains(lower, "let me") || strings.Contains(lower, "i'll")) {
		pd.phase = PhaseFixing
		pd.fixAttempts++
	}

	if isStructuredStreamingPreviewMode(pd.mode) {
		if status := extractStreamingPreview(text, pd.mode); status != "" {
			pd.statusText = status
		}
		return
	}

	// Extract a short meaningful status from assistant text
	status := extractStatus(text)
	if status != "" {
		pd.statusText = status
	}
}

// renderLoop runs the rendering goroutine.
func (pd *ProgressDisplay) renderLoop() {
	frame := 0
	for {
		select {
		case <-pd.done:
			return
		default:
			pd.render(frame)
			frame++
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// render draws the current state to the terminal.
func (pd *ProgressDisplay) render(frame int) {
	pd.mu.Lock()
	phase := pd.phase
	totalFiles := pd.totalFiles
	filesWritten := pd.filesWritten
	activities := make([]activity, len(pd.activities))
	copy(activities, pd.activities)
	statusText := pd.statusText
	totalPhases := pd.totalPhases
	buildFailed := pd.buildFailed
	elapsed := time.Since(pd.startedAt)
	interactive := pd.interactive
	pd.mu.Unlock()

	if !interactive {
		pd.renderNonInteractive(phase, totalPhases, totalFiles, filesWritten, activities, statusText, buildFailed)
		return
	}

	spinChar := spinnerFrames[frame%len(spinnerFrames)]
	var lines []string

	// Phase header with progress bar and elapsed time
	phaseHeader := pd.buildPhaseHeader(phase, totalPhases, totalFiles, filesWritten, buildFailed, spinChar, elapsed)
	lines = append(lines, phaseHeader)

	// Activity tree
	for i, act := range activities {
		prefix := "  ├─ "
		if i == len(activities)-1 {
			prefix = "  └─ "
		}
		marker := spinChar
		color := Cyan
		if act.done {
			marker = "✓"
			color = Green
		}
		lines = append(lines, fmt.Sprintf("%s%s%s%s %s%s", Dim, prefix, color, marker, Reset+act.text, Reset))
	}

	// Status text (assistant thinking)
	if statusText != "" {
		lines = append(lines, fmt.Sprintf("  %s%s%s", Dim, statusText, Reset))
	}

	// Pad to fixed height to avoid flicker
	for len(lines) < maxActivities+3 {
		lines = append(lines, "")
	}

	// Move cursor up and overwrite
	totalLines := len(lines)
	if frame > 0 {
		fmt.Printf("\033[%dA", totalLines) // move up
	}
	for _, line := range lines {
		// Clear line and print
		fmt.Printf("\r\033[K%s\n", line)
	}
}

func (pd *ProgressDisplay) renderNonInteractive(phase Phase, totalPhases, totalFiles, filesWritten int, activities []activity, statusText string, buildFailed bool) {
	header := pd.buildPhaseHeader(phase, totalPhases, totalFiles, filesWritten, buildFailed, "•", 0)

	latestActivity := ""
	if len(activities) > 0 {
		act := activities[len(activities)-1]
		marker := "•"
		if act.done {
			marker = "✓"
		}
		latestActivity = fmt.Sprintf("  %s %s", marker, act.text)
	}

	parts := []string{header, latestActivity, statusText}
	renderID := strings.Join(parts, "\n")

	pd.mu.Lock()
	if renderID == pd.lastRenderID {
		pd.mu.Unlock()
		return
	}
	pd.lastRenderID = renderID
	pd.mu.Unlock()

	fmt.Println(header)
	if latestActivity != "" {
		fmt.Println(latestActivity)
	}
	if statusText != "" {
		fmt.Println("  " + statusText)
	}
}

// buildPhaseHeader builds the header line with optional progress bar.
func (pd *ProgressDisplay) buildPhaseHeader(phase Phase, totalPhases, totalFiles, filesWritten int, buildFailed bool, spinChar string, elapsed time.Duration) string {
	var sb strings.Builder
	sb.WriteString("  ")

	// Phase number (only for build mode with numbered phases)
	if totalPhases > 0 {
		phaseNum := phase.number()
		if buildFailed && phase == PhaseFixing {
			sb.WriteString(fmt.Sprintf("%s%s %s...%s", Yellow, spinChar, phase.label(), Reset))
		} else {
			sb.WriteString(fmt.Sprintf("%sPhase %d/%d:%s %s%s %s...%s",
				Dim, phaseNum, totalPhases, Reset, Cyan, spinChar, phase.label(), Reset))
		}
	} else {
		sb.WriteString(fmt.Sprintf("%s%s %s...%s", Cyan, spinChar, phase.label(), Reset))
	}

	// Progress bar for building code phase
	if phase == PhaseBuildingCode && totalFiles > 0 {
		sb.WriteString("  ")
		sb.WriteString(buildProgressBar(filesWritten, totalFiles))
		sb.WriteString(fmt.Sprintf(" %s%d/%d files%s", Dim, filesWritten, totalFiles, Reset))
	}

	// Elapsed time
	sb.WriteString(fmt.Sprintf("  %s%s%s", Dim, formatElapsed(elapsed), Reset))

	return sb.String()
}

// formatElapsed formats a duration as a compact time string.
func formatElapsed(d time.Duration) string {
	s := int(d.Seconds())
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	m := s / 60
	s = s % 60
	return fmt.Sprintf("%dm%02ds", m, s)
}

// clearDisplay clears the progress display area.
func (pd *ProgressDisplay) clearDisplay() {
	total := maxActivities + 3
	for i := 0; i < total; i++ {
		fmt.Printf("\033[K\n") // clear line and move down
	}
	fmt.Printf("\033[%dA", total) // move back up
}

// buildProgressBar creates a progress bar string.
func buildProgressBar(current, total int) string {
	if total <= 0 {
		return ""
	}
	width := 16
	filled := (current * width) / total
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("%s[%s]%s", Dim, bar, Reset)
}

// shortPath extracts a meaningful short path from a full file path.
func shortPath(fullPath string) string {
	// Find the app source directory and show relative path
	parts := strings.Split(fullPath, "/")
	// Show last 2 components (e.g., "Models/Habit.swift")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	if len(parts) >= 1 {
		return parts[len(parts)-1]
	}
	return fullPath
}

// friendlyToolName maps MCP tool names to human-readable activity labels.
func friendlyToolName(toolName string, inputGetter func(key string) string) string {
	switch toolName {
	// Apple docs
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

	// XcodeGen project config
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

// truncateActivity truncates an activity label to fit the display.
func truncateActivity(s string) string {
	const maxWidth = 45
	if len(s) > maxWidth {
		return s[:maxWidth] + "..."
	}
	return s
}

// extractStreamingPreview returns a mode-aware live preview for streaming text.
// Structured modes intentionally avoid raw JSON tails in the UI.
func extractStreamingPreview(text, mode string) string {
	if !isStructuredStreamingPreviewMode(mode) {
		return extractLastLine(text)
	}
	if strings.TrimSpace(text) == "" {
		return ""
	}
	switch mode {
	case "intent":
		return "Preparing routing decision..."
	case "analyze":
		return "Preparing analysis output..."
	case "plan":
		return "Preparing build plan..."
	default:
		return "Preparing structured output..."
	}
}

func isStructuredStreamingPreviewMode(mode string) bool {
	return mode == "intent" || mode == "analyze" || mode == "plan"
}

func tailRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[len(r)-max:])
}

func truncateTailWithEllipsis(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[len(r)-max:])
	}
	return "..." + string(r[len(r)-(max-3):])
}

// extractStatus extracts a short, meaningful status from assistant text.
func extractStatus(text string) string {
	// Take the first sentence, truncated
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	// Find first sentence boundary
	for i, ch := range text {
		if ch == '.' || ch == '\n' {
			text = text[:i]
			break
		}
	}

	// Truncate to max width
	if len(text) > maxStatusWidth {
		text = text[:maxStatusWidth] + "..."
	}

	return text
}

// extractLastLine returns the last non-empty line from streaming text,
// skipping JSON content and code blocks. Used to show real-time status
// from token-by-token streaming during generation.
func extractLastLine(text string) string {
	lines := strings.Split(text, "\n")

	// Walk backwards to find the last meaningful line
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		// Skip JSON and code block content
		if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "}") ||
			strings.HasPrefix(line, "\"") || strings.HasPrefix(line, "[") ||
			strings.HasPrefix(line, "]") || strings.HasPrefix(line, "```") {
			continue
		}
		// Truncate
		if len(line) > maxStatusWidth {
			line = line[:maxStatusWidth] + "..."
		}
		return line
	}

	return ""
}
