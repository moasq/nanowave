package terminal

import (
	"fmt"
	"strings"
	"sync"
	"time"
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
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// NewSpinner creates a new spinner.
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		done:    make(chan struct{}),
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
	fmt.Printf("\r%s\r", strings.Repeat(" ", 80))
}

// StopWithMessage stops the spinner and prints a final message.
func (s *Spinner) StopWithMessage(message string) {
	s.Stop()
	fmt.Println(message)
}

// UI helper functions.

// Success prints a green success message.
func Success(msg string) {
	fmt.Printf("%s%s✓%s %s\n", Bold, Green, Reset, msg)
}

// Error prints a red error message.
func Error(msg string) {
	fmt.Printf("%s%s✗%s %s\n", Bold, Red, Reset, msg)
}

// Info prints a blue info message.
func Info(msg string) {
	fmt.Printf("%s%si%s %s\n", Bold, Blue, Reset, msg)
}

// Warning prints a yellow warning message.
func Warning(msg string) {
	fmt.Printf("%s%s!%s %s\n", Bold, Yellow, Reset, msg)
}

// Header prints a bold header.
func Header(msg string) {
	fmt.Printf("\n%s%s%s\n", Bold, msg, Reset)
}

// Detail prints an indented detail line.
func Detail(label, value string) {
	fmt.Printf("  %s%s:%s %s\n", Dim, label, Reset, value)
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
	ClaudeVersion string
	HasXcode      bool
	HasXcodeCLT   bool
	HasSimulator  bool
	HasXcodegen   bool
	AuthEmail     string // Claude account email (empty if not logged in)
	AuthPlan      string // Subscription plan (e.g., "max", "pro", "free")
	AuthLoggedIn  bool   // Whether Claude auth is active
}

// ToolStatus prints tool availability.
func ToolStatus(opts ToolStatusOpts) {
	mark := func(ok bool) string {
		if ok {
			return Green + "✓" + Reset
		}
		return Red + "✗" + Reset
	}

	claudeStatus := mark(opts.ClaudeVersion != "")
	if opts.ClaudeVersion != "" {
		claudeStatus = opts.ClaudeVersion
	}

	fmt.Printf("  %sTools:%s Claude Code %s, Xcode %s, Simulator %s, XcodeGen %s\n",
		Dim, Reset, claudeStatus, mark(opts.HasXcode), mark(opts.HasSimulator), mark(opts.HasXcodegen))

	// Auth status line
	if opts.AuthLoggedIn && opts.AuthEmail != "" {
		planLabel := opts.AuthPlan
		if planLabel != "" {
			planLabel = strings.ToUpper(planLabel[:1]) + planLabel[1:] + " plan"
		}
		if planLabel != "" {
			fmt.Printf("  %sAccount:%s %s (%s)\n", Dim, Reset, opts.AuthEmail, planLabel)
		} else {
			fmt.Printf("  %sAccount:%s %s\n", Dim, Reset, opts.AuthEmail)
		}
	} else if opts.ClaudeVersion != "" {
		fmt.Printf("  %sAccount:%s %sNot signed in%s %s— run %sclaude auth login%s\n",
			Dim, Reset, Yellow, Reset, Dim, Bold, Reset)
	}

	missing := !opts.HasXcode || opts.ClaudeVersion == "" || !opts.HasSimulator || !opts.HasXcodegen
	if missing {
		fmt.Printf("  %sRun /setup to install missing tools.%s\n", Dim, Reset)
	}
	fmt.Println()
}

// Prompt prints the input prompt.
func Prompt() {
	fmt.Printf("%s> %s", Bold, Reset)
}

