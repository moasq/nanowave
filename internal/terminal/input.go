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

// runeLen returns the number of runes in a byte slice.
func runeLen(b []byte) int {
	return utf8.RuneCount(b)
}

// runeIndex returns the byte offset of the i-th rune in b.
func runeIndex(b []byte, i int) int {
	off := 0
	for j := 0; j < i; j++ {
		_, size := utf8.DecodeRune(b[off:])
		off += size
	}
	return off
}

// ReadInput reads input from the terminal with slash command completion.
// Single Enter submits. Shift+Enter (or Esc then Enter) adds a newline.
// Image file paths (dragged/pasted) are detected and returned separately.
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

	// Enable bracketed paste mode — terminals that support it will wrap
	// pasted text in ESC[200~ … ESC[201~ markers so we can detect pastes.
	os.Stdout.WriteString("\033[?2004h")

	restore := func() {
		os.Stdout.WriteString("\033[?2004l") // disable bracketed paste
		term.Restore(fd, oldState)
	}
	defer restore()

	var lines []string
	var currentLine []byte
	cursorPos := 0 // cursor position in runes
	first := true
	menuLines := 0
	selectedIdx := -1
	escPressed := false // track Esc key for Esc+Enter newline
	prevVisualLines := 1 // tracks wrapped visual lines for redraw clearing

	// promptWidth returns the visible character width of the prompt.
	promptWidth := func() int {
		return 2 // "> " or "  "
	}

	printPrompt := func() {
		if first {
			rawWrite(Bold + "> " + Reset)
		} else {
			rawWrite(Dim + "  " + Reset)
		}
	}

	// termWidth returns the current terminal width, defaulting to 80.
	termWidth := func() int {
		w, _, err := term.GetSize(fd)
		if err != nil || w <= 0 {
			return 80
		}
		return w
	}

	redrawLine := func() {
		tw := termWidth()

		// Move cursor up to the first visual line of the previous content
		if prevVisualLines > 1 {
			rawWrite(fmt.Sprintf("\033[%dA", prevVisualLines-1))
		}
		// Clear from the first visual line downward (clears all wrapped lines)
		rawWrite("\r\033[J")

		printPrompt()
		rawWrite(string(currentLine))

		// Update visual line count for next redraw
		contentLen := promptWidth() + runeLen(currentLine)
		prevVisualLines = 1
		if tw > 0 && contentLen > tw {
			prevVisualLines = (contentLen + tw - 1) / tw
		}

		// Move cursor to correct position if not at end.
		// Use absolute row/col positioning since \033[D doesn't cross line wraps.
		totalRunes := runeLen(currentLine)
		if cursorPos < totalRunes {
			cursorAbs := promptWidth() + cursorPos
			endAbs := promptWidth() + totalRunes

			cursorRow := cursorAbs / tw
			endRow := endAbs / tw
			// If endAbs lands exactly on a boundary, the cursor is at col 0 of
			// the next visual line, but the terminal may not have scrolled yet.
			if endAbs > 0 && endAbs%tw == 0 {
				endRow--
			}

			// Move up from end row to cursor row
			if endRow > cursorRow {
				rawWrite(fmt.Sprintf("\033[%dA", endRow-cursorRow))
			}
			// Move to exact column
			col := cursorAbs % tw
			rawWrite(fmt.Sprintf("\r\033[%dC", col))
		}
	}

	// setCursorCol positions the terminal cursor at the right column.
	setCursorCol := func() {
		col := promptWidth() + cursorPos
		rawWrite(fmt.Sprintf("\r\033[%dC", col))
	}

	// clearMenuLines wipes all menu lines below the input line.
	clearMenuLines := func() {
		if menuLines == 0 {
			return
		}
		for i := 0; i < menuLines; i++ {
			rawWrite("\r\n\033[K")
		}
		rawWrite(fmt.Sprintf("\033[%dA", menuLines))
		setCursorCol()
		menuLines = 0
		selectedIdx = -1
	}

	// drawMenuBelow draws the completion menu below the current input line.
	drawMenuBelow := func() {
		prefix := string(currentLine)
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

	isSlashMode := func() bool {
		return first && len(currentLine) > 0 && currentLine[0] == '/'
	}

	submitResult := func() InputResult {
		lineStr := strings.TrimSpace(string(currentLine))
		if lineStr != "" {
			lines = append(lines, lineStr)
		}
		combined := strings.TrimSpace(strings.Join(lines, "\n"))
		text, imgs := extractImages(combined)
		return InputResult{Text: text, Images: imgs}
	}

	// processPasteBytes handles raw bytes received during a bracketed paste.
	// Newlines become new lines (with continuation prompt), tabs expand to
	// spaces, printable characters are inserted at the cursor, and control
	// characters are ignored.
	processPasteBytes := func(data []byte) {
		i := 0
		for i < len(data) {
			ch := data[i]

			// CRLF → single newline
			if ch == '\r' && i+1 < len(data) && data[i+1] == '\n' {
				lineStr := string(currentLine)
				lines = append(lines, lineStr)
				currentLine = nil
				cursorPos = 0
				prevVisualLines = 1
				first = false
				rawWrite("\r\n")
				printPrompt()
				i += 2
				continue
			}

			// CR or LF → newline
			if ch == '\r' || ch == '\n' {
				lineStr := string(currentLine)
				lines = append(lines, lineStr)
				currentLine = nil
				cursorPos = 0
				prevVisualLines = 1
				first = false
				rawWrite("\r\n")
				printPrompt()
				i++
				continue
			}

			// Tab → 4 spaces
			if ch == '\t' {
				spaces := []byte("    ")
				bytePos := runeIndex(currentLine, cursorPos)
				tmp := make([]byte, len(currentLine)+len(spaces))
				copy(tmp, currentLine[:bytePos])
				copy(tmp[bytePos:], spaces)
				copy(tmp[bytePos+len(spaces):], currentLine[bytePos:])
				currentLine = tmp
				cursorPos += 4
				i++
				continue
			}

			// Skip control characters (except those handled above)
			if ch < 32 {
				i++
				continue
			}

			// Printable / UTF-8 character
			_, size := utf8.DecodeRune(data[i:])
			if size == 0 {
				i++
				continue
			}
			chunk := data[i : i+size]
			bytePos := runeIndex(currentLine, cursorPos)
			tmp := make([]byte, len(currentLine)+size)
			copy(tmp, currentLine[:bytePos])
			copy(tmp[bytePos:], chunk)
			copy(tmp[bytePos+size:], currentLine[bytePos:])
			currentLine = tmp
			cursorPos++
			i += size
		}
	}

	printPrompt()

	buf := make([]byte, 256)

	for {
		n, readErr := os.Stdin.Read(buf)
		if readErr != nil || n == 0 {
			break
		}

		b := buf[:n]

		// Handle escape sequences (arrow keys etc.)
		if b[0] == 0x1b {
			if n == 1 {
				// Single Esc byte — try to read more to distinguish
				// standalone Esc from the start of an escape sequence.
				extra := make([]byte, 8)
				en, _ := os.Stdin.Read(extra)
				if en > 0 {
					b = append(b, extra[:en]...)
					n = len(b)
				} else {
					// Standalone Esc — set flag for Esc+Enter
					escPressed = true
					continue
				}
			}

			// If the byte after Esc is not '[', this is Esc+<key>.
			// Set escPressed and re-process the remaining bytes as input.
			if n >= 2 && b[1] != '[' {
				escPressed = true
				// Feed the non-Esc bytes back into the main switch below
				remaining := b[1:n]
				replay := make([]byte, len(remaining))
				copy(replay, remaining)
				copy(buf, replay)
				b = buf[:len(replay)]
				n = len(replay)
				// DO NOT continue — fall through to the main switch
				goto handleByte
			}
			if n >= 3 && b[1] == '[' {
				seq := string(b[2:n])

				// Shift+Enter: ESC[13;2u (kitty) or ESC[27;2;13~ (xterm)
				if seq == "13;2u" || strings.HasPrefix(seq, "27;2;13") {
					// Insert newline (same as Esc+Enter)
					clearMenuLines()
					lineStr := string(currentLine)
					lines = append(lines, lineStr)
					currentLine = nil
					cursorPos = 0
					prevVisualLines = 1
					first = false
					rawWrite("\r\n")
					printPrompt()
					continue
				}

				// Bracketed paste start: ESC[200~
				// The marker and pasted content may arrive in the same read,
				// so use HasPrefix and process any trailing bytes as paste data.
				if strings.HasPrefix(seq, "200~") {
					clearMenuLines()

					pasteEnd := []byte("\033[201~")
					pasteBuf := make([]byte, 1024)
					var trailer []byte // holds partial end-marker bytes across reads

					// Process any bytes that arrived in the same read as the start marker
					overflow := b[2+len("200~") : n]
					if len(overflow) > 0 {
						// Check if the end marker is already in this first chunk
						if idx := bytes.Index(overflow, pasteEnd); idx >= 0 {
							if idx > 0 {
								processPasteBytes(overflow[:idx])
							}
							redrawLine()
							continue
						}
						// Seed the trailer with overflow for the paste loop
						trailer = make([]byte, len(overflow))
						copy(trailer, overflow)
					}

				pasteLoop:
					for {
						pn, perr := os.Stdin.Read(pasteBuf)
						if perr != nil || pn == 0 {
							break
						}

						// Prepend any trailer from previous read
						var chunk []byte
						if len(trailer) > 0 {
							chunk = append(trailer, pasteBuf[:pn]...)
							trailer = nil
						} else {
							chunk = pasteBuf[:pn]
						}

						// Scan for end marker ESC[201~
						if idx := bytes.Index(chunk, pasteEnd); idx >= 0 {
							// Process everything before the end marker
							if idx > 0 {
								processPasteBytes(chunk[:idx])
							}
							break pasteLoop
						}

						// The end marker (6 bytes) might be split across reads.
						// Keep up to 5 trailing bytes as a trailer for next read.
						safeLen := len(chunk)
						trailerSize := len(pasteEnd) - 1 // 5
						if safeLen > trailerSize {
							processPasteBytes(chunk[:safeLen-trailerSize])
							trailer = make([]byte, trailerSize)
							copy(trailer, chunk[safeLen-trailerSize:])
						} else {
							// Entire chunk is shorter than marker; hold it all
							trailer = make([]byte, len(chunk))
							copy(trailer, chunk)
						}
					}

					// Process any remaining trailer that wasn't an end marker
					if len(trailer) > 0 {
						processPasteBytes(trailer)
					}

					redrawLine()
					continue
				}

				// Bracketed paste end (defensive; normally consumed in paste loop)
				if strings.HasPrefix(seq, "201~") {
					continue
				}

				switch b[2] {
				case 'A': // Up arrow
					if menuLines > 0 {
						matches := filterCommands(string(currentLine))
						if len(matches) > 0 {
							if selectedIdx <= 0 {
								selectedIdx = len(matches) - 1
							} else {
								selectedIdx--
							}
							drawMenuBelow()
						}
					}
				case 'B': // Down arrow
					if menuLines > 0 {
						matches := filterCommands(string(currentLine))
						if len(matches) > 0 {
							if selectedIdx >= len(matches)-1 {
								selectedIdx = 0
							} else {
								selectedIdx++
							}
							drawMenuBelow()
						}
					}
				case 'C': // Right arrow
					if cursorPos < runeLen(currentLine) {
						cursorPos++
						rawWrite("\033[C")
					}
				case 'D': // Left arrow
					if cursorPos > 0 {
						cursorPos--
						rawWrite("\033[D")
					}
				case 'H': // Home
					cursorPos = 0
					setCursorCol()
				case 'F': // End
					cursorPos = runeLen(currentLine)
					setCursorCol()
				case '3': // Delete key (ESC [ 3 ~)
					if n >= 4 && b[3] == '~' {
						totalRunes := runeLen(currentLine)
						if cursorPos < totalRunes {
							bytePos := runeIndex(currentLine, cursorPos)
							_, size := utf8.DecodeRune(currentLine[bytePos:])
							currentLine = append(currentLine[:bytePos], currentLine[bytePos+size:]...)
							redrawLine()
							if isSlashMode() {
								drawMenuBelow()
							} else {
								clearMenuLines()
							}
						}
					}
				}
			}
			escPressed = false
			continue
		}

	handleByte:
		switch b[0] {
		case 1: // Ctrl+A — move to beginning of line
			escPressed = false
			cursorPos = 0
			setCursorCol()

		case 3: // Ctrl+C
			clearMenuLines()
			rawWrite("\r\n")
			restore()
			os.Exit(130)

		case 4: // Ctrl+D
			clearMenuLines()
			rawWrite("\r\n")
			return InputResult{}

		case 5: // Ctrl+E — move to end of line
			escPressed = false
			cursorPos = runeLen(currentLine)
			setCursorCol()

		case 11: // Ctrl+K — kill from cursor to end of line
			escPressed = false
			if cursorPos < runeLen(currentLine) {
				bytePos := runeIndex(currentLine, cursorPos)
				currentLine = currentLine[:bytePos]
				redrawLine()
				if isSlashMode() {
					drawMenuBelow()
				} else {
					clearMenuLines()
				}
			}

		case 12: // Ctrl+L — clear screen
			escPressed = false
			clearMenuLines()
			rawWrite("\033[2J\033[H") // clear screen and move to top
			redrawLine()
			if isSlashMode() {
				drawMenuBelow()
			}

		case 21: // Ctrl+U — clear line
			escPressed = false
			clearMenuLines()
			currentLine = nil
			cursorPos = 0
			redrawLine()

		case 23: // Ctrl+W — delete word backward
			escPressed = false
			if cursorPos > 0 {
				// Work backward from cursor: skip trailing spaces, then delete to next space
				runes := []rune(string(currentLine))
				newPos := cursorPos
				// Skip spaces
				for newPos > 0 && runes[newPos-1] == ' ' {
					newPos--
				}
				// Skip non-spaces
				for newPos > 0 && runes[newPos-1] != ' ' {
					newPos--
				}
				// Remove runes from newPos to cursorPos
				runes = append(runes[:newPos], runes[cursorPos:]...)
				currentLine = []byte(string(runes))
				cursorPos = newPos
				redrawLine()
				if isSlashMode() {
					drawMenuBelow()
				} else {
					clearMenuLines()
				}
			}

		case 9: // Tab — accept completion
			escPressed = false
			if menuLines > 0 {
				matches := filterCommands(string(currentLine))
				if len(matches) == 0 {
					continue
				}
				idx := selectedIdx
				if idx < 0 {
					idx = 0
				}
				clearMenuLines()
				currentLine = []byte(matches[idx].Name)
				if matches[idx].Name == "/model" || matches[idx].Name == "/simulator" {
					currentLine = append(currentLine, ' ')
				}
				cursorPos = runeLen(currentLine)
				redrawLine()
				if isSlashMode() {
					drawMenuBelow()
				}
			}

		case 13, 10: // Enter
			// Esc+Enter → insert newline (multi-line mode)
			if escPressed {
				escPressed = false
				clearMenuLines()
				lineStr := string(currentLine)
				lines = append(lines, lineStr)
				currentLine = nil
				cursorPos = 0
				prevVisualLines = 1
				first = false
				rawWrite("\r\n")
				printPrompt()
				continue
			}

			lineStr := strings.TrimSpace(string(currentLine))

			// Slash commands: submit on single Enter
			if strings.HasPrefix(lineStr, "/") && first {
				if menuLines > 0 {
					matches := filterCommands(string(currentLine))
					if len(matches) > 0 {
						idx := selectedIdx
						if idx < 0 {
							idx = 0 // auto-select first match
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

			// Empty Enter with no accumulated text — just re-prompt
			if lineStr == "" && len(lines) == 0 {
				rawWrite("\r\n")
				first = true
				currentLine = nil
				cursorPos = 0
				prevVisualLines = 1
				printPrompt()
				continue
			}

			// Submit on Enter (single Enter submits)
			rawWrite("\r\n")
			return submitResult()

		case 127, 8: // Backspace
			escPressed = false
			if cursorPos > 0 {
				bytePos := runeIndex(currentLine, cursorPos)
				_, size := utf8.DecodeLastRune(currentLine[:bytePos])
				currentLine = append(currentLine[:bytePos-size], currentLine[bytePos:]...)
				cursorPos--
				redrawLine()
				if isSlashMode() {
					drawMenuBelow()
				} else {
					clearMenuLines()
				}
			}

		default:
			escPressed = false
			if b[0] >= 32 {
				// Insert at cursor position, not just append
				bytePos := runeIndex(currentLine, cursorPos)
				insert := make([]byte, len(currentLine)+n)
				copy(insert, currentLine[:bytePos])
				copy(insert[bytePos:], b[:n])
				copy(insert[bytePos+n:], currentLine[bytePos:])
				currentLine = insert
				cursorPos += utf8.RuneCount(b[:n])
				redrawLine()

				if isSlashMode() {
					drawMenuBelow()
				} else if menuLines > 0 {
					clearMenuLines()
				}
			}
		}
	}

	if len(currentLine) > 0 {
		lines = append(lines, strings.TrimSpace(string(currentLine)))
	}
	combined := strings.TrimSpace(strings.Join(lines, "\n"))
	text, imgs := extractImages(combined)
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
