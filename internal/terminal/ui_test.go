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
			RuntimeName:    "Codex",
			RuntimeVersion: "codex-cli 0.114.0",
			HasXcode:       true,
			HasSimulator:   true,
			HasXcodegen:    true,
			AuthLoggedIn:   true,
			AuthDetail:     "Logged in using ChatGPT",
		})
	})

	if !strings.Contains(out, "Signed in") {
		t.Fatalf("ToolStatus() output = %q, want Signed in", out)
	}
	if strings.Contains(out, "Not signed in") {
		t.Fatalf("ToolStatus() output = %q, should not say Not signed in", out)
	}
	if !strings.Contains(out, "Logged in using ChatGPT") {
		t.Fatalf("ToolStatus() output = %q, want auth detail", out)
	}
}

func TestQueuedInputLinesIncludeTextAndAttachments(t *testing.T) {
	lines := queuedInputLines("Build a habit tracker\nwith widgets", []string{
		filepath.Join("/tmp", "mockup.png"),
	})

	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Build a habit tracker") {
		t.Fatalf("queuedInputLines() = %q, want submitted text", joined)
	}
	if !strings.Contains(joined, "with widgets") {
		t.Fatalf("queuedInputLines() = %q, want multiline content", joined)
	}
	if !strings.Contains(joined, "Attached: mockup.png") {
		t.Fatalf("queuedInputLines() = %q, want attachment line", joined)
	}
}

func TestEchoInputPrintsSubmittedTranscript(t *testing.T) {
	out := captureStdout(t, func() {
		EchoInput("Build a notes app", []string{filepath.Join("/tmp", "wireframe.png")})
	})

	if !strings.Contains(out, "Build a notes app") {
		t.Fatalf("EchoInput() output = %q, want submitted text", out)
	}
	if !strings.Contains(out, "Attached: wireframe.png") {
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
