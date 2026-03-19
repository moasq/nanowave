package terminal

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rivo/uniseg"
	"golang.org/x/term"
)

// Colors for terminal output.
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
)

// Spinner provides a terminal spinner for long-running operations.
type Spinner struct {
	mu      sync.Mutex
	message string
	running bool
	done    chan struct{}
	exited  chan struct{} // closed when goroutine exits
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// NewSpinner creates a new spinner.
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		done:    make(chan struct{}),
		exited:  make(chan struct{}),
	}
}

// Start begins the spinner animation.
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		defer close(s.exited)
		i := 0
		for {
			select {
			case <-s.done:
				return
			default:
				s.mu.Lock()
				msg := s.message
				s.mu.Unlock()

				frame := spinnerFrames[i%len(spinnerFrames)]
				fmt.Printf("\r%s%s %s%s", Cyan, frame, msg, Reset)
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
}

// Update changes the spinner message.
func (s *Spinner) Update(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = message
}

// Stop stops the spinner and clears the line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.done)
	<-s.exited // wait for goroutine to stop writing
	fmt.Printf("\r%s\r", strings.Repeat(" ", 80))
}

// StopWithMessage stops the spinner and prints a final message.
func (s *Spinner) StopWithMessage(message string) {
	s.Stop()
	printTerminalLine(message)
}

// UI helper functions.

// Success prints a green success message.
func Success(msg string) {
	printTerminalLine(fmt.Sprintf("%s%s✓%s %s", Bold, Green, Reset, msg))
}

// Error prints a red error message.
func Error(msg string) {
	printTerminalLine(fmt.Sprintf("%s%s✗%s %s", Bold, Red, Reset, msg))
}

// Info prints a blue info message.
func Info(msg string) {
	printTerminalLine(fmt.Sprintf("%s%si%s %s", Bold, Blue, Reset, msg))
}

// Warning prints a yellow warning message.
func Warning(msg string) {
	printTerminalLine(fmt.Sprintf("%s%s!%s %s", Bold, Yellow, Reset, msg))
}

// Header prints a bold header.
func Header(msg string) {
	fmt.Printf("\n%s%s%s\n", Bold, msg, Reset)
}

// Detail prints an indented detail line.
func Detail(label, value string) {
	printTerminalLine(fmt.Sprintf("  %s%s:%s %s", Dim, label, Reset, value))
}

// Progress prints a progress indicator.
func Progress(current, total int, label string) {
	fmt.Printf("\r  %s[%d/%d]%s %s", Cyan, current, total, Reset, label)
	if current == total {
		fmt.Println()
	}
}

// Divider prints a horizontal line.
func Divider() {
	fmt.Printf("%s%s%s\n", Dim, strings.Repeat("─", 60), Reset)
}

// Banner prints the welcome box with the given version.
func Banner(version string) {
	fmt.Println()
	fmt.Printf("  %s╭─────────────────────────────────╮%s\n", Dim, Reset)
	fmt.Printf("  %s│%s  Nanowave %s%-22s%s%s│%s\n", Dim, Reset, Bold, "v"+version, Reset, Dim, Reset)
	fmt.Printf("  %s│%s  Autonomous iOS app builder     %s│%s\n", Dim, Reset, Dim, Reset)
	fmt.Printf("  %s╰─────────────────────────────────╯%s\n", Dim, Reset)
	fmt.Println()
}

// ToolStatusOpts holds the status of each prerequisite tool.
type ToolStatusOpts struct {
	RuntimeVersion string
	HasXcode       bool
	HasSimulator   bool
	HasXcodegen    bool
	AuthEmail      string
	AuthPlan       string
	AuthLoggedIn   bool
	AuthDetail     string
}

// ToolStatus prints tool availability.
func ToolStatus(opts ToolStatusOpts) {
	mark := func(ok bool) string {
		if ok {
			return Green + "✓" + Reset
		}
		return Red + "✗" + Reset
	}

	fmt.Printf("  %sTools:%s Xcode %s, Simulator %s, XcodeGen %s\n",
		Dim, Reset, mark(opts.HasXcode), mark(opts.HasSimulator), mark(opts.HasXcodegen))

	// Auth status line
	if opts.AuthLoggedIn {
		fmt.Printf("  %sAccount:%s %sSigned in%s", Dim, Reset, Green, Reset)
		planLabel := opts.AuthPlan
		if planLabel != "" {
			planLabel = strings.ToUpper(planLabel[:1]) + planLabel[1:] + " plan"
			fmt.Printf(" (%s)", planLabel)
		}
		if detail := strings.TrimSpace(opts.AuthDetail); detail != "" && !strings.Contains(detail, "\n") {
			fmt.Printf(" %s— %s%s", Dim, detail, Reset)
		}
		fmt.Println()
	} else if opts.RuntimeVersion != "" {
		loginHint := opts.AuthDetail
		if loginHint == "" {
			loginHint = "login with the selected runtime"
		}
		fmt.Printf("  %sAccount:%s %sNot signed in%s %s— %s%s\n",
			Dim, Reset, Yellow, Reset, Dim, loginHint, Reset)
	}

	missing := !opts.HasXcode || opts.RuntimeVersion == "" || !opts.HasSimulator || !opts.HasXcodegen
	if missing {
		fmt.Printf("  %sRun /setup to install missing tools.%s\n", Dim, Reset)
	}
	fmt.Println()
}

// Prompt prints the input prompt.
func Prompt() {
	fmt.Printf("%s> %s", Bold, Reset)
}

// EchoInput echoes the submitted request into the terminal transcript so it
// remains visible while the request is being processed.
func EchoInput(text string, images []string) {
	text = strings.TrimRight(normalizeEditorText(text), "\n")
	lines := queuedInputLines(text, images)
	if len(lines) == 0 {
		return
	}

	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		width = 80
	}
	padding, contentWidth := inputBoxMetrics(width)

	fmt.Println()
	for i := 0; i < inputBoxVerticalPadding; i++ {
		fmt.Println(renderQueuedBoxLine("", width, padding, contentWidth))
	}
	for _, line := range lines {
		fmt.Println(renderQueuedBoxLine(line, width, padding, contentWidth))
	}
	for i := 0; i < inputBoxVerticalPadding; i++ {
		fmt.Println(renderQueuedBoxLine("", width, padding, contentWidth))
	}
	fmt.Println()
}

// ReadSimpleLine reads a single line from stdin. Used for HITL prompts
// during streaming operations where the full readline editor isn't needed.
func ReadSimpleLine() string {
	fmt.Printf("%s> %s", Bold, Reset)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

func queuedInputLines(text string, images []string) []string {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		width = 80
	}
	_, contentWidth := inputBoxMetrics(width)

	text = normalizeEditorText(text)
	attachmentLabels := make([]string, 0, len(images))
	for i := range images {
		attachmentLabels = append(attachmentLabels, formatImageReference(i+1))
	}

	if text != "" && len(attachmentLabels) > 0 || strings.Contains(text, "[") {
		textLines := strings.Split(text, "\n")
		last := len(textLines) - 1
		if last >= 0 {
			if body, labels, ok := extractTrailingAttachmentLabels(textLines[last]); ok {
				textLines[last] = body
				attachmentLabels = append(attachmentLabels, labels...)
				text = strings.Join(textLines, "\n")
			}
		}
	}

	combined := appendAttachmentLabelsToText(text, attachmentLabels)
	if strings.TrimSpace(combined) == "" {
		return nil
	}
	return layoutEditorBuffer([]rune(combined), contentWidth).lines
}

func extractTrailingAttachmentLabels(line string) (string, []string, bool) {
	for start := 0; start < len(line); start++ {
		if line[start] != '[' {
			continue
		}
		if start > 0 && line[start-1] != ' ' {
			continue
		}
		labels, ok := parseAttachmentLabels(line[start:])
		if !ok {
			continue
		}
		return strings.TrimRight(line[:start], " "), labels, true
	}
	return line, nil, false
}

func parseAttachmentLabels(text string) ([]string, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, false
	}

	var labels []string
	for len(text) > 0 {
		if text[0] != '[' {
			return nil, false
		}
		end := strings.IndexByte(text, ']')
		if end < 0 {
			return nil, false
		}

		label := text[:end+1]
		if !isAttachmentLabel(label) {
			return nil, false
		}
		labels = append(labels, label)
		text = strings.TrimSpace(text[end+1:])
	}

	return labels, len(labels) > 0
}

func isAttachmentLabel(label string) bool {
	return strings.HasPrefix(label, "[Image #") || strings.HasPrefix(label, "[Pasted Text #")
}

func renderQueuedBoxLine(content string, width, padding, contentWidth int) string {
	content = trimDisplayWidth(content, contentWidth)
	contentWidthUsed := uniseg.StringWidth(content)
	rightPadding := width - padding - contentWidthUsed
	if rightPadding < 0 {
		rightPadding = 0
	}

	return inputBoxBackground +
		inputBoxForeground +
		strings.Repeat(" ", padding) +
		content +
		strings.Repeat(" ", rightPadding) +
		Reset
}
