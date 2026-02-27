package terminal

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/reeflective/readline"
	"golang.org/x/term"
)

// SlashCommands is the list of available commands for autocomplete.
var SlashCommands = []CommandInfo{
	{Name: "/run", Desc: "Build and launch the app"},
	{Name: "/simulator", Desc: "Select simulator device"},
	{Name: "/model", Desc: "Show or switch model"},
	{Name: "/fix", Desc: "Auto-fix build errors"},
	{Name: "/ask", Desc: "Ask a question about your project"},
	{Name: "/open", Desc: "Open project in Xcode"},
	{Name: "/info", Desc: "Show project info"},
	{Name: "/usage", Desc: "Show token usage and costs"},
	{Name: "/clear", Desc: "Clear conversation session"},
	{Name: "/projects", Desc: "Switch project"},
	{Name: "/setup", Desc: "Install prerequisites"},
	{Name: "/integrations", Desc: "Manage backend integrations"},
	{Name: "/help", Desc: "Show available commands"},
	{Name: "/quit", Desc: "Exit session"},
}

// CommandInfo holds a command name and description.
type CommandInfo struct {
	Name string
	Desc string
}

// InputResult holds the parsed result from ReadInput.
type InputResult struct {
	Text   string   // The text prompt (with image paths removed)
	Images []string // Absolute paths to image files found in the input
}

// imageExtensions are file extensions recognized as images.
var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".webp": true, ".bmp": true, ".tiff": true, ".tif": true,
	".heic": true, ".heif": true, ".svg": true,
}

// isImagePath checks if a string looks like a path to an image file.
func isImagePath(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(s))
	if !imageExtensions[ext] {
		return false
	}
	// Must be an absolute path or start with ~
	if !filepath.IsAbs(s) && !strings.HasPrefix(s, "~") {
		return false
	}
	// Expand ~ to home dir
	resolved := s
	if strings.HasPrefix(s, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			resolved = filepath.Join(home, s[1:])
		}
	}
	info, err := os.Stat(resolved)
	return err == nil && !info.IsDir()
}

// resolveImagePath expands ~ and cleans up a path.
func resolveImagePath(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			s = filepath.Join(home, s[1:])
		}
	}
	return filepath.Clean(s)
}

// extractImages separates image paths from the input text.
// Returns the remaining text and a list of image paths.
func extractImages(input string) (string, []string) {
	// First check if the entire input (single line) is just an image path
	trimmed := strings.TrimSpace(input)
	if isImagePath(trimmed) {
		return "", []string{resolveImagePath(trimmed)}
	}

	lines := strings.Split(input, "\n")
	var textLines []string
	var images []string

	for _, line := range lines {
		lt := strings.TrimSpace(line)
		if isImagePath(lt) {
			images = append(images, resolveImagePath(lt))
		} else {
			textLines = append(textLines, line)
		}
	}

	return strings.TrimSpace(strings.Join(textLines, "\n")), images
}

// rl is the shared readline shell instance, initialized eagerly so the
// first ReadInput call has no startup delay.
var rl = newShell()

func newShell() *readline.Shell {
	sh := readline.NewShell()

	// Enable as-you-type autocomplete so slash commands appear
	// immediately while typing (like the old custom implementation).
	sh.Config.Set("autocomplete", true)

	sh.Prompt.Primary(func() string {
		return Bold + "> " + Reset
	})
	sh.Prompt.Secondary(func() string {
		return Dim + "  " + Reset
	})

	// Enter always submits; Ctrl+J inserts a newline (standard readline).
	sh.AcceptMultiline = func(line []rune) bool {
		return true
	}

	// Slash command completion: only activate for "/" prefix.
	sh.Completer = func(line []rune, cursor int) readline.Completions {
		text := string(line[:cursor])
		if !strings.HasPrefix(text, "/") {
			return readline.Completions{}
		}
		matches := filterCommands(text)
		if len(matches) == 0 {
			return readline.Completions{}
		}
		pairs := make([]string, 0, len(matches)*2)
		for _, cmd := range matches {
			pairs = append(pairs, cmd.Name, cmd.Desc)
		}
		return readline.CompleteValuesDescribed(pairs...).NoSpace()
	}

	return sh
}

// ReadInput reads input from the terminal with slash command completion.
// Enter submits. Ctrl+J adds a newline. Pasted multiline text is kept intact.
// Image file paths (dragged/pasted) are detected and returned separately.
func ReadInput() InputResult {
	line, err := rl.Readline()
	if err != nil {
		if errors.Is(err, readline.ErrInterrupt) {
			fmt.Println()
			os.Exit(130)
		}
		if errors.Is(err, io.EOF) {
			fmt.Println()
			return InputResult{}
		}
		// Other error — return empty.
		return InputResult{}
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return InputResult{}
	}

	text, imgs := extractImages(line)
	return InputResult{Text: text, Images: imgs}
}

// filterCommands returns commands matching the given prefix.
func filterCommands(prefix string) []CommandInfo {
	if len(prefix) == 0 || prefix[0] != '/' {
		return nil
	}
	lower := strings.ToLower(prefix)
	var matches []CommandInfo
	for _, cmd := range SlashCommands {
		if strings.HasPrefix(strings.ToLower(cmd.Name), lower) {
			matches = append(matches, cmd)
		}
	}
	return matches
}

// rawWrite writes directly to stdout in raw mode.
func rawWrite(s string) {
	os.Stdout.WriteString(s)
}

// readWithTimeout tries to read from stdin within the given duration.
// Returns bytes read and count. If timeout expires, returns 0.
func readWithTimeout(buf []byte, timeout time.Duration) int {
	fd := int(os.Stdin.Fd())

	// Set non-blocking
	syscall.SetNonblock(fd, true)
	defer syscall.SetNonblock(fd, false)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			return n
		}
		if err != nil {
			return 0
		}
		time.Sleep(5 * time.Millisecond)
	}
	return 0
}

// PickerOption represents an option in the interactive picker.
type PickerOption struct {
	Label string
	Desc  string
}

// Pick shows an interactive picker with arrow key navigation.
// Returns the selected option's Label, or "" if cancelled.
// The picker limits visible options and scrolls when the list is long.
func Pick(title string, options []PickerOption, currentLabel string) string {
	if len(options) == 0 {
		return ""
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return ""
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return ""
	}
	defer term.Restore(fd, oldState)

	// Hide cursor during picker
	rawWrite("\033[?25l")

	// Find initial selected index based on current value
	selected := 0
	for i, opt := range options {
		if opt.Label == currentLabel {
			selected = i
			break
		}
	}

	// Limit visible rows to prevent scrolling issues
	_, termHeight, _ := term.GetSize(fd)
	maxVisible := len(options)
	if termHeight > 0 && maxVisible > termHeight-4 {
		maxVisible = termHeight - 4
	}
	if maxVisible < 3 {
		maxVisible = 3
	}

	scrollOffset := 0 // index of first visible option

	adjustScroll := func() {
		if selected < scrollOffset {
			scrollOffset = selected
		} else if selected >= scrollOffset+maxVisible {
			scrollOffset = selected - maxVisible + 1
		}
	}
	adjustScroll()

	// Print title
	titleLines := 0
	if title != "" {
		rawWrite(fmt.Sprintf("\r\n  %s%s%s\r\n", Bold, title, Reset))
		titleLines = 2
	} else {
		rawWrite("\r\n")
		titleLines = 1
	}

	visibleCount := maxVisible
	if visibleCount > len(options) {
		visibleCount = len(options)
	}

	drawOptions := func() {
		end := scrollOffset + visibleCount
		if end > len(options) {
			end = len(options)
		}
		for i := scrollOffset; i < end; i++ {
			opt := options[i]
			rawWrite("\r\033[K")
			if i == selected {
				rawWrite(fmt.Sprintf("  %s%s▸%s %s%-12s%s %s%s%s\r\n", Bold, Cyan, Reset, Bold, opt.Label, Reset, Dim, opt.Desc, Reset))
			} else {
				rawWrite(fmt.Sprintf("    %-12s %s%s%s\r\n", opt.Label, Dim, opt.Desc, Reset))
			}
		}
		// Hint line
		rawWrite("\r\033[K")
		hint := "↑↓ navigate  Enter select  q cancel"
		if len(options) > visibleCount {
			hint = fmt.Sprintf("↑↓ scroll (%d/%d)  Enter select  q cancel", selected+1, len(options))
		}
		rawWrite(fmt.Sprintf("  %s%s%s\r\n", Dim, hint, Reset))
	}

	drawnLines := visibleCount + 1 // visible options + hint

	moveUp := func(n int) {
		if n > 0 {
			rawWrite(fmt.Sprintf("\033[%dA", n))
		}
	}

	cleanup := func() {
		// Move up to first option line (we're already there after moveUp in the loop)
		// Then move up past title
		moveUp(titleLines)
		total := titleLines + drawnLines
		for i := 0; i < total; i++ {
			rawWrite("\r\033[K\r\n")
		}
		moveUp(total)
		rawWrite("\033[?25h")
	}

	drawOptions()
	moveUp(drawnLines)

	buf := make([]byte, 1)
	for {
		n, readErr := os.Stdin.Read(buf)
		if readErr != nil || n == 0 {
			break
		}

		b0 := buf[0]

		// Escape sequences
		if b0 == 0x1b {
			// Try to read more bytes with a short timeout to distinguish
			// standalone Esc from an escape sequence (e.g. arrow keys)
			extra := make([]byte, 7)
			en := readWithTimeout(extra, 50*time.Millisecond)
			if en == 0 {
				// No follow-up bytes — standalone Esc, cancel
				cleanup()
				return ""
			}
			if en >= 2 && extra[0] == '[' {
				switch extra[1] {
				case 'A': // Up
					if selected > 0 {
						selected--
					} else {
						selected = len(options) - 1
					}
					adjustScroll()
					drawOptions()
					moveUp(drawnLines)
				case 'B': // Down
					if selected < len(options)-1 {
						selected++
					} else {
						selected = 0
					}
					adjustScroll()
					drawOptions()
					moveUp(drawnLines)
				}
			}
			continue
		}

		switch b0 {
		case 13, 10: // Enter — confirm selection
			result := options[selected].Label
			cleanup()
			return result

		case 3: // Ctrl+C — cancel
			cleanup()
			return ""

		case 'q': // q — cancel
			cleanup()
			return ""
		}
	}

	cleanup()
	return ""
}
