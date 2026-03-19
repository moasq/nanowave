package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// clipboardState tracks images pasted via Ctrl+V during a readline session.
var clipboardState struct {
	mu      sync.Mutex
	images  []string // temp file paths of pasted images
	tempDir string   // reusable temp directory for clipboard images
	counter int      // monotonic counter for unique filenames
}

func ensureClipboardTempDir() (string, bool) {
	clipboardState.mu.Lock()
	defer clipboardState.mu.Unlock()

	if clipboardState.tempDir == "" {
		dir, err := os.MkdirTemp("", "nanowave-clipboard-*")
		if err != nil {
			return "", false
		}
		clipboardState.tempDir = dir
	}
	return clipboardState.tempDir, true
}

func nextClipboardFilename(ext string) (string, string, bool) {
	tempDir, ok := ensureClipboardTempDir()
	if !ok {
		return "", "", false
	}

	clipboardState.mu.Lock()
	clipboardState.counter++
	counter := clipboardState.counter
	clipboardState.mu.Unlock()

	filename := fmt.Sprintf("paste_%d%s", counter, ext)
	return filename, filepath.Join(tempDir, filename), true
}

func appendClipboardImages(paths []string) {
	if len(paths) == 0 {
		return
	}

	clipboardState.mu.Lock()
	clipboardState.images = append(clipboardState.images, paths...)
	clipboardState.mu.Unlock()
}

func hasClipboardRasterImage() bool {
	cmd := exec.Command("osascript", "-e", `try
  the clipboard as «class PNGf»
  return "yes"
on error
  return "no"
end try`)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "yes"
}

func clipboardImageFilePaths() []string {
	cmd := exec.Command("osascript", "-e", `set AppleScript's text item delimiters to linefeed
try
  set outputLines to {}
  set clipboardItems to the clipboard as alias list
  repeat with itemRef in clipboardItems
    set end of outputLines to POSIX path of itemRef
  end repeat
  return outputLines as text
on error
  try
    return POSIX path of (the clipboard as alias)
  on error
    return ""
  end try
end try`)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var images []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !isImagePath(line) {
			continue
		}
		images = append(images, resolveImagePath(line))
	}
	return uniqueStrings(images)
}

// saveClipboardImageToFile extracts clipboard image data and writes it to destPath.
func saveClipboardRasterImage(destPath string) error {
	script := fmt.Sprintf(`set theFile to POSIX file %q
try
  set theImage to the clipboard as «class PNGf»
  set fRef to open for access theFile with write permission
  set eof fRef to 0
  write theImage to fRef
  close access fRef
  return "ok"
on error errMsg
  try
    close access theFile
  end try
  return "error:" & errMsg
end try`, destPath)
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("osascript failed: %w", err)
	}
	result := strings.TrimSpace(string(out))
	if strings.HasPrefix(result, "error:") {
		return fmt.Errorf("clipboard read failed: %s", result)
	}
	return nil
}

func pasteClipboardRasterImage() (string, bool) {
	if !hasClipboardRasterImage() {
		return "", false
	}

	_, destPath, ok := nextClipboardFilename(".png")
	if !ok {
		return "", false
	}
	if err := saveClipboardRasterImage(destPath); err != nil {
		return "", false
	}

	// Verify the file was actually written
	info, err := os.Stat(destPath)
	if err != nil || info.Size() < 8 {
		os.Remove(destPath)
		return "", false
	}

	return destPath, true
}

// pasteClipboardImages attaches Finder image files first, then falls back to
// raster image data from the clipboard. It returns newly attached image paths.
func pasteClipboardImages() []string {
	if images := clipboardImageFilePaths(); len(images) > 0 {
		appendClipboardImages(images)
		return images
	}

	imagePath, ok := pasteClipboardRasterImage()
	if !ok {
		return nil
	}
	appendClipboardImages([]string{imagePath})
	return []string{imagePath}
}

// takeClipboardImages returns and clears any images pasted during the current input.
func takeClipboardImages() []string {
	clipboardState.mu.Lock()
	defer clipboardState.mu.Unlock()
	images := clipboardState.images
	clipboardState.images = nil
	return images
}

// CleanupClipboard removes the clipboard temp directory.
func CleanupClipboard() {
	clipboardState.mu.Lock()
	defer clipboardState.mu.Unlock()
	if clipboardState.tempDir != "" {
		os.RemoveAll(clipboardState.tempDir)
		clipboardState.tempDir = ""
	}
	clipboardState.images = nil
}

// clipboardImageCount returns how many images have been pasted in the current input.
func clipboardImageCount() int {
	clipboardState.mu.Lock()
	defer clipboardState.mu.Unlock()
	return len(clipboardState.images)
}
