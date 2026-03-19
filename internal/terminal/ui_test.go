package terminal

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolStatusShowsSignedInWithoutEmail(t *testing.T) {
	out := captureStdout(t, func() {
		ToolStatus(ToolStatusOpts{
			RuntimeVersion: "codex-cli 0.114.0",
			HasXcode:       true,
			HasSimulator:   true,
			HasXcodegen:    true,
			AuthLoggedIn:   true,
			AuthDetail:     "Logged in using ChatGPT",
		})
	})

	// Should show tools without runtime name
	if !strings.Contains(out, "Tools:") {
		t.Fatalf("ToolStatus() output = %q, want Tools header", out)
	}
	if strings.Contains(out, "Codex") {
		t.Fatalf("ToolStatus() output = %q, should not contain runtime name", out)
	}
	if !strings.Contains(out, "Signed in") {
		t.Fatalf("ToolStatus() output = %q, want Signed in", out)
	}
	if !strings.Contains(out, "Logged in using ChatGPT") {
		t.Fatalf("ToolStatus() output = %q, want auth detail", out)
	}
}

func TestToolStatusHidesEmailAddress(t *testing.T) {
	out := captureStdout(t, func() {
		ToolStatus(ToolStatusOpts{
			RuntimeVersion: "claude 1.0.0",
			HasXcode:       true,
			HasSimulator:   true,
			HasXcodegen:    true,
			AuthLoggedIn:   true,
			AuthEmail:      "gmohammed0020@gmail.com",
			AuthPlan:       "max",
			AuthDetail:     "Logged in using ChatGPT",
		})
	})

	if strings.Contains(out, "gmohammed0020@gmail.com") {
		t.Fatalf("ToolStatus() output = %q, should hide account email", out)
	}
	if !strings.Contains(out, "Signed in") {
		t.Fatalf("ToolStatus() output = %q, want signed-in status", out)
	}
	if !strings.Contains(out, "Max plan") {
		t.Fatalf("ToolStatus() output = %q, want plan label", out)
	}
}

func TestQueuedInputLinesIncludeTextAndAttachments(t *testing.T) {
	lines := queuedInputLines("Build a habit tracker\nwith widgets", []string{
		filepath.Join("/tmp", "mockup.png"),
	})

	if len(lines) < 2 {
		t.Fatalf("queuedInputLines() len = %d, want at least 2", len(lines))
	}
	if got := lines[len(lines)-1]; !strings.Contains(got, "with widgets [Image #1]") {
		t.Fatalf("queuedInputLines() last line = %q, want inline attachment chip", got)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Build a habit tracker") {
		t.Fatalf("queuedInputLines() = %q, want submitted text", joined)
	}
	if !strings.Contains(joined, "with widgets [Image #1]") {
		t.Fatalf("queuedInputLines() = %q, want multiline content", joined)
	}
}

func TestQueuedInputLinesMergeImageAndPastedTextMarkers(t *testing.T) {
	lines := queuedInputLines("/agent [Pasted Text #1: 3 lines]", []string{
		filepath.Join("/tmp", "mockup.png"),
	})

	if len(lines) != 1 {
		t.Fatalf("queuedInputLines() len = %d, want 1", len(lines))
	}
	if got, want := lines[0], "/agent [Image #1] [Pasted Text #1: 3 lines]"; got != want {
		t.Fatalf("queuedInputLines()[0] = %q, want %q", got, want)
	}
}

func TestEchoInputPrintsSubmittedTranscript(t *testing.T) {
	out := captureStdout(t, func() {
		EchoInput("Build a notes app", []string{filepath.Join("/tmp", "wireframe.png")})
	})

	if !strings.Contains(out, "Build a notes app") {
		t.Fatalf("EchoInput() output = %q, want submitted text", out)
	}
	if !strings.Contains(out, "[Image #1]") {
		t.Fatalf("EchoInput() output = %q, want attachment summary", out)
	}
	if strings.Contains(out, "Queued") {
		t.Fatalf("EchoInput() output = %q, should not include queue label", out)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}

	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy() error = %v", err)
	}
	return buf.String()
}
