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
	"sort"
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
	{Name: "/supabase", Desc: "Connect Supabase backend"},
	{Name: "/revenuecat", Desc: "Connect RevenueCat payments"},
	{Name: "/integrations", Desc: "Manage all integrations"},
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
	Text        string   // The text prompt (with image paths removed)
	DisplayText string   // User-facing transcript text with attachment markers
	Images      []string // Absolute paths to image files found in the input
}

type inputToken struct {
	start int
	end   int
	value string
	ready bool
}

// imageExtensions are file extensions recognized as images.
var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".webp": true, ".bmp": true, ".tiff": true, ".tif": true,
	".heic": true, ".heif": true, ".svg": true, ".pdf": true,
}

var imageIndicatorPattern = regexp.MustCompile(`\[(?i:image)\d+\]\s*`)

const (
	inputBoxBackground        = "\033[48;5;236m"
	inputBoxForeground        = "\033[38;5;252m"
	inputBoxHorizontalPadding = 1
	inputBoxVerticalPadding   = 1
	inputBoxVisibleRows       = 5
	inputBoxHelperRows        = 3
	inputBoxHelperMatches     = 8
	inputBoxBottomSafeRows    = 2
	inputBoxMinContentWidth   = 20
	pastedTextCollapseLines   = 4 // pastes longer than this become collapsed blocks
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

// pastedBlock holds large pasted text that is shown collapsed above the text area.
type pastedBlock struct {
	content  string // full pasted text
	numLines int    // line count for the label
}

type inlineAttachmentKind int

const (
	inlineAttachmentImage inlineAttachmentKind = iota
	inlineAttachmentPastedText
)

type inlineAttachment struct {
	id     int
	pos    int
	kind   inlineAttachmentKind
	path   string
	pasted pastedBlock
}

type inputEditor struct {
	fd             int
	width          int
	padding        int
	contentWidth   int
	maxVisibleRows int
	helperRows     int
	regionRows     int
	renderCursor   cursorCoord
	buffer         []rune
	cursor         int
	preferredCol   int
	scrollTop      int
	backgroundLine string
	nextAttachment int
	attachments    []inlineAttachment
}

func formatImageReference(index int) string {
	return fmt.Sprintf("[Image #%d]", index)
}

func formatPastedBlockReference(index, lineCount int) string {
	return fmt.Sprintf("[Pasted Text #%d: %d lines]", index, lineCount)
}

func joinAttachmentLabels(labels []string) string {
	return strings.Join(labels, " ")
}

func appendAttachmentLabelsToText(text string, labels []string) string {
	if len(labels) == 0 {
		return text
	}

	labelText := joinAttachmentLabels(labels)
	lastRune, _ := utf8.DecodeLastRuneInString(text)
	switch {
	case text == "":
		return labelText
	case strings.HasSuffix(text, "\n"):
		return text + labelText
	case unicode.IsSpace(lastRune):
		return text + labelText
	default:
		return text + " " + labelText
	}
}

func attachmentDisplayText(labels []string, prev, next rune) string {
	if len(labels) == 0 {
		return ""
	}

	group := joinAttachmentLabels(labels)
	if prev != 0 && prev != '\n' && !unicode.IsSpace(prev) {
		group = " " + group
	}
	if next != 0 && next != '\n' && !unicode.IsSpace(next) {
		group += " "
	}
	return group
}

func layoutAttachmentLabels(labels []string, width int) []string {
	if len(labels) == 0 {
		return nil
	}
	if width <= 0 {
		return []string{joinAttachmentLabels(labels)}
	}

	lines := make([]string, 0, 1)
	current := labels[0]
	currentWidth := uniseg.StringWidth(current)

	for _, label := range labels[1:] {
		labelWidth := uniseg.StringWidth(label)
		if currentWidth+1+labelWidth <= width {
			current += " " + label
			currentWidth += 1 + labelWidth
			continue
		}
		lines = append(lines, current)
		current = label
		currentWidth = labelWidth
	}

	lines = append(lines, current)
	return lines
}

func materializeEditorContent(buffer []rune, attachmentGroups map[int]string) ([]rune, []int) {
	if len(attachmentGroups) == 0 {
		indices := make([]int, len(buffer)+1)
		for i := range indices {
			indices[i] = i
		}
		return append([]rune(nil), buffer...), indices
	}

	display := make([]rune, 0, len(buffer))
	indices := make([]int, len(buffer)+1)
	for i := 0; i <= len(buffer); i++ {
		if group := attachmentGroups[i]; group != "" {
			display = append(display, []rune(group)...)
		}
		indices[i] = len(display)
		if i < len(buffer) {
			display = append(display, buffer[i])
		}
	}
	return display, indices
}

func layoutEditorContent(buffer []rune, attachmentGroups map[int]string, contentWidth int) editorLayout {
	if len(attachmentGroups) == 0 {
		return layoutEditorBuffer(buffer, contentWidth)
	}

	display, indices := materializeEditorContent(buffer, attachmentGroups)
	layout := layoutEditorBuffer(display, contentWidth)
	bufferPositions := make([]cursorCoord, len(indices))
	for i, idx := range indices {
		bufferPositions[i] = layout.positions[idx]
	}

	return editorLayout{
		lines:     layout.lines,
		positions: bufferPositions,
	}
}

// isImagePath checks if a string looks like a path to an image file.
func isImagePath(s string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(s)))
	return imageExtensions[ext] && isFilePath(s)
}

// isFilePath checks if a string looks like an absolute path to an existing file.
func isFilePath(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	// Must be an absolute path or start with ~
	if !filepath.IsAbs(s) && !strings.HasPrefix(s, "~") {
		return false
	}
	resolved := expandHome(s)
	info, err := os.Stat(resolved)
	return err == nil && !info.IsDir()
}

func expandHome(s string) string {
	if strings.HasPrefix(s, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, s[1:])
		}
	}
	return s
}

// resolveImagePath expands ~ and cleans up a path.
func resolveImagePath(s string) string {
	return filepath.Clean(expandHome(strings.TrimSpace(s)))
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
		if !token.ready {
			continue
		}
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
				case r == '\\':
					// Handle backslash escapes inside both single and double quotes.
					// macOS Finder wraps dragged paths in single quotes and escapes
					// embedded single quotes as \' (e.g. Queen\'s Park).
					next, nextWidth := utf8DecodeRuneInString(input[i+width:])
					if nextWidth == 0 {
						i += width
						continue
					}
					sb.WriteRune(next)
					i += width + nextWidth
				case r == quote:
					quote = 0
					i += width
				default:
					sb.WriteRune(r)
					i += width
				}
				continue
			}

			switch {
			case unicode.IsSpace(r):
				tokens = append(tokens, inputToken{start: start, end: i, value: sb.String(), ready: true})
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

		tokens = append(tokens, inputToken{start: start, end: len(input), value: sb.String(), ready: quote == 0})
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
	return resolveReferenceWith(raw, isImagePath)
}

func resolveFileReference(raw string) (string, bool) {
	return resolveReferenceWith(raw, isFilePath)
}

func resolveReferenceWith(raw string, check func(string) bool) (string, bool) {
	candidates := []string{strings.TrimSpace(raw)}
	if trimmed := strings.TrimRight(candidates[0], ".,;:!?"); trimmed != candidates[0] {
		candidates = append(candidates, trimmed)
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if resolved, ok := resolvePathReferenceWith(candidate, check); ok {
			return resolved, true
		}
	}
	return "", false
}

func resolveImagePathReference(candidate string) (string, bool) {
	return resolvePathReferenceWith(candidate, isImagePath)
}

func resolveImageReferencesByLine(input string) ([]string, bool) {
	lines := strings.Split(normalizeEditorText(input), "\n")
	images := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		resolved, ok := resolveImageReference(line)
		if !ok {
			return nil, false
		}
		images = append(images, resolved)
	}
	if len(images) == 0 {
		return nil, false
	}
	return uniqueStrings(images), true
}

func resolvePathReferenceWith(candidate string, check func(string) bool) (string, bool) {
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

	if !check(candidate) {
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

func (e *inputEditor) renderAttachmentLine(content string) string {
	return e.renderBoxLine(content)
}

func (e *inputEditor) orderedAttachments() []inlineAttachment {
	if len(e.attachments) == 0 {
		return nil
	}

	ordered := append([]inlineAttachment(nil), e.attachments...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].pos != ordered[j].pos {
			return ordered[i].pos < ordered[j].pos
		}
		return ordered[i].id < ordered[j].id
	})
	return ordered
}

func (e *inputEditor) attachmentDisplayGroups() map[int]string {
	ordered := e.orderedAttachments()
	if len(ordered) == 0 {
		return nil
	}

	labelsByPos := make(map[int][]string)
	imageCount := 0
	pastedCount := 0
	for _, attachment := range ordered {
		switch attachment.kind {
		case inlineAttachmentImage:
			imageCount++
			labelsByPos[attachment.pos] = append(labelsByPos[attachment.pos], formatImageReference(imageCount))
		case inlineAttachmentPastedText:
			pastedCount++
			labelsByPos[attachment.pos] = append(labelsByPos[attachment.pos], formatPastedBlockReference(pastedCount, attachment.pasted.numLines))
		}
	}

	groups := make(map[int]string, len(labelsByPos))
	for pos, labels := range labelsByPos {
		var prev rune
		if pos > 0 {
			prev = e.buffer[pos-1]
		}
		var next rune
		if pos < len(e.buffer) {
			next = e.buffer[pos]
		}
		groups[pos] = attachmentDisplayText(labels, prev, next)
	}
	return groups
}

func (e *inputEditor) imagePaths() []string {
	ordered := e.orderedAttachments()
	paths := make([]string, 0, len(ordered))
	for _, attachment := range ordered {
		if attachment.kind == inlineAttachmentImage {
			paths = append(paths, attachment.path)
		}
	}
	return paths
}

func (e *inputEditor) shiftAttachmentsForInsert(pos, delta int) {
	if delta <= 0 {
		return
	}
	for i := range e.attachments {
		if e.attachments[i].pos > pos {
			e.attachments[i].pos += delta
		}
	}
}

func (e *inputEditor) shiftAttachmentsForDelete(start, end int) {
	if end <= start {
		return
	}
	delta := end - start
	for i := range e.attachments {
		switch {
		case e.attachments[i].pos > end:
			e.attachments[i].pos -= delta
		case e.attachments[i].pos > start:
			e.attachments[i].pos = start
		}
	}
}

func (e *inputEditor) insertAttachment(attachment inlineAttachment) {
	e.insertAttachmentAt(e.cursor, attachment)
}

func (e *inputEditor) insertAttachmentAt(pos int, attachment inlineAttachment) {
	e.nextAttachment++
	attachment.id = e.nextAttachment
	attachment.pos = pos
	e.attachments = append(e.attachments, attachment)
	e.preferredCol = -1
}

func (e *inputEditor) insertPastedBlock(text string, lineCount int) {
	e.insertAttachment(inlineAttachment{
		kind: inlineAttachmentPastedText,
		pasted: pastedBlock{
			content:  text,
			numLines: lineCount,
		},
	})
}

func (e *inputEditor) insertAttachedFiles(paths []string) {
	for _, path := range paths {
		e.insertAttachment(inlineAttachment{
			kind: inlineAttachmentImage,
			path: path,
		})
	}
}

func (e *inputEditor) removeLastAttachmentAt(pos int) bool {
	index := -1
	for i := range e.attachments {
		if e.attachments[i].pos == pos {
			if index == -1 || e.attachments[i].id > e.attachments[index].id {
				index = i
			}
		}
	}
	if index < 0 {
		return false
	}
	e.attachments = append(e.attachments[:index], e.attachments[index+1:]...)
	e.preferredCol = -1
	return true
}

func (e *inputEditor) removeAttachmentsInRange(start, end int, includeStart bool) bool {
	if end < start {
		start, end = end, start
	}

	kept := e.attachments[:0]
	removed := false
	for _, attachment := range e.attachments {
		remove := attachment.pos <= end
		if includeStart {
			remove = remove && attachment.pos >= start
		} else {
			remove = remove && attachment.pos > start
		}
		if remove {
			removed = true
			continue
		}
		kept = append(kept, attachment)
	}
	e.attachments = kept
	if removed {
		e.preferredCol = -1
	}
	return removed
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

func (e *inputEditor) helperLines(layout editorLayout) []string {
	text := string(e.buffer)
	lines := make([]string, 0, e.helperRows+inputBoxHelperMatches)
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
		for i := 0; i < minInt(len(matches), inputBoxHelperMatches); i++ {
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
	return lines
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

func (e *inputEditor) totalRows(boxRows, helperRows int) int {
	return inputBoxVerticalPadding*2 + boxRows + helperRows + inputBoxBottomSafeRows
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
	layout := layoutEditorContent(e.buffer, e.attachmentDisplayGroups(), e.contentWidth)
	boxRows := e.boxRows(layout)
	e.ensureCursorVisible(layout, boxRows)
	helperLines := e.helperLines(layout)
	totalRows := e.totalRows(boxRows, len(helperLines))

	lines := make([]string, 0, totalRows)
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
	lines = append(lines, helperLines...)
	for i := 0; i < inputBoxBottomSafeRows; i++ {
		lines = append(lines, "")
	}

	writeStdoutUnlocked("\033[?25l")
	cursor := layout.positions[e.cursor]
	cursorRow := inputBoxVerticalPadding + (cursor.row - e.scrollTop)
	cursorCol := e.padding + cursor.col
	if cursorRow < inputBoxVerticalPadding {
		cursorRow = inputBoxVerticalPadding
	}
	if cursorRow >= inputBoxVerticalPadding+boxRows {
		cursorRow = inputBoxVerticalPadding + boxRows - 1
	}

	e.moveToRenderOriginLocked()
	writeStdoutUnlocked("\033[J")
	for i, line := range lines {
		writeStdoutUnlocked(line)
		if i < len(lines)-1 {
			writeStdoutUnlocked("\r\n")
		}
	}

	if lastRow := len(lines) - 1; lastRow > cursorRow {
		writeStdoutUnlocked(fmt.Sprintf("\033[%dA", lastRow-cursorRow))
	}
	writeStdoutUnlocked("\r")
	if cursorCol > 0 {
		writeStdoutUnlocked(fmt.Sprintf("\033[%dC", cursorCol))
	}
	writeStdoutUnlocked("\033[?25h")

	e.regionRows = len(lines)
	e.renderCursor = cursorCoord{row: cursorRow, col: cursorCol}
}

func (e *inputEditor) reserveRegionLocked() {
	writeStdoutUnlocked("\033[?2004h")
	e.regionRows = 0
	e.renderCursor = cursorCoord{}
}

func (e *inputEditor) cleanupRegionLocked() {
	writeStdoutUnlocked("\033[?25l")
	e.moveToRenderOriginLocked()
	writeStdoutUnlocked("\033[J")
	writeStdoutUnlocked("\r")
	writeStdoutUnlocked("\033[?2004l")
	writeStdoutUnlocked("\033[?25h")
	e.regionRows = 0
	e.renderCursor = cursorCoord{}
}

func (e *inputEditor) moveToRenderOriginLocked() {
	writeStdoutUnlocked("\r")
	if e.renderCursor.row > 0 {
		writeStdoutUnlocked(fmt.Sprintf("\033[%dA", e.renderCursor.row))
		writeStdoutUnlocked("\r")
	}
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
	e.shiftAttachmentsForInsert(e.cursor, len(inserted))
	e.buffer = next
	e.cursor += len(inserted)
	e.preferredCol = -1
	e.promoteImageReferencesFromBuffer()
}

func (e *inputEditor) backspace() {
	if e.removeLastAttachmentAt(e.cursor) {
		return
	}
	if e.cursor <= 0 || len(e.buffer) == 0 {
		// No text to delete — try removing the most recent attachment.
		// This handles: paste image → press backspace (cursor at 0, attachment at 0).
		// Also handles: paste image → type text → select-all-delete → backspace.
		if len(e.attachments) > 0 {
			e.removeNewestAttachment()
		}
		return
	}
	e.shiftAttachmentsForDelete(e.cursor-1, e.cursor)
	e.buffer = append(e.buffer[:e.cursor-1], e.buffer[e.cursor:]...)
	e.cursor--
	e.preferredCol = -1
}

// removeNewestAttachment removes the attachment with the highest id (most recently added).
func (e *inputEditor) removeNewestAttachment() {
	if len(e.attachments) == 0 {
		return
	}
	newest := 0
	for i := range e.attachments {
		if e.attachments[i].id > e.attachments[newest].id {
			newest = i
		}
	}
	e.attachments = append(e.attachments[:newest], e.attachments[newest+1:]...)
	e.preferredCol = -1
}

func (e *inputEditor) delete() {
	if e.removeLastAttachmentAt(e.cursor) {
		return
	}
	if e.cursor < 0 || e.cursor >= len(e.buffer) {
		return
	}
	e.shiftAttachmentsForDelete(e.cursor, e.cursor+1)
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
	layout := layoutEditorContent(e.buffer, e.attachmentDisplayGroups(), e.contentWidth)
	if e.cursor < len(layout.positions) {
		row := layout.positions[e.cursor].row
		e.cursor = layout.indexForPosition(row, 0)
	}
	e.preferredCol = -1
}

func (e *inputEditor) moveEnd() {
	layout := layoutEditorContent(e.buffer, e.attachmentDisplayGroups(), e.contentWidth)
	if e.cursor < len(layout.positions) {
		row := layout.positions[e.cursor].row
		e.cursor = layout.indexForPosition(row, e.contentWidth)
	}
	e.preferredCol = -1
}

func (e *inputEditor) moveVertical(delta int) {
	layout := layoutEditorContent(e.buffer, e.attachmentDisplayGroups(), e.contentWidth)
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
		e.removeAttachmentsInRange(start, e.cursor, true)
		return
	}

	e.removeAttachmentsInRange(start, e.cursor, true)
	e.shiftAttachmentsForDelete(start, e.cursor)
	e.buffer = append(e.buffer[:start], e.buffer[e.cursor:]...)
	e.cursor = start
	e.preferredCol = -1
}

func (e *inputEditor) clearToEnd() {
	if e.cursor >= len(e.buffer) {
		e.removeAttachmentsInRange(e.cursor, len(e.buffer), false)
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
		e.removeAttachmentsInRange(e.cursor, end, false)
		return
	}

	e.removeAttachmentsInRange(e.cursor, end, false)
	e.shiftAttachmentsForDelete(e.cursor, end)
	e.buffer = append(e.buffer[:e.cursor], e.buffer[end:]...)
	e.preferredCol = -1
}

// insertPossibleAttachments checks if inserted text contains pasted image
// references (for example from Finder or bracketed paste). Image paths are
// promoted into attachment rows; the remaining text is inserted into the buffer
// normally. Large multi-line pastes are collapsed into a single attachment row.
func (e *inputEditor) insertPossibleAttachments(text string) {
	if images, ok := resolveImageReferencesByLine(text); ok {
		e.insertAttachedFiles(images)
		return
	}

	tokens := scanInputTokens(text)
	if len(tokens) == 0 {
		e.maybeCollapsePaste(text)
		return
	}

	var (
		hasImage     bool
		lastConsumed int
	)
	for _, tok := range tokens {
		if !tok.ready {
			continue
		}
		if resolved, ok := resolveImageReference(tok.value); ok {
			if tok.start > lastConsumed {
				e.maybeCollapsePaste(text[lastConsumed:tok.start])
			}
			e.insertAttachedFiles([]string{resolved})
			lastConsumed = tok.end
			hasImage = true
		}
	}
	if !hasImage {
		e.maybeCollapsePaste(text)
		return
	}
	if lastConsumed < len(text) {
		e.maybeCollapsePaste(text[lastConsumed:])
	}
}

// maybeCollapsePaste inserts text normally if it's short, or collapses it into
// a pasted block attachment row (like Codex/Claude Code) if it's large.
func (e *inputEditor) maybeCollapsePaste(text string) {
	lineCount := strings.Count(text, "\n") + 1
	if lineCount <= pastedTextCollapseLines {
		e.insertText(text)
		return
	}
	e.insertPastedBlock(text, lineCount)
}

func (e *inputEditor) appendAttachedFiles(paths []string) {
	e.insertAttachedFiles(paths)
}

func (e *inputEditor) removeLastAttachment() bool {
	return e.removeLastAttachmentAt(e.cursor)
}

func (e *inputEditor) promoteImageReferencesFromBuffer() {
	if len(e.buffer) == 0 {
		return
	}

	fullText := string(e.buffer)
	if images, ok := resolveImageReferencesByLine(fullText); ok {
		e.insertAttachedFiles(images)
		e.buffer = nil
		e.cursor = 0
		e.preferredCol = -1
		return
	}

	tokens := scanInputTokens(fullText)
	if len(tokens) == 0 {
		return
	}

	var (
		clean        []rune
		removed      []inputToken
		replacements []inlineAttachment
		last         int
	)
	oldCursor := e.cursor
	for _, tok := range tokens {
		if !tok.ready {
			continue
		}
		resolved, ok := resolveImageReference(tok.value)
		if !ok {
			continue
		}
		if tok.start > last {
			clean = append(clean, []rune(fullText[last:tok.start])...)
		}
		replacements = append(replacements, inlineAttachment{
			pos:  len(clean),
			kind: inlineAttachmentImage,
			path: resolved,
		})
		removed = append(removed, tok)
		last = tok.end
	}
	if len(replacements) == 0 {
		return
	}
	if last < len(fullText) {
		clean = append(clean, []rune(fullText[last:])...)
	}

	for i := range e.attachments {
		pos := e.attachments[i].pos
		for _, tok := range removed {
			start := utf8.RuneCountInString(fullText[:tok.start])
			end := start + utf8.RuneCountInString(tok.value)
			length := end - start
			switch {
			case pos > end:
				pos -= length
			case pos > start:
				pos = start
			}
		}
		e.attachments[i].pos = pos
	}
	for _, replacement := range replacements {
		e.insertAttachmentAt(replacement.pos, replacement)
	}
	cursor := oldCursor
	for _, tok := range removed {
		start := utf8.RuneCountInString(fullText[:tok.start])
		end := start + utf8.RuneCountInString(tok.value)
		length := end - start
		switch {
		case cursor > end:
			cursor -= length
		case cursor > start:
			cursor = start
		}
	}
	if cursor > len(clean) {
		cursor = len(clean)
	}
	e.buffer = clean
	e.cursor = cursor
	e.preferredCol = -1
}

func (e *inputEditor) deleteWordBackward() {
	if e.cursor <= 0 {
		e.removeAttachmentsInRange(0, e.cursor, true)
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
		e.removeAttachmentsInRange(start, e.cursor, true)
		return
	}

	e.removeAttachmentsInRange(start, e.cursor, true)
	e.shiftAttachmentsForDelete(start, e.cursor)
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

func (e *inputEditor) read() (string, string, []string, error) {
	oldState, err := term.MakeRaw(e.fd)
	if err != nil {
		return "", "", nil, err
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
			return "", "", nil, err
		}

		switch event.kind {
		case editorEventNone:
		case editorEventInsert:
			e.insertPossibleAttachments(event.text)
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
			added := pasteClipboardImages()
			e.insertAttachedFiles(added)
		case editorEventSubmit:
			text := e.submitText()
			return text, e.displayText(), e.imagePaths(), nil
		case editorEventInterrupt:
			return "", "", nil, errInputInterrupted
		}
	}
}

// submitText returns the full text to submit, combining the buffer with any
// collapsed pasted blocks.
func (e *inputEditor) submitText() string {
	var sb strings.Builder
	ordered := e.orderedAttachments()
	index := 0
	for pos := 0; pos <= len(e.buffer); pos++ {
		for index < len(ordered) && ordered[index].pos == pos {
			if ordered[index].kind == inlineAttachmentPastedText {
				sb.WriteString(ordered[index].pasted.content)
			}
			index++
		}
		if pos < len(e.buffer) {
			sb.WriteRune(e.buffer[pos])
		}
	}
	return sb.String()
}

func (e *inputEditor) displayText() string {
	display, _ := materializeEditorContent(e.buffer, e.attachmentDisplayGroups())
	return string(display)
}

// ReadInput reads input from the terminal without redrawing previous output.
// Enter submits. Pasted multiline text is kept intact. Image file paths
// (dragged/pasted) and clipboard images (Ctrl+V) are returned separately.
func ReadInput() InputResult {
	_ = takeClipboardImages()

	fd := int(os.Stdin.Fd())
	var (
		line        string
		displayLine string
		editorFiles []string
		err         error
	)

	if term.IsTerminal(fd) {
		line, displayLine, editorFiles, err = newInputEditor(fd).read()
	} else {
		line, err = readInputFallback()
		displayLine = line
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
	displayLine = stripImageIndicators(normalizeEditorText(displayLine))

	allImages := append(editorFiles, clipImages...)

	if strings.TrimSpace(line) == "" && len(allImages) == 0 {
		return InputResult{}
	}

	text, dragImages := extractImages(line)
	allImages = append(allImages, dragImages...)
	allImages = uniqueStrings(allImages)

	if displayLine == "" {
		displayLine = text
	}
	return InputResult{Text: text, DisplayText: displayLine, Images: allImages}
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
