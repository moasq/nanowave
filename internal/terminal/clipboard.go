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

// hasClipboardImage checks if the macOS clipboard contains image data.
func hasClipboardImage() bool {
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

// saveClipboardImageToFile extracts clipboard image data and writes it to destPath.
func saveClipboardImageToFile(destPath string) error {
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

// pasteClipboardImage attempts to read an image from the clipboard,
// saves it to a temp file, and records it for the current input.
// Returns true if an image was pasted.
func pasteClipboardImage() bool {
	if !hasClipboardImage() {
		return false
	}

	clipboardState.mu.Lock()
	if clipboardState.tempDir == "" {
		dir, err := os.MkdirTemp("", "nanowave-clipboard-*")
		if err != nil {
			clipboardState.mu.Unlock()
			return false
		}
		clipboardState.tempDir = dir
	}
	clipboardState.counter++
	counter := clipboardState.counter
	tempDir := clipboardState.tempDir
	clipboardState.mu.Unlock()

	filename := fmt.Sprintf("paste_%d.png", counter)
	destPath := filepath.Join(tempDir, filename)

	if err := saveClipboardImageToFile(destPath); err != nil {
		return false
	}

	// Verify the file was actually written
	info, err := os.Stat(destPath)
	if err != nil || info.Size() < 8 {
		os.Remove(destPath)
		return false
	}

	clipboardState.mu.Lock()
	clipboardState.images = append(clipboardState.images, destPath)
	clipboardState.mu.Unlock()

	return true
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
