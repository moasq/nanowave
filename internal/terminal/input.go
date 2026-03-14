package terminal

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
	"golang.org/x/term"
)

// SlashCommands is the list of available commands for autocomplete.
var SlashCommands = []CommandInfo{
	{Name: "/run", Desc: "Build and launch the app"},
	{Name: "/simulator", Desc: "Select simulator device"},
	{Name: "/agent", Desc: "Show or switch AI runtime"},
	{Name: "/model", Desc: "Show or switch model"},
	{Name: "/fix", Desc: "Auto-fix build errors"},
	{Name: "/connect", Desc: "App Store Connect (publish, TestFlight, metadata)"},
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

type inputToken struct {
	start int
	end   int
	value string
}

// imageExtensions are file extensions recognized as images.
var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".webp": true, ".bmp": true, ".tiff": true, ".tif": true,
	".heic": true, ".heif": true, ".svg": true,
}

var imageIndicatorPattern = regexp.MustCompile(`\[(?i:image)\d+\]\s*`)

const (
	inputBoxBackground        = "\033[48;5;236m"
	inputBoxForeground        = "\033[38;5;252m"
	inputBoxHorizontalPadding = 1
	inputBoxVerticalPadding   = 1
	inputBoxVisibleRows       = 5
	inputBoxHelperRows        = 3
	inputBoxMinContentWidth   = 20
)

var errInputInterrupted = errors.New("input interrupted")

type editorEventKind int

const (
	editorEventNone editorEventKind = iota
	editorEventInsert
	editorEventBackspace
	editorEventDelete
	editorEventMoveLeft
	editorEventMoveRight
	editorEventMoveUp
	editorEventMoveDown
	editorEventMoveHome
	editorEventMoveEnd
	editorEventClearToStart
	editorEventClearToEnd
	editorEventDeleteWord
	editorEventSubmit
	editorEventInterrupt
	editorEventClipboardImage
)

type editorEvent struct {
	kind editorEventKind
	text string
}

type cursorCoord struct {
	row int
	col int
}

type editorLayout struct {
	lines     []string
	positions []cursorCoord
}

type inputEditor struct {
	fd             int
	width          int
	padding        int
	contentWidth   int
	maxVisibleRows int
	helperRows     int
	regionRows     int
	buffer         []rune
	cursor         int
	preferredCol   int
	scrollTop      int
	backgroundLine string
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
	tokens := scanInputTokens(input)
	if len(tokens) == 0 {
		if strings.TrimSpace(input) == "" {
			return "", nil
		}
		return input, nil
	}

	type removal struct {
		start int
		end   int
	}

	var removals []removal
	var images []string
	seen := make(map[string]struct{})

	for _, token := range tokens {
		if resolved, ok := resolveImageReference(token.value); ok {
			if _, exists := seen[resolved]; !exists {
				images = append(images, resolved)
				seen[resolved] = struct{}{}
			}
			removals = append(removals, removal{start: token.start, end: token.end})
		}
	}

	if len(images) == 0 {
		return input, nil
	}

	var sb strings.Builder
	last := 0
	for _, removal := range removals {
		if removal.start > last {
			sb.WriteString(input[last:removal.start])
		}
		last = removal.end
	}
	if last < len(input) {
		sb.WriteString(input[last:])
	}

	text := sb.String()
	if strings.TrimSpace(text) == "" {
		return "", images
	}
	return text, images
}

func scanInputTokens(input string) []inputToken {
	var tokens []inputToken

	for i := 0; i < len(input); {
		r, width := utf8DecodeRuneInString(input[i:])
		if unicode.IsSpace(r) {
			i += width
			continue
		}

		start := i
		var sb strings.Builder
		var quote rune

		for i < len(input) {
			r, width = utf8DecodeRuneInString(input[i:])

			if quote != 0 {
				switch {
				case r == quote:
					quote = 0
					i += width
				case r == '\\' && quote == '"':
					next, nextWidth := utf8DecodeRuneInString(input[i+width:])
					if nextWidth == 0 {
						i += width
						continue
					}
					sb.WriteRune(next)
					i += width + nextWidth
				default:
					sb.WriteRune(r)
					i += width
				}
				continue
			}

			switch {
			case unicode.IsSpace(r):
				tokens = append(tokens, inputToken{start: start, end: i, value: sb.String()})
				goto nextToken
			case r == '"' || r == '\'':
				quote = r
				i += width
			case r == '\\':
				next, nextWidth := utf8DecodeRuneInString(input[i+width:])
				if nextWidth == 0 {
					i += width
					continue
				}
				sb.WriteRune(next)
				i += width + nextWidth
			default:
				sb.WriteRune(r)
				i += width
			}
		}

		tokens = append(tokens, inputToken{start: start, end: len(input), value: sb.String()})
		break

	nextToken:
	}

	return tokens
}

func utf8DecodeRuneInString(s string) (rune, int) {
	if s == "" {
		return 0, 0
	}
	r := rune(s[0])
	if r < utf8.RuneSelf {
		return r, 1
	}
	return utf8.DecodeRuneInString(s)
}

func resolveImageReference(raw string) (string, bool) {
	candidates := []string{strings.TrimSpace(raw)}
	if trimmed := strings.TrimRight(candidates[0], ".,;:!?"); trimmed != candidates[0] {
		candidates = append(candidates, trimmed)
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if resolved, ok := resolveImagePathReference(candidate); ok {
			return resolved, true
		}
	}
	return "", false
}

func resolveImagePathReference(candidate string) (string, bool) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return "", false
	}

	if strings.HasPrefix(candidate, "file://") {
		parsed, err := url.Parse(candidate)
		if err != nil {
			return "", false
		}
		if parsed.Scheme != "file" {
			return "", false
		}
		if parsed.Host != "" && parsed.Host != "localhost" {
			return "", false
		}
		unescaped, err := url.PathUnescape(parsed.Path)
		if err != nil {
			return "", false
		}
		candidate = unescaped
	}

	if !isImagePath(candidate) {
		return "", false
	}
	return resolveImagePath(candidate), true
}

func newInputEditor(fd int) *inputEditor {
	width, _, err := term.GetSize(fd)
	if err != nil || width <= 0 {
		width = 80
	}

	padding, contentWidth := inputBoxMetrics(width)
	visibleRows := inputBoxVisibleRows
	if visibleRows > width/8 {
		visibleRows = maxInt(3, width/8)
	}

	return &inputEditor{
		fd:             fd,
		width:          width,
		padding:        padding,
		contentWidth:   contentWidth,
		maxVisibleRows: maxInt(3, visibleRows),
		helperRows:     inputBoxHelperRows,
		preferredCol:   -1,
	}
}

func inputBoxMetrics(width int) (padding int, contentWidth int) {
	padding = inputBoxHorizontalPadding
	maxPadding := (width - inputBoxMinContentWidth) / 2
	if maxPadding < 1 {
		maxPadding = 1
	}
	if padding > maxPadding {
		padding = maxPadding
	}
	contentWidth = width - (padding * 2)
	if contentWidth < 1 {
		contentWidth = 1
	}
	return padding, contentWidth
}

func normalizeEditorText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\t", "    ")
	return text
}

func layoutEditorBuffer(buffer []rune, contentWidth int) editorLayout {
	if contentWidth < 1 {
		contentWidth = 1
	}

	positions := make([]cursorCoord, len(buffer)+1)
	lines := make([]string, 0, 1)
	var line strings.Builder
	row, col := 0, 0
	positions[0] = cursorCoord{row: 0, col: 0}

	for i, r := range buffer {
		if r == '\n' {
			positions[i] = cursorCoord{row: row, col: col}
			lines = append(lines, line.String())
			line.Reset()
			row++
			col = 0
			positions[i+1] = cursorCoord{row: row, col: col}
			continue
		}

		width := runeDisplayWidth(r)
		if col > 0 && col+width > contentWidth {
			lines = append(lines, line.String())
			line.Reset()
			row++
			col = 0
		}

		positions[i] = cursorCoord{row: row, col: col}
		line.WriteRune(r)
		col += width
		positions[i+1] = cursorCoord{row: row, col: col}
	}

	lines = append(lines, line.String())
	if len(lines) == 0 {
		lines = append(lines, "")
	}

	return editorLayout{lines: lines, positions: positions}
}

func (l editorLayout) indexForPosition(targetRow, targetCol int) int {
	lastOnRow := -1
	best := -1
	bestCol := -1

	for i, pos := range l.positions {
		if pos.row < targetRow {
			continue
		}
		if pos.row > targetRow {
			break
		}

		lastOnRow = i
		if pos.col <= targetCol && pos.col >= bestCol {
			best = i
			bestCol = pos.col
		}
		if pos.col >= targetCol {
			if pos.col == targetCol || best == -1 {
				return i
			}
			return best
		}
	}

	if best != -1 {
		return best
	}
	if lastOnRow != -1 {
		return lastOnRow
	}
	return 0
}

func runeDisplayWidth(r rune) int {
	width := uniseg.StringWidth(string(r))
	if width <= 0 && r != 0 {
		return 1
	}
	return width
}

func trimDisplayWidth(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	var out strings.Builder
	width := 0
	graphemes := uniseg.NewGraphemes(text)
	for graphemes.Next() {
		cluster := graphemes.Str()
		clusterWidth := graphemes.Width()
		if clusterWidth <= 0 {
			clusterWidth = uniseg.StringWidth(cluster)
		}
		if width+clusterWidth > maxWidth {
			break
		}
		out.WriteString(cluster)
		width += clusterWidth
	}
	return out.String()
}

func (e *inputEditor) renderBoxLine(content string) string {
	content = trimDisplayWidth(content, e.contentWidth)
	contentWidth := uniseg.StringWidth(content)
	rightPadding := e.width - e.padding - contentWidth
	if rightPadding < 0 {
		rightPadding = 0
	}

	return inputBoxBackground +
		inputBoxForeground +
		strings.Repeat(" ", e.padding) +
		content +
		strings.Repeat(" ", rightPadding) +
		Reset
}

func (e *inputEditor) renderBlankBoxLine() string {
	return e.renderBoxLine("")
}

func currentCommandPrefix(text string, cursor int) string {
	buffer := []rune(text)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(buffer) {
		cursor = len(buffer)
	}

	prefix := strings.TrimSpace(string(buffer[:cursor]))
	if prefix == "" || strings.ContainsAny(prefix, " \n\t") || !strings.HasPrefix(prefix, "/") {
		return ""
	}
	return prefix
}

func (e *inputEditor) helperLines() []string {
	text := string(e.buffer)
	lines := make([]string, 0, e.helperRows)
	if e.backgroundLine != "" {
		bg := strings.Repeat(" ", e.padding) + e.backgroundLine
		lines = append(lines, trimDisplayWidth(bg, e.width))
	}
	status := fmt.Sprintf("%s%sEnter submit%s  %s•%s  %sCtrl+V attach image%s  %s•%s  %sCtrl+U/K clear%s",
		strings.Repeat(" ", e.padding),
		Dim, Reset,
		Dim, Reset,
		Dim, Reset,
		Dim, Reset,
		Dim, Reset)

	layout := layoutEditorBuffer(e.buffer, e.contentWidth)
	boxRows := e.boxRows(layout)
	if len(layout.lines) > boxRows {
		start := e.scrollTop + 1
		end := minInt(len(layout.lines), e.scrollTop+boxRows)
		status += fmt.Sprintf("  %s•%s  %sLines %d-%d of %d%s", Dim, Reset, Dim, start, end, len(layout.lines), Reset)
	}
	lines = append(lines, trimDisplayWidth(status, e.width))

	prefix := currentCommandPrefix(text, e.cursor)
	if prefix != "" {
		matches := filterCommands(prefix)
		for i := 0; i < minInt(len(matches), e.helperRows-1); i++ {
			line := fmt.Sprintf("%s%s%-10s%s %s",
				strings.Repeat(" ", e.padding),
				Cyan, matches[i].Name, Reset,
				matches[i].Desc)
			lines = append(lines, trimDisplayWidth(line, e.width))
		}
	}

	for len(lines) < e.helperRows {
		lines = append(lines, "")
	}
	return lines[:e.helperRows]
}

func (e *inputEditor) boxRows(layout editorLayout) int {
	rows := len(layout.lines)
	if rows < 1 {
		rows = 1
	}
	if rows > e.maxVisibleRows {
		rows = e.maxVisibleRows
	}
	return rows
}

func (e *inputEditor) totalRows(boxRows int) int {
	return inputBoxVerticalPadding*2 + boxRows + e.helperRows
}

func (e *inputEditor) ensureCursorVisible(layout editorLayout, boxRows int) {
	cursorRow := 0
	if e.cursor >= 0 && e.cursor < len(layout.positions) {
		cursorRow = layout.positions[e.cursor].row
	}

	if cursorRow < e.scrollTop {
		e.scrollTop = cursorRow
	}
	if cursorRow >= e.scrollTop+boxRows {
		e.scrollTop = cursorRow - boxRows + 1
	}

	maxScroll := maxInt(0, len(layout.lines)-boxRows)
	if e.scrollTop > maxScroll {
		e.scrollTop = maxScroll
	}
	if e.scrollTop < 0 {
		e.scrollTop = 0
	}
}

func (e *inputEditor) render() {
	withTerminalOutputLock(func() {
		e.renderLocked()
	})
}

func (e *inputEditor) renderLocked() {
	layout := layoutEditorBuffer(e.buffer, e.contentWidth)
	boxRows := e.boxRows(layout)
	e.ensureCursorVisible(layout, boxRows)
	e.adjustReservedRowsLocked(e.totalRows(boxRows))

	lines := make([]string, 0, e.regionRows)
	for i := 0; i < inputBoxVerticalPadding; i++ {
		lines = append(lines, e.renderBlankBoxLine())
	}
	for i := 0; i < boxRows; i++ {
		idx := e.scrollTop + i
		content := ""
		if idx < len(layout.lines) {
			content = layout.lines[idx]
		}
		lines = append(lines, e.renderBoxLine(content))
	}
	for i := 0; i < inputBoxVerticalPadding; i++ {
		lines = append(lines, e.renderBlankBoxLine())
	}
	lines = append(lines, e.helperLines()...)

	writeStdoutUnlocked("\033[?25l")
	writeStdoutUnlocked("\033[u")
	clearReservedRowsLocked(e.regionRows)
	for i, line := range lines {
		writeStdoutUnlocked(line)
		if i < len(lines)-1 {
			writeStdoutUnlocked("\r\n")
		}
	}

	cursor := layout.positions[e.cursor]
	cursorRow := inputBoxVerticalPadding + (cursor.row - e.scrollTop)
	cursorCol := e.padding + cursor.col
	if cursorRow < inputBoxVerticalPadding {
		cursorRow = inputBoxVerticalPadding
	}
	if cursorRow >= inputBoxVerticalPadding+boxRows {
		cursorRow = inputBoxVerticalPadding + boxRows - 1
	}

	writeStdoutUnlocked("\033[u")
	if cursorRow > 0 {
		writeStdoutUnlocked(fmt.Sprintf("\033[%dB", cursorRow))
	}
	writeStdoutUnlocked("\r")
	if cursorCol > 0 {
		writeStdoutUnlocked(fmt.Sprintf("\033[%dC", cursorCol))
	}
	writeStdoutUnlocked("\033[?25h")
}

func clearReservedRowsLocked(rows int) {
	if rows <= 0 {
		return
	}
	for i := 0; i < rows; i++ {
		writeStdoutUnlocked("\r\033[2K")
		if i < rows-1 {
			writeStdoutUnlocked("\n")
		}
	}
	if rows > 1 {
		writeStdoutUnlocked(fmt.Sprintf("\033[%dA", rows-1))
	}
	writeStdoutUnlocked("\r")
}

func (e *inputEditor) adjustReservedRowsLocked(rows int) {
	if rows < 1 {
		rows = 1
	}
	if rows == e.regionRows {
		return
	}

	writeStdoutUnlocked("\033[u\r")
	switch {
	case rows > e.regionRows:
		writeStdoutUnlocked(fmt.Sprintf("\033[%dL", rows-e.regionRows))
	case rows < e.regionRows:
		writeStdoutUnlocked(fmt.Sprintf("\033[%dM", e.regionRows-rows))
	}
	e.regionRows = rows
}

func (e *inputEditor) reserveRegionLocked() {
	writeStdoutUnlocked("\r\033[s")
	writeStdoutUnlocked("\033[?2004h")
}

func (e *inputEditor) cleanupRegionLocked() {
	writeStdoutUnlocked("\033[u")
	clearReservedRowsLocked(e.regionRows)
	writeStdoutUnlocked(fmt.Sprintf("\033[%dM", e.regionRows))
	writeStdoutUnlocked("\r")
	writeStdoutUnlocked("\033[?2004l")
	writeStdoutUnlocked("\033[?25h")
}

func (e *inputEditor) pushBackgroundLineLocked(line string) {
	line = sanitizeInlineTerminalText(line)
	if line == "" {
		return
	}
	e.backgroundLine = line
}

func (e *inputEditor) insertText(text string) {
	text = normalizeEditorText(text)
	if text == "" {
		return
	}

	inserted := []rune(text)
	if e.cursor < 0 {
		e.cursor = 0
	}
	if e.cursor > len(e.buffer) {
		e.cursor = len(e.buffer)
	}

	next := make([]rune, 0, len(e.buffer)+len(inserted))
	next = append(next, e.buffer[:e.cursor]...)
	next = append(next, inserted...)
	next = append(next, e.buffer[e.cursor:]...)
	e.buffer = next
	e.cursor += len(inserted)
	e.preferredCol = -1
}

func (e *inputEditor) backspace() {
	if e.cursor <= 0 || len(e.buffer) == 0 {
		return
	}
	e.buffer = append(e.buffer[:e.cursor-1], e.buffer[e.cursor:]...)
	e.cursor--
	e.preferredCol = -1
}

func (e *inputEditor) delete() {
	if e.cursor < 0 || e.cursor >= len(e.buffer) {
		return
	}
	e.buffer = append(e.buffer[:e.cursor], e.buffer[e.cursor+1:]...)
	e.preferredCol = -1
}

func (e *inputEditor) moveLeft() {
	if e.cursor > 0 {
		e.cursor--
	}
	e.preferredCol = -1
}

func (e *inputEditor) moveRight() {
	if e.cursor < len(e.buffer) {
		e.cursor++
	}
	e.preferredCol = -1
}

func (e *inputEditor) moveHome() {
	layout := layoutEditorBuffer(e.buffer, e.contentWidth)
	if e.cursor < len(layout.positions) {
		row := layout.positions[e.cursor].row
		e.cursor = layout.indexForPosition(row, 0)
	}
	e.preferredCol = -1
}

func (e *inputEditor) moveEnd() {
	layout := layoutEditorBuffer(e.buffer, e.contentWidth)
	if e.cursor < len(layout.positions) {
		row := layout.positions[e.cursor].row
		e.cursor = layout.indexForPosition(row, e.contentWidth)
	}
	e.preferredCol = -1
}

func (e *inputEditor) moveVertical(delta int) {
	layout := layoutEditorBuffer(e.buffer, e.contentWidth)
	if e.cursor >= len(layout.positions) {
		e.cursor = len(layout.positions) - 1
	}
	current := layout.positions[e.cursor]
	targetRow := current.row + delta
	if targetRow < 0 || targetRow >= len(layout.lines) {
		return
	}

	if e.preferredCol < 0 {
		e.preferredCol = current.col
	}
	e.cursor = layout.indexForPosition(targetRow, e.preferredCol)
}

func (e *inputEditor) clearToStart() {
	if e.cursor <= 0 {
		return
	}

	start := 0
	for i := e.cursor - 1; i >= 0; i-- {
		if e.buffer[i] == '\n' {
			start = i + 1
			break
		}
	}
	if start >= e.cursor {
		return
	}

	e.buffer = append(e.buffer[:start], e.buffer[e.cursor:]...)
	e.cursor = start
	e.preferredCol = -1
}

func (e *inputEditor) clearToEnd() {
	if e.cursor >= len(e.buffer) {
		return
	}

	end := len(e.buffer)
	for i := e.cursor; i < len(e.buffer); i++ {
		if e.buffer[i] == '\n' {
			end = i
			break
		}
	}
	if end <= e.cursor {
		return
	}

	e.buffer = append(e.buffer[:e.cursor], e.buffer[end:]...)
	e.preferredCol = -1
}

func (e *inputEditor) deleteWordBackward() {
	if e.cursor <= 0 {
		return
	}

	start := e.cursor
	for start > 0 && unicode.IsSpace(e.buffer[start-1]) && e.buffer[start-1] != '\n' {
		start--
	}
	for start > 0 && !unicode.IsSpace(e.buffer[start-1]) {
		start--
	}
	if start >= e.cursor {
		return
	}

	e.buffer = append(e.buffer[:start], e.buffer[e.cursor:]...)
	e.cursor = start
	e.preferredCol = -1
}

func (e *inputEditor) readEvent() (editorEvent, error) {
	var first [1]byte
	if _, err := os.Stdin.Read(first[:]); err != nil {
		return editorEvent{}, err
	}

	switch first[0] {
	case '\r', '\n':
		return editorEvent{kind: editorEventSubmit}, nil
	case 0x03:
		return editorEvent{kind: editorEventInterrupt}, nil
	case 0x7f, 0x08:
		return editorEvent{kind: editorEventBackspace}, nil
	case 0x04:
		return editorEvent{kind: editorEventDelete}, nil
	case 0x01:
		return editorEvent{kind: editorEventMoveHome}, nil
	case 0x05:
		return editorEvent{kind: editorEventMoveEnd}, nil
	case 0x0b:
		return editorEvent{kind: editorEventClearToEnd}, nil
	case 0x15:
		return editorEvent{kind: editorEventClearToStart}, nil
	case 0x16:
		return editorEvent{kind: editorEventClipboardImage}, nil
	case 0x17:
		return editorEvent{kind: editorEventDeleteWord}, nil
	case 0x1b:
		return readEscapeEvent()
	default:
		if first[0] < 0x20 {
			return editorEvent{kind: editorEventNone}, nil
		}

		r, err := readUTF8Rune(first[0])
		if err != nil {
			return editorEvent{}, err
		}
		return editorEvent{kind: editorEventInsert, text: string(r)}, nil
	}
}

func readUTF8Rune(first byte) (rune, error) {
	if first < utf8.RuneSelf {
		return rune(first), nil
	}

	buf := []byte{first}
	for len(buf) < utf8.UTFMax && !utf8.FullRune(buf) {
		var next [1]byte
		if _, err := os.Stdin.Read(next[:]); err != nil {
			return utf8.RuneError, err
		}
		buf = append(buf, next[0])
	}

	r, _ := utf8.DecodeRune(buf)
	return r, nil
}

func readEscapeEvent() (editorEvent, error) {
	extra := make([]byte, 128)
	n := readWithTimeout(extra, 50*time.Millisecond)
	if n == 0 {
		return editorEvent{kind: editorEventNone}, nil
	}

	seq := extra[:n]
	switch {
	case bytes.HasPrefix(seq, []byte("[A")):
		return editorEvent{kind: editorEventMoveUp}, nil
	case bytes.HasPrefix(seq, []byte("[B")):
		return editorEvent{kind: editorEventMoveDown}, nil
	case bytes.HasPrefix(seq, []byte("[C")):
		return editorEvent{kind: editorEventMoveRight}, nil
	case bytes.HasPrefix(seq, []byte("[D")):
		return editorEvent{kind: editorEventMoveLeft}, nil
	case bytes.HasPrefix(seq, []byte("[H")), bytes.HasPrefix(seq, []byte("OH")):
		return editorEvent{kind: editorEventMoveHome}, nil
	case bytes.HasPrefix(seq, []byte("[F")), bytes.HasPrefix(seq, []byte("OF")):
		return editorEvent{kind: editorEventMoveEnd}, nil
	case bytes.HasPrefix(seq, []byte("[3~")):
		return editorEvent{kind: editorEventDelete}, nil
	case bytes.Equal(seq, []byte{0x7f}), bytes.Equal(seq, []byte{0x08}):
		return editorEvent{kind: editorEventClearToStart}, nil
	case bytes.HasPrefix(seq, []byte("[200~")):
		pasted, err := readBracketedPaste(seq[len("[200~"):])
		if err != nil {
			return editorEvent{}, err
		}
		return editorEvent{kind: editorEventInsert, text: pasted}, nil
	default:
		return editorEvent{kind: editorEventNone}, nil
	}
}

func readBracketedPaste(initial []byte) (string, error) {
	marker := []byte("\x1b[201~")
	data := append([]byte(nil), initial...)

	for {
		if idx := bytes.Index(data, marker); idx >= 0 {
			return normalizeEditorText(string(data[:idx])), nil
		}

		chunk := make([]byte, 512)
		n, err := os.Stdin.Read(chunk)
		if n > 0 {
			data = append(data, chunk[:n]...)
			continue
		}
		if err != nil {
			return normalizeEditorText(string(data)), err
		}
	}
}

func readInputFallback() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) && line != "" {
			return normalizeEditorText(strings.TrimRight(line, "\r\n")), nil
		}
		return "", err
	}
	return normalizeEditorText(strings.TrimRight(line, "\r\n")), nil
}

func (e *inputEditor) read() (string, error) {
	oldState, err := term.MakeRaw(e.fd)
	if err != nil {
		return "", err
	}
	defer term.Restore(e.fd, oldState)

	withTerminalOutputLock(func() {
		setActiveInputEditor(e)
		e.reserveRegionLocked()
	})
	defer withTerminalOutputLock(func() {
		e.cleanupRegionLocked()
		clearActiveInputEditor(e)
	})

	for {
		e.render()

		event, err := e.readEvent()
		if err != nil {
			return "", err
		}

		switch event.kind {
		case editorEventNone:
		case editorEventInsert:
			e.insertText(event.text)
		case editorEventBackspace:
			e.backspace()
		case editorEventDelete:
			e.delete()
		case editorEventMoveLeft:
			e.moveLeft()
		case editorEventMoveRight:
			e.moveRight()
		case editorEventMoveUp:
			e.moveVertical(-1)
		case editorEventMoveDown:
			e.moveVertical(1)
		case editorEventMoveHome:
			e.moveHome()
		case editorEventMoveEnd:
			e.moveEnd()
		case editorEventClearToStart:
			e.clearToStart()
		case editorEventClearToEnd:
			e.clearToEnd()
		case editorEventDeleteWord:
			e.deleteWordBackward()
		case editorEventClipboardImage:
			startIndex := clipboardImageCount() + 1
			added := pasteClipboardImages()
			for i := range added {
				e.insertText(fmt.Sprintf("[image%d] ", startIndex+i))
			}
		case editorEventSubmit:
			return string(e.buffer), nil
		case editorEventInterrupt:
			return "", errInputInterrupted
		}
	}
}

// ReadInput reads input from the terminal without redrawing previous output.
// Enter submits. Pasted multiline text is kept intact. Image file paths
// (dragged/pasted) and clipboard images (Ctrl+V) are returned separately.
func ReadInput() InputResult {
	_ = takeClipboardImages()

	fd := int(os.Stdin.Fd())
	var (
		line string
		err  error
	)

	if term.IsTerminal(fd) {
		line, err = newInputEditor(fd).read()
	} else {
		line, err = readInputFallback()
	}
	if err != nil {
		if errors.Is(err, errInputInterrupted) {
			fmt.Println()
			os.Exit(130)
		}
		if errors.Is(err, io.EOF) {
			fmt.Println()
			return InputResult{}
		}
		return InputResult{}
	}

	clipImages := takeClipboardImages()
	line = stripImageIndicators(normalizeEditorText(line))

	if strings.TrimSpace(line) == "" && len(clipImages) == 0 {
		return InputResult{}
	}

	text, dragImages := extractImages(line)
	allImages := append(clipImages, dragImages...)
	allImages = uniqueStrings(allImages)

	return InputResult{Text: text, Images: allImages}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// stripImageIndicators removes [imageN] markers from the input text.
func stripImageIndicators(s string) string {
	return imageIndicatorPattern.ReplaceAllString(s, "")
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}

	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
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
	withTerminalOutputLock(func() {
		writeStdoutUnlocked(s)
	})
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
	Label       string
	Desc        string
	IsTextEntry bool // When true, selecting this option opens a text input prompt
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
