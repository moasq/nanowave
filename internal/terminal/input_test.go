package terminal

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

func TestExtractImagesPreservesPlainMultilineText(t *testing.T) {
	input := "  if foo {\n    bar()\n  }\n"

	text, images := extractImages(input)

	if text != input {
		t.Fatalf("extractImages() text = %q, want %q", text, input)
	}
	if len(images) != 0 {
		t.Fatalf("extractImages() images = %#v, want none", images)
	}
}

func TestExtractImagesHandlesEscapedAndQuotedPaths(t *testing.T) {
	dir := t.TempDir()
	first := createTestImage(t, dir, "space image.png")
	second := createTestImage(t, dir, "quote-image.jpg")

	input := fmt.Sprintf("Compare %s and %q please", escapePath(first), second)

	text, images := extractImages(input)

	if got, want := images, []string{first, second}; !sameStrings(got, want) {
		t.Fatalf("extractImages() images = %#v, want %#v", got, want)
	}
	if strings.Contains(text, first) || strings.Contains(text, second) {
		t.Fatalf("extractImages() text still contains image paths: %q", text)
	}
	if !strings.Contains(text, "Compare") || !strings.Contains(text, "please") {
		t.Fatalf("extractImages() text lost surrounding prompt: %q", text)
	}
}

func TestExtractImagesHandlesFileURLs(t *testing.T) {
	dir := t.TempDir()
	imagePath := createTestImage(t, dir, "with space.webp")
	fileURL := (&url.URL{Scheme: "file", Path: imagePath}).String()

	text, images := extractImages("Inspect " + fileURL + " now")

	if got, want := images, []string{imagePath}; !sameStrings(got, want) {
		t.Fatalf("extractImages() images = %#v, want %#v", got, want)
	}
	if strings.Contains(text, fileURL) {
		t.Fatalf("extractImages() text still contains file URL: %q", text)
	}
}

func TestExtractImagesExtractsMultipleDroppedImages(t *testing.T) {
	dir := t.TempDir()
	first := createTestImage(t, dir, "first.png")
	second := createTestImage(t, dir, "second image.jpeg")

	text, images := extractImages(escapePath(first) + " " + escapePath(second))

	if text != "" {
		t.Fatalf("extractImages() text = %q, want empty", text)
	}
	if got, want := images, []string{first, second}; !sameStrings(got, want) {
		t.Fatalf("extractImages() images = %#v, want %#v", got, want)
	}
}

func TestExtractImagesKeepsNonImageFiles(t *testing.T) {
	dir := t.TempDir()
	notesPath := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(notesPath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	text, images := extractImages(notesPath)

	if text != notesPath {
		t.Fatalf("extractImages() text = %q, want %q", text, notesPath)
	}
	if len(images) != 0 {
		t.Fatalf("extractImages() images = %#v, want none", images)
	}
}

func TestStripImageIndicatorsRemovesAttachmentMarkers(t *testing.T) {
	input := "Review [image1] and [IMAGE2] now"

	got := stripImageIndicators(input)

	if got != "Review and now" {
		t.Fatalf("stripImageIndicators() = %q", got)
	}
}

func TestInputBoxMetricsMatchesVerticalPaddingOnWideTerminal(t *testing.T) {
	padding, contentWidth := inputBoxMetrics(120)

	if padding != 1 {
		t.Fatalf("inputBoxMetrics() padding = %d, want 1", padding)
	}
	if contentWidth != 118 {
		t.Fatalf("inputBoxMetrics() contentWidth = %d, want 118", contentWidth)
	}
}

func TestLayoutEditorBufferWrapsAndTracksCursor(t *testing.T) {
	layout := layoutEditorBuffer([]rune("abcdefghij"), 4)

	if got, want := layout.lines, []string{"abcd", "efgh", "ij"}; !sameStrings(got, want) {
		t.Fatalf("layoutEditorBuffer() lines = %#v, want %#v", got, want)
	}
	if got, want := layout.positions[len(layout.positions)-1], (cursorCoord{row: 2, col: 2}); got != want {
		t.Fatalf("layoutEditorBuffer() last cursor = %#v, want %#v", got, want)
	}
}

func TestCurrentCommandPrefixIgnoresArguments(t *testing.T) {
	if got := currentCommandPrefix("/agent codex", len([]rune("/agent codex"))); got != "" {
		t.Fatalf("currentCommandPrefix() = %q, want empty", got)
	}
	if got := currentCommandPrefix("/ag", len([]rune("/ag"))); got != "/ag" {
		t.Fatalf("currentCommandPrefix() = %q, want /ag", got)
	}
}

func TestInputEditorBoxRowsGrowWithContent(t *testing.T) {
	editor := &inputEditor{maxVisibleRows: 5}

	if got := editor.boxRows(editorLayout{lines: []string{""}}); got != 1 {
		t.Fatalf("boxRows() = %d, want 1", got)
	}
	if got := editor.boxRows(editorLayout{lines: []string{"a", "b", "c"}}); got != 3 {
		t.Fatalf("boxRows() = %d, want 3", got)
	}
	if got := editor.boxRows(editorLayout{lines: []string{"1", "2", "3", "4", "5", "6"}}); got != 5 {
		t.Fatalf("boxRows() = %d, want 5", got)
	}
}

func TestInputEditorClearToStartRemovesCurrentLogicalLinePrefix(t *testing.T) {
	editor := &inputEditor{
		buffer: []rune("first line\nsecond line"),
		cursor: len([]rune("first line\nsecond")),
	}

	editor.clearToStart()

	if got, want := string(editor.buffer), "first line\n line"; got != want {
		t.Fatalf("clearToStart() buffer = %q, want %q", got, want)
	}
	if got, want := editor.cursor, len([]rune("first line\n")); got != want {
		t.Fatalf("clearToStart() cursor = %d, want %d", got, want)
	}
}

func TestInputEditorClearToStartRemovesInlineAttachmentInDeletedSpan(t *testing.T) {
	editor := &inputEditor{
		buffer: []rune("hello world"),
		cursor: len([]rune("hello world")),
		attachments: []inlineAttachment{
			{
				id:   1,
				pos:  len([]rune("hello ")),
				kind: inlineAttachmentPastedText,
				pasted: pastedBlock{
					content:  "line 1\nline 2",
					numLines: 2,
				},
			},
		},
	}

	editor.clearToStart()

	if got, want := string(editor.buffer), ""; got != want {
		t.Fatalf("clearToStart() buffer = %q, want %q", got, want)
	}
	if len(editor.attachments) != 0 {
		t.Fatalf("attachments len = %d, want 0", len(editor.attachments))
	}
}

func TestInputEditorClearToEndRemovesCurrentLogicalLineSuffix(t *testing.T) {
	editor := &inputEditor{
		buffer: []rune("first line\nsecond line"),
		cursor: len([]rune("first line\nsecond")),
	}

	editor.clearToEnd()

	if got, want := string(editor.buffer), "first line\nsecond"; got != want {
		t.Fatalf("clearToEnd() buffer = %q, want %q", got, want)
	}
}

func TestInputEditorDeleteWordBackwardRemovesPreviousWord(t *testing.T) {
	editor := &inputEditor{
		buffer: []rune("hello brave world"),
		cursor: len([]rune("hello brave ")),
	}

	editor.deleteWordBackward()

	if got, want := string(editor.buffer), "hello world"; got != want {
		t.Fatalf("deleteWordBackward() buffer = %q, want %q", got, want)
	}
	if got, want := editor.cursor, len([]rune("hello ")); got != want {
		t.Fatalf("deleteWordBackward() cursor = %d, want %d", got, want)
	}
}

func TestInputEditorDeleteWordBackwardRemovesInlineAttachmentInDeletedSpan(t *testing.T) {
	editor := &inputEditor{
		buffer: []rune("hello world"),
		cursor: len([]rune("hello world")),
		attachments: []inlineAttachment{
			{
				id:   1,
				pos:  len([]rune("hello ")),
				kind: inlineAttachmentPastedText,
				pasted: pastedBlock{
					content:  "line 1\nline 2",
					numLines: 2,
				},
			},
		},
	}

	editor.deleteWordBackward()

	if got, want := string(editor.buffer), "hello "; got != want {
		t.Fatalf("deleteWordBackward() buffer = %q, want %q", got, want)
	}
	if len(editor.attachments) != 0 {
		t.Fatalf("attachments len = %d, want 0", len(editor.attachments))
	}
}

func TestInputEditorPromotesFinderImagePasteIntoAttachment(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Queen's Park")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	imagePath := createTestImage(t, dir, "mockup image.png")
	pasted := finderQuotedPath(imagePath)

	editor := &inputEditor{}
	for _, r := range pasted {
		editor.insertText(string(r))
	}

	if got, want := editor.imagePaths(), []string{imagePath}; !sameStrings(got, want) {
		t.Fatalf("imagePaths() = %#v, want %#v", got, want)
	}
	if got := string(editor.buffer); got != "" {
		t.Fatalf("buffer = %q, want empty", got)
	}
	if editor.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", editor.cursor)
	}
}

func TestInputEditorPromotesRawImagePathWithSpacesIntoAttachment(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Summer Shots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	imagePath := createTestImage(t, dir, "family photo.png")

	editor := &inputEditor{}
	for _, r := range imagePath {
		editor.insertText(string(r))
	}

	if got, want := editor.imagePaths(), []string{imagePath}; !sameStrings(got, want) {
		t.Fatalf("imagePaths() = %#v, want %#v", got, want)
	}
	if got := string(editor.buffer); got != "" {
		t.Fatalf("buffer = %q, want empty", got)
	}
}

func TestInputEditorKeepsNonImagePasteAsText(t *testing.T) {
	root := t.TempDir()
	// Use .txt which is never an image extension.
	txtPath := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(txtPath, []byte("text-data"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	pasted := finderQuotedPath(txtPath)

	editor := &inputEditor{}
	for _, r := range pasted {
		editor.insertText(string(r))
	}

	if got := editor.imagePaths(); len(got) != 0 {
		t.Fatalf("imagePaths() = %#v, want none", got)
	}
	if got := string(editor.buffer); got != pasted {
		t.Fatalf("buffer = %q, want %q", got, pasted)
	}
}

func TestInputEditorPromotesPDFPasteIntoAttachment(t *testing.T) {
	root := t.TempDir()
	pdfPath := filepath.Join(root, "mockup.pdf")
	if err := os.WriteFile(pdfPath, []byte("pdf-data"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	pasted := finderQuotedPath(pdfPath)

	editor := &inputEditor{}
	for _, r := range pasted {
		editor.insertText(string(r))
	}

	got := editor.imagePaths()
	if len(got) != 1 {
		t.Fatalf("imagePaths() = %#v, want 1 PDF path", got)
	}
	if got[0] != pdfPath {
		t.Fatalf("imagePaths()[0] = %q, want %q", got[0], pdfPath)
	}
}

func TestResolveImageReferencesByLineHandlesSingleRawPathWithSpaces(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Summer Shots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	imagePath := createTestImage(t, dir, "family photo.png")

	images, ok := resolveImageReferencesByLine(imagePath + "\n")

	if !ok {
		t.Fatalf("resolveImageReferencesByLine() ok = false, want true")
	}
	if got, want := images, []string{imagePath}; !sameStrings(got, want) {
		t.Fatalf("resolveImageReferencesByLine() = %#v, want %#v", got, want)
	}
}

func TestInputEditorDisplayTextUsesPastedBlockMarkers(t *testing.T) {
	editor := &inputEditor{
		buffer: []rune("Summarize this"),
		attachments: []inlineAttachment{
			{id: 1, pos: len([]rune("Summarize this")), kind: inlineAttachmentPastedText, pasted: pastedBlock{content: "line 1\nline 2", numLines: 2}},
			{id: 2, pos: len([]rune("Summarize this")), kind: inlineAttachmentPastedText, pasted: pastedBlock{content: "line 3\nline 4", numLines: 2}},
		},
	}

	got := editor.displayText()
	want := "Summarize this [Pasted Text #1: 2 lines] [Pasted Text #2: 2 lines]"
	if got != want {
		t.Fatalf("displayText() = %q, want %q", got, want)
	}
}

func TestLayoutEditorContentKeepsPastedBlockAtInsertionPoint(t *testing.T) {
	editor := &inputEditor{
		buffer: []rune("hello dfkldjsfkl jsdf"),
		attachments: []inlineAttachment{
			{id: 1, pos: len([]rune("hello ")), kind: inlineAttachmentPastedText, pasted: pastedBlock{content: strings.Repeat("line\n", 16) + "line", numLines: 17}},
		},
	}
	layout := layoutEditorContent(editor.buffer, editor.attachmentDisplayGroups(), 120)

	if len(layout.lines) != 1 {
		t.Fatalf("layoutEditorContent() len = %d, want 1", len(layout.lines))
	}
	want := "hello [Pasted Text #1: 17 lines] dfkldjsfkl jsdf"
	if got := layout.lines[0]; got != want {
		t.Fatalf("layoutEditorContent()[0] = %q, want %q", got, want)
	}
	if got, want := layout.positions[len([]rune("hello "))].col, uniseg.StringWidth("hello [Pasted Text #1: 17 lines] "); got != want {
		t.Fatalf("cursor col = %d, want %d", got, want)
	}
}

func TestInputEditorBackspaceRemovesLastAttachmentWhenBufferEmpty(t *testing.T) {
	editor := &inputEditor{
		attachments: []inlineAttachment{
			{id: 1, pos: 0, kind: inlineAttachmentImage, path: "/tmp/one.png"},
			{id: 2, pos: 0, kind: inlineAttachmentPastedText, pasted: pastedBlock{content: "a\nb\nc\nd\ne", numLines: 5}},
		},
	}

	editor.backspace()
	if len(editor.attachments) != 1 {
		t.Fatalf("attachments len = %d, want 1", len(editor.attachments))
	}
	if got, want := editor.imagePaths(), []string{"/tmp/one.png"}; !sameStrings(got, want) {
		t.Fatalf("imagePaths() = %#v, want %#v", got, want)
	}

	editor.backspace()
	if len(editor.attachments) != 0 {
		t.Fatalf("attachments len = %d, want 0", len(editor.attachments))
	}
}

func TestInputEditorBackspaceRemovesLastAttachmentWhenCursorAtBufferEnd(t *testing.T) {
	editor := &inputEditor{
		buffer: []rune("fdsf"),
		cursor: 4,
		attachments: []inlineAttachment{
			{id: 1, pos: 4, kind: inlineAttachmentImage, path: "/tmp/one.png"},
			{id: 2, pos: 4, kind: inlineAttachmentPastedText, pasted: pastedBlock{content: "a\nb\nc\nd\ne", numLines: 5}},
		},
	}

	editor.backspace()
	if len(editor.attachments) != 1 {
		t.Fatalf("attachments len = %d, want 1", len(editor.attachments))
	}
	if got := string(editor.buffer); got != "fdsf" {
		t.Fatalf("buffer = %q, want unchanged text", got)
	}

	editor.backspace()
	if len(editor.attachments) != 0 {
		t.Fatalf("attachments len = %d, want 0", len(editor.attachments))
	}
	if got := string(editor.buffer); got != "fdsf" {
		t.Fatalf("buffer = %q, want unchanged text", got)
	}
}

func TestInputEditorKeepsCollapsedPasteAtOriginalInsertionPoint(t *testing.T) {
	editor := &inputEditor{
		buffer: []rune("hello "),
		cursor: len([]rune("hello ")),
	}

	editor.maybeCollapsePaste(strings.Repeat("line\n", 16) + "line")
	editor.insertText("dfkldjsfkl jsdf dfskjf")

	got := editor.displayText()
	want := "hello [Pasted Text #1: 17 lines] dfkldjsfkl jsdf dfskjf"
	if got != want {
		t.Fatalf("displayText() = %q, want %q", got, want)
	}
}

func TestInputEditorSubmitTextExpandsCollapsedPasteAtOriginalInsertionPoint(t *testing.T) {
	editor := &inputEditor{
		buffer: []rune("hello world"),
		attachments: []inlineAttachment{
			{
				id:   1,
				pos:  len([]rune("hello ")),
				kind: inlineAttachmentPastedText,
				pasted: pastedBlock{
					content:  "line 1\nline 2\n",
					numLines: 2,
				},
			},
		},
	}

	got := editor.submitText()
	want := "hello line 1\nline 2\nworld"
	if got != want {
		t.Fatalf("submitText() = %q, want %q", got, want)
	}
}

func TestInputEditorHelperLinesIncludeBackgroundLine(t *testing.T) {
	editor := &inputEditor{
		width:          80,
		padding:        1,
		contentWidth:   78,
		helperRows:     3,
		backgroundLine: "[sim-log] latest line",
	}

	lines := editor.helperLines(editorLayout{lines: []string{""}})
	if len(lines) < 3 {
		t.Fatalf("helperLines() len = %d, want at least 3", len(lines))
	}
	if !strings.Contains(lines[0], "[sim-log] latest line") {
		t.Fatalf("helperLines()[0] = %q, want background log line", lines[0])
	}
}

func TestInputEditorHelperLinesExpandForSlashMatches(t *testing.T) {
	editor := &inputEditor{
		width:        80,
		padding:      1,
		contentWidth: 78,
		helperRows:   3,
		buffer:       []rune("/"),
		cursor:       1,
	}

	lines := editor.helperLines(editorLayout{lines: []string{"/"}})
	if len(lines) <= 3 {
		t.Fatalf("helperLines() len = %d, want expanded helper area", len(lines))
	}
	if !strings.Contains(strings.Join(lines, "\n"), "/model") {
		t.Fatalf("helperLines() = %q, want additional slash command suggestions", lines)
	}
}

func TestRenderAttachmentLineUsesInputBoxStyling(t *testing.T) {
	editor := &inputEditor{width: 80, padding: 1, contentWidth: 78}

	line := editor.renderAttachmentLine("[Image #1]")

	if !strings.Contains(line, inputBoxBackground) {
		t.Fatalf("renderAttachmentLine() = %q, should use input box background", line)
	}
	if !strings.Contains(line, "[Image #1]") {
		t.Fatalf("renderAttachmentLine() = %q, want attachment text", line)
	}
}

func TestInputEditorRenderRepaintsWithoutCursorSaveRestore(t *testing.T) {
	editor := newRenderTestEditor()

	out := captureStdout(t, func() {
		withTerminalOutputLock(func() {
			editor.reserveRegionLocked()
			editor.renderLocked()
			editor.insertText("f")
			editor.renderLocked()
			editor.insertText("d")
			editor.renderLocked()
			editor.insertText("s")
			editor.renderLocked()
			editor.insertText("f")
			editor.renderLocked()
		})
	})

	if ansi := regexp.MustCompile(`\x1b\[[0-9;?]*[suLM]`); ansi.MatchString(out) {
		t.Fatalf("render output = %q, want no cursor save/restore or insert/delete line sequences", out)
	}

	screen := renderTerminalSnapshot(out)
	visible := strings.Join(screen.visibleLines(), "\n")
	if strings.Count(visible, "Ctrl+V attach image") != 1 {
		t.Fatalf("final screen = %q, want one input region", visible)
	}
	if strings.Count(visible, "fdsf") != 1 {
		t.Fatalf("final screen = %q, want final input text once", visible)
	}
}

func TestInputEditorRenderShrinksWithoutLeavingStaleLines(t *testing.T) {
	editor := newRenderTestEditor()
	editor.insertText("line 1\nline 2\nline 3")

	out := captureStdout(t, func() {
		withTerminalOutputLock(func() {
			editor.reserveRegionLocked()
			editor.renderLocked()
			editor.buffer = []rune("ok")
			editor.cursor = len(editor.buffer)
			editor.scrollTop = 0
			editor.renderLocked()
		})
	})

	screen := renderTerminalSnapshot(out)
	visible := strings.Join(screen.visibleLines(), "\n")
	if strings.Contains(visible, "line 2") || strings.Contains(visible, "line 3") {
		t.Fatalf("final screen = %q, want old multiline content cleared", visible)
	}
	if strings.Count(visible, "ok") != 1 {
		t.Fatalf("final screen = %q, want short replacement text once", visible)
	}
}

func TestInputEditorCleanupClearsRenderedRegion(t *testing.T) {
	editor := newRenderTestEditor()
	editor.insertText("keep screen clean")

	out := captureStdout(t, func() {
		withTerminalOutputLock(func() {
			editor.reserveRegionLocked()
			editor.renderLocked()
			editor.cleanupRegionLocked()
		})
	})

	screen := renderTerminalSnapshot(out)
	if visible := screen.visibleLines(); len(visible) != 0 {
		t.Fatalf("cleanup screen = %q, want cleared input region", strings.Join(visible, "\n"))
	}
}

func TestInputEditorBackgroundOutputStaysInOneRegion(t *testing.T) {
	editor := newRenderTestEditor()

	out := captureStdout(t, func() {
		withTerminalOutputLock(func() {
			lastBackgroundRenderAt = time.Time{}
			setActiveInputEditor(editor)
			defer clearActiveInputEditor(editor)

			editor.reserveRegionLocked()
			editor.renderLocked()
			printTerminalLineLocked("[sim-log] newest line")
		})
	})

	screen := renderTerminalSnapshot(out)
	visible := strings.Join(screen.visibleLines(), "\n")
	if strings.Count(visible, "[sim-log] newest line") != 1 {
		t.Fatalf("final screen = %q, want one background log line", visible)
	}
	if strings.Count(visible, "Ctrl+V attach image") != 1 {
		t.Fatalf("final screen = %q, want one input region", visible)
	}
}

func newRenderTestEditor() *inputEditor {
	return &inputEditor{
		width:          80,
		padding:        1,
		contentWidth:   78,
		maxVisibleRows: 5,
		helperRows:     3,
		preferredCol:   -1,
	}
}

type terminalSnapshot struct {
	lines [][]rune
	row   int
	col   int
}

func renderTerminalSnapshot(output string) terminalSnapshot {
	screen := terminalSnapshot{lines: [][]rune{{}}}

	for i := 0; i < len(output); {
		if output[i] == 0x1b && i+1 < len(output) && output[i+1] == '[' {
			j := i + 2
			for j < len(output) && (output[j] < 0x40 || output[j] > 0x7e) {
				j++
			}
			if j >= len(output) {
				break
			}
			screen.handleCSI(output[i+2:j], output[j])
			i = j + 1
			continue
		}

		r, size := utf8.DecodeRuneInString(output[i:])
		switch r {
		case '\r':
			screen.col = 0
		case '\n':
			screen.row++
			screen.ensureRow(screen.row)
		default:
			if unicode.IsGraphic(r) || unicode.IsSpace(r) {
				screen.writeRune(r)
			}
		}
		i += size
	}

	return screen
}

func (s *terminalSnapshot) handleCSI(params string, final byte) {
	switch final {
	case 'A':
		s.row = maxInt(0, s.row-parseCSIInt(params, 1))
	case 'B':
		s.row += parseCSIInt(params, 1)
		s.ensureRow(s.row)
	case 'C':
		s.col += parseCSIInt(params, 1)
	case 'J':
		s.clearBelow()
	case 'K':
		s.clearLine(parseCSIInt(params, 0))
	case 'h', 'l', 'm':
		// Ignore terminal modes and styling sequences in the snapshot.
	default:
		// Unsupported control sequences are intentionally ignored to simulate
		// terminals that do not honor advanced cursor save/restore behavior.
	}
}

func (s *terminalSnapshot) ensureRow(row int) {
	for len(s.lines) <= row {
		s.lines = append(s.lines, []rune{})
	}
}

func (s *terminalSnapshot) writeRune(r rune) {
	s.ensureRow(s.row)
	line := s.lines[s.row]
	for len(line) < s.col {
		line = append(line, ' ')
	}
	if s.col < len(line) {
		line[s.col] = r
	} else {
		line = append(line, r)
	}
	s.lines[s.row] = line
	s.col++
}

func (s *terminalSnapshot) clearBelow() {
	s.ensureRow(s.row)
	line := s.lines[s.row]
	if s.col < len(line) {
		line = line[:s.col]
	}
	s.lines[s.row] = line
	for i := s.row + 1; i < len(s.lines); i++ {
		s.lines[i] = []rune{}
	}
}

func (s *terminalSnapshot) clearLine(mode int) {
	s.ensureRow(s.row)
	switch mode {
	case 2:
		s.lines[s.row] = []rune{}
	default:
		line := s.lines[s.row]
		if s.col < len(line) {
			line = line[:s.col]
		}
		s.lines[s.row] = line
	}
}

func (s terminalSnapshot) visibleLines() []string {
	lines := make([]string, len(s.lines))
	last := -1
	for i, line := range s.lines {
		trimmed := strings.TrimRightFunc(string(line), unicode.IsSpace)
		lines[i] = trimmed
		if trimmed != "" {
			last = i
		}
	}
	if last < 0 {
		return nil
	}
	return lines[:last+1]
}

func parseCSIInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value := 0
	for _, r := range raw {
		if r < '0' || r > '9' {
			return fallback
		}
		value = value*10 + int(r-'0')
	}
	if value <= 0 {
		return fallback
	}
	return value
}

func createTestImage(t *testing.T, dir, name string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("image-data"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func escapePath(path string) string {
	var sb strings.Builder
	for _, r := range path {
		switch r {
		case ' ', '(', ')', '[', ']', '&':
			sb.WriteByte('\\')
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func finderQuotedPath(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "\\'") + "'"
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
