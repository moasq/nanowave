package terminal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
)

// SlashCommands is the list of available commands for autocomplete.
var SlashCommands = []CommandInfo{
	{Name: "/run", Desc: "Build and launch in simulator"},
	{Name: "/simulator", Desc: "Select simulator device"},
	{Name: "/model", Desc: "Show or switch model"},
	{Name: "/fix", Desc: "Auto-fix build errors"},
	{Name: "/open", Desc: "Open project in Xcode"},
	{Name: "/info", Desc: "Show project info"},
	{Name: "/usage", Desc: "Show token usage and costs"},
	{Name: "/clear", Desc: "Clear conversation session"},
	{Name: "/projects", Desc: "Switch project"},
	{Name: "/setup", Desc: "Install prerequisites"},
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


// ReadInput reads input from the terminal with slash command completion.
// Single Enter submits. Shift+Enter (or Esc then Enter) adds a newline.
// Image file paths (dragged/pasted) are detected and returned separately.
// Multi-line content can be navigated with up/down arrows.
func ReadInput() InputResult {
	fd := int(os.Stdin.Fd())

	if !term.IsTerminal(fd) {
		text := readLineFallback()
		t, imgs := extractImages(text)
		return InputResult{Text: t, Images: imgs}
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		text := readLineFallback()
		t, imgs := extractImages(text)
		return InputResult{Text: t, Images: imgs}
	}

	os.Stdout.WriteString("\033[?2004h") // enable bracketed paste

	restore := func() {
		os.Stdout.WriteString("\033[?2004l")
		term.Restore(fd, oldState)
	}
	defer restore()

	// --- Core data model: array of lines ---
	lines := [][]rune{{}} // start with one empty line
	row := 0              // current line index
	col := 0              // cursor column (rune index) within current line

	menuLines := 0
	selectedIdx := -1
	escPressed := false

	// How many terminal rows our content occupied on the last draw.
	drawnRows := 0

	tw := func() int {
		w, _, err := term.GetSize(fd)
		if err != nil || w <= 0 {
			return 80
		}
		return w
	}

	promptStr := func(lineIdx int) string {
		if lineIdx == 0 {
			return Bold + "> " + Reset
		}
		return Dim + "  " + Reset
	}

	// visualRows returns how many terminal rows a line occupies.
	visualRows := func(lineLen int) int {
		total := 2 + lineLen // 2 = prompt width
		w := tw()
		if total <= w {
			return 1
		}
		return (total + w - 1) / w
	}

	// totalDrawnRows returns total terminal rows for all lines.
	totalDrawnRows := func() int {
		n := 0
		for _, l := range lines {
			n += visualRows(len(l))
		}
		return n
	}

	// cursorRowFromTop returns how many terminal rows from the top
	// of our content to the cursor position.
	cursorRowFromTop := func() int {
		n := 0
		for i := 0; i < row; i++ {
			n += visualRows(len(lines[i]))
		}
		// Add wrapped rows within current line
		cursorAbs := 2 + col // 2 = prompt width
		n += cursorAbs / tw()
		return n
	}

	// redrawAll redraws all lines from scratch. The cursor is assumed
	// to be at the first row of our content (row 0, col 0 of terminal).
	redrawAll := func() {
		// Move cursor to top of our drawn area
		curRow := cursorRowFromTop()
		if curRow > 0 {
			rawWrite(fmt.Sprintf("\033[%dA", curRow))
		}
		rawWrite("\r\033[J") // clear from here to end of screen

		for i, l := range lines {
			rawWrite(promptStr(i))
			rawWrite(string(l))
			if i < len(lines)-1 {
				rawWrite("\r\n")
			}
		}

		drawnRows = totalDrawnRows()

		// Position cursor
		endRow := drawnRows - 1
		targetRow := cursorRowFromTop()
		if endRow > targetRow {
			rawWrite(fmt.Sprintf("\033[%dA", endRow-targetRow))
		}
		targetCol := (2 + col) % tw()
		rawWrite(fmt.Sprintf("\r\033[%dC", targetCol))
	}

	isSlashMode := func() bool {
		return row == 0 && len(lines) == 1 && len(lines[0]) > 0 && lines[0][0] == '/'
	}

	setCursorCol := func() {
		targetCol := 2 + col
		rawWrite(fmt.Sprintf("\r\033[%dC", targetCol%tw()))
	}

	clearMenuLines := func() {
		if menuLines == 0 {
			return
		}
		// Save position, clear menu, restore
		for i := 0; i < menuLines; i++ {
			rawWrite("\r\n\033[K")
		}
		rawWrite(fmt.Sprintf("\033[%dA", menuLines))
		setCursorCol()
		menuLines = 0
		selectedIdx = -1
	}

	drawMenuBelow := func() {
		prefix := string(lines[0])
		matches := filterCommands(prefix)
		clearMenuLines()
		if len(matches) == 0 {
			return
		}
		for i, cmd := range matches {
			rawWrite("\r\n\033[K")
			if i == selectedIdx {
				rawWrite(fmt.Sprintf("    %s%s▸ %-14s%s  %s%s%s", Bold, Cyan, cmd.Name, Reset, Dim, cmd.Desc, Reset))
			} else {
				rawWrite(fmt.Sprintf("      %s%-14s%s  %s%s%s", White, cmd.Name, Reset, Dim, cmd.Desc, Reset))
			}
		}
		rawWrite(fmt.Sprintf("\033[%dA", len(matches)))
		setCursorCol()
		menuLines = len(matches)
	}

	submitResult := func() InputResult {
		var parts []string
		for _, l := range lines {
			parts = append(parts, string(l))
		}
		combined := strings.TrimSpace(strings.Join(parts, "\n"))
		if combined == "" {
			return InputResult{}
		}
		text, imgs := extractImages(combined)
		return InputResult{Text: text, Images: imgs}
	}

	// insertNewline splits the current line at the cursor.
	insertNewline := func() {
		clearMenuLines()
		tail := make([]rune, len(lines[row][col:]))
		copy(tail, lines[row][col:])
		lines[row] = lines[row][:col]
		// Insert new line after current
		newLines := make([][]rune, len(lines)+1)
		copy(newLines, lines[:row+1])
		newLines[row+1] = tail
		copy(newLines[row+2:], lines[row+1:])
		lines = newLines
		row++
		col = 0
		redrawAll()
	}

	// handlePaste processes pasted text, splitting on newlines.
	handlePaste := func(data []byte) {
		clearMenuLines()
		i := 0
		for i < len(data) {
			ch := data[i]

			// CRLF
			if ch == '\r' && i+1 < len(data) && data[i+1] == '\n' {
				tail := make([]rune, len(lines[row][col:]))
				copy(tail, lines[row][col:])
				lines[row] = lines[row][:col]
				newLines := make([][]rune, len(lines)+1)
				copy(newLines, lines[:row+1])
				newLines[row+1] = tail
				copy(newLines[row+2:], lines[row+1:])
				lines = newLines
				row++
				col = 0
				i += 2
				continue
			}

			// CR or LF
			if ch == '\r' || ch == '\n' {
				tail := make([]rune, len(lines[row][col:]))
				copy(tail, lines[row][col:])
				lines[row] = lines[row][:col]
				newLines := make([][]rune, len(lines)+1)
				copy(newLines, lines[:row+1])
				newLines[row+1] = tail
				copy(newLines[row+2:], lines[row+1:])
				lines = newLines
				row++
				col = 0
				i++
				continue
			}

			// Tab → spaces
			if ch == '\t' {
				spaces := []rune("    ")
				lines[row] = append(lines[row][:col], append(spaces, lines[row][col:]...)...)
				col += 4
				i++
				continue
			}

			// Skip control chars
			if ch < 32 {
				i++
				continue
			}

			// Printable / UTF-8
			r, size := utf8.DecodeRune(data[i:])
			if size == 0 {
				i++
				continue
			}
			lines[row] = append(lines[row][:col], append([]rune{r}, lines[row][col:]...)...)
			col++
			i += size
		}
		redrawAll()
	}

	// Initial prompt
	rawWrite(promptStr(0))
	drawnRows = 1

	buf := make([]byte, 256)

	for {
		n, readErr := os.Stdin.Read(buf)
		if readErr != nil || n == 0 {
			break
		}

		b := buf[:n]

		// Handle escape sequences
		if b[0] == 0x1b {
			if n == 1 {
				extra := make([]byte, 8)
				en, _ := os.Stdin.Read(extra)
				if en > 0 {
					b = append(b, extra[:en]...)
					n = len(b)
				} else {
					escPressed = true
					continue
				}
			}

			// Esc + non-CSI byte → set escPressed, replay byte
			if n >= 2 && b[1] != '[' {
				escPressed = true
				replay := make([]byte, n-1)
				copy(replay, b[1:n])
				copy(buf, replay)
				b = buf[:len(replay)]
				n = len(replay)
				goto handleByte
			}

			if n >= 3 && b[1] == '[' {
				seq := string(b[2:n])

				// Shift+Enter
				if seq == "13;2u" || strings.HasPrefix(seq, "27;2;13") {
					insertNewline()
					continue
				}

				// Bracketed paste start
				if strings.HasPrefix(seq, "200~") {
					pasteEnd := []byte("\033[201~")
					pasteBuf := make([]byte, 1024)
					var allData []byte

					// Overflow from first read
					overflow := b[2+len("200~") : n]
					if len(overflow) > 0 {
						if idx := bytes.Index(overflow, pasteEnd); idx >= 0 {
							handlePaste(overflow[:idx])
							continue
						}
						allData = append(allData, overflow...)
					}

					// Read until end marker
					for {
						pn, perr := os.Stdin.Read(pasteBuf)
						if perr != nil || pn == 0 {
							break
						}
						allData = append(allData, pasteBuf[:pn]...)
						if idx := bytes.Index(allData, pasteEnd); idx >= 0 {
							allData = allData[:idx]
							break
						}
					}
					handlePaste(allData)
					continue
				}

				// Bracketed paste end (defensive)
				if strings.HasPrefix(seq, "201~") {
					continue
				}

				switch b[2] {
				case 'A': // Up arrow
					if menuLines > 0 && isSlashMode() {
						matches := filterCommands(string(lines[0]))
						if len(matches) > 0 {
							if selectedIdx <= 0 {
								selectedIdx = len(matches) - 1
							} else {
								selectedIdx--
							}
							drawMenuBelow()
						}
					} else if row > 0 {
						row--
						if col > len(lines[row]) {
							col = len(lines[row])
						}
						redrawAll()
					}
				case 'B': // Down arrow
					if menuLines > 0 && isSlashMode() {
						matches := filterCommands(string(lines[0]))
						if len(matches) > 0 {
							if selectedIdx >= len(matches)-1 {
								selectedIdx = 0
							} else {
								selectedIdx++
							}
							drawMenuBelow()
						}
					} else if row < len(lines)-1 {
						row++
						if col > len(lines[row]) {
							col = len(lines[row])
						}
						redrawAll()
					}
				case 'C': // Right arrow
					if col < len(lines[row]) {
						col++
						redrawAll()
					} else if row < len(lines)-1 {
						row++
						col = 0
						redrawAll()
					}
				case 'D': // Left arrow
					if col > 0 {
						col--
						redrawAll()
					} else if row > 0 {
						row--
						col = len(lines[row])
						redrawAll()
					}
				case 'H': // Home
					col = 0
					redrawAll()
				case 'F': // End
					col = len(lines[row])
					redrawAll()
				case '3': // Delete key
					if n >= 4 && b[3] == '~' {
						if col < len(lines[row]) {
							lines[row] = append(lines[row][:col], lines[row][col+1:]...)
							redrawAll()
						} else if row < len(lines)-1 {
							// Join with next line
							lines[row] = append(lines[row], lines[row+1]...)
							lines = append(lines[:row+1], lines[row+2:]...)
							redrawAll()
						}
					}
				}
			}
			escPressed = false
			continue
		}

	handleByte:
		switch b[0] {
		case 1: // Ctrl+A
			escPressed = false
			col = 0
			redrawAll()

		case 3: // Ctrl+C
			clearMenuLines()
			rawWrite("\r\n")
			restore()
			os.Exit(130)

		case 4: // Ctrl+D
			clearMenuLines()
			rawWrite("\r\n")
			return InputResult{}

		case 5: // Ctrl+E
			escPressed = false
			col = len(lines[row])
			redrawAll()

		case 11: // Ctrl+K — kill to end of line
			escPressed = false
			if col < len(lines[row]) {
				lines[row] = lines[row][:col]
				redrawAll()
			}

		case 12: // Ctrl+L — clear screen
			escPressed = false
			clearMenuLines()
			rawWrite("\033[2J\033[H")
			drawnRows = 0
			redrawAll()
			if isSlashMode() {
				drawMenuBelow()
			}

		case 21: // Ctrl+U — clear line
			escPressed = false
			clearMenuLines()
			lines[row] = nil
			col = 0
			redrawAll()

		case 23: // Ctrl+W — delete word backward
			escPressed = false
			if col > 0 {
				newCol := col
				for newCol > 0 && lines[row][newCol-1] == ' ' {
					newCol--
				}
				for newCol > 0 && lines[row][newCol-1] != ' ' {
					newCol--
				}
				lines[row] = append(lines[row][:newCol], lines[row][col:]...)
				col = newCol
				redrawAll()
			}

		case 9: // Tab — accept completion
			escPressed = false
			if menuLines > 0 && isSlashMode() {
				matches := filterCommands(string(lines[0]))
				if len(matches) == 0 {
					continue
				}
				idx := selectedIdx
				if idx < 0 {
					idx = 0
				}
				clearMenuLines()
				lines[0] = []rune(matches[idx].Name)
				if matches[idx].Name == "/model" || matches[idx].Name == "/simulator" {
					lines[0] = append(lines[0], ' ')
				}
				col = len(lines[0])
				redrawAll()
				if isSlashMode() {
					drawMenuBelow()
				}
			}

		case 13, 10: // Enter
			if escPressed {
				escPressed = false
				insertNewline()
				continue
			}

			// Slash commands
			if isSlashMode() {
				lineStr := strings.TrimSpace(string(lines[0]))
				if menuLines > 0 {
					matches := filterCommands(string(lines[0]))
					if len(matches) > 0 {
						idx := selectedIdx
						if idx < 0 {
							idx = 0
						}
						if idx < len(matches) {
							lineStr = matches[idx].Name
						}
					}
				}
				clearMenuLines()
				rawWrite("\r\n")
				return InputResult{Text: lineStr}
			}

			clearMenuLines()

			// Empty input — re-prompt
			result := submitResult()
			if result.Text == "" && len(result.Images) == 0 {
				rawWrite("\r\n")
				lines = [][]rune{{}}
				row = 0
				col = 0
				drawnRows = 0
				redrawAll()
				continue
			}

			rawWrite("\r\n")
			return result

		case 127, 8: // Backspace
			escPressed = false
			if col > 0 {
				lines[row] = append(lines[row][:col-1], lines[row][col:]...)
				col--
				redrawAll()
				if isSlashMode() {
					drawMenuBelow()
				} else {
					clearMenuLines()
				}
			} else if row > 0 {
				// Join with previous line
				col = len(lines[row-1])
				lines[row-1] = append(lines[row-1], lines[row]...)
				lines = append(lines[:row], lines[row+1:]...)
				row--
				redrawAll()
			}

		default:
			escPressed = false
			if b[0] >= 32 {
				r, size := utf8.DecodeRune(b[:n])
				if size > 0 {
					lines[row] = append(lines[row][:col], append([]rune{r}, lines[row][col:]...)...)
					col++
					// Handle remaining runes in the buffer
					for off := size; off < n; {
						r2, s2 := utf8.DecodeRune(b[off:])
						if s2 == 0 {
							break
						}
						lines[row] = append(lines[row][:col], append([]rune{r2}, lines[row][col:]...)...)
						col++
						off += s2
					}
					redrawAll()
					if isSlashMode() {
						drawMenuBelow()
					} else if menuLines > 0 {
						clearMenuLines()
					}
				}
			}
		}
	}

	return submitResult()
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

// readLineFallback is a simple line reader for non-terminal input.
func readLineFallback() string {
	buf := make([]byte, 4096)
	n, err := os.Stdin.Read(buf)
	if err != nil || n == 0 {
		return ""
	}
	return strings.TrimSpace(string(buf[:n]))
}

// ContinuationPrompt prints the continuation prompt for multi-line input.
func ContinuationPrompt() {
	fmt.Printf("%s  %s", Dim, Reset)
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
