package terminal

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestInputEditorHelperLinesIncludeBackgroundLine(t *testing.T) {
	editor := &inputEditor{
		width:          80,
		padding:        1,
		contentWidth:   78,
		helperRows:     3,
		backgroundLine: "[sim-log] latest line",
	}

	lines := editor.helperLines()
	if len(lines) != 3 {
		t.Fatalf("helperLines() len = %d, want 3", len(lines))
	}
	if !strings.Contains(lines[0], "[sim-log] latest line") {
		t.Fatalf("helperLines()[0] = %q, want background log line", lines[0])
	}
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
