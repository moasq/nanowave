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
