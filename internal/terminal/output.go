package terminal

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	terminalOutputMu       sync.Mutex
	activeInputEditor      *inputEditor
	ansiEscapePattern      = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	lastBackgroundRenderAt time.Time
)

const backgroundRenderThrottle = 500 * time.Millisecond

func withTerminalOutputLock(fn func()) {
	terminalOutputMu.Lock()
	defer terminalOutputMu.Unlock()
	fn()
}

func writeStdoutUnlocked(s string) {
	_, _ = os.Stdout.WriteString(s)
}

func setActiveInputEditor(e *inputEditor) {
	activeInputEditor = e
}

func clearActiveInputEditor(e *inputEditor) {
	if activeInputEditor == e {
		activeInputEditor = nil
	}
}

func sanitizeInlineTerminalText(text string) string {
	text = ansiEscapePattern.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

func printTerminalLine(text string) {
	withTerminalOutputLock(func() {
		printTerminalLineLocked(text)
	})
}

func printTerminalLineLocked(text string) {
	if activeInputEditor != nil {
		inline := sanitizeInlineTerminalText(text)
		if inline != "" {
			activeInputEditor.pushBackgroundLineLocked(inline)
			// Throttle renders: only redraw if enough time has passed.
			// This prevents sim-log floods from corrupting the input UI.
			now := time.Now()
			if now.Sub(lastBackgroundRenderAt) >= backgroundRenderThrottle {
				lastBackgroundRenderAt = now
				activeInputEditor.renderLocked()
			}
		}
		return
	}
	_, _ = fmt.Fprintln(os.Stdout, text)
}

// OutputLine prints a line while respecting the active input renderer.
func OutputLine(text string) {
	printTerminalLine(text)
}
