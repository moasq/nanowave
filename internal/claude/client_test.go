package claude

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildImageContextAllowsImageOnlyPrompts(t *testing.T) {
	got := buildImageContext("", []string{"/tmp/example.png"})

	if strings.HasPrefix(got, "\n") {
		t.Fatalf("buildImageContext() added leading newline: %q", got)
	}
	if !strings.Contains(got, "Image 1: /tmp/example.png") {
		t.Fatalf("buildImageContext() missing image path: %q", got)
	}
}

func TestBuildImageContextKeepsPromptAndImages(t *testing.T) {
	got := buildImageContext("Inspect this", []string{"/tmp/example.png"})

	if !strings.HasPrefix(got, "Inspect this\n\n[Attached images") {
		t.Fatalf("buildImageContext() = %q", got)
	}
}

func TestStreamNDJSONLinesHandlesLargeLine(t *testing.T) {
	large := strings.Repeat("a", 1024*1024+128)
	input := []byte(large + "\n")

	var got [][]byte
	err := streamNDJSONLines(bytes.NewReader(input), func(line []byte) error {
		cp := append([]byte(nil), line...)
		got = append(got, cp)
		return nil
	})
	if err != nil {
		t.Fatalf("streamNDJSONLines() error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 line, got %d", len(got))
	}
	if len(got[0]) != len(large) {
		t.Fatalf("line length = %d, want %d", len(got[0]), len(large))
	}
}

func TestStreamNDJSONLinesProcessesFinalLineWithoutNewline(t *testing.T) {
	input := []byte("{\"a\":1}\n{\"b\":2}")

	var lines []string
	err := streamNDJSONLines(bytes.NewReader(input), func(line []byte) error {
		lines = append(lines, string(line))
		return nil
	})
	if err != nil {
		t.Fatalf("streamNDJSONLines() error = %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "{\"a\":1}" || lines[1] != "{\"b\":2}" {
		t.Fatalf("unexpected lines: %#v", lines)
	}
}

func TestStreamNDJSONLinesReturnsReaderError(t *testing.T) {
	wantErr := errors.New("boom")
	r := &failingReader{
		first: []byte("{\"ok\":true}\n"),
		err:   wantErr,
	}

	var lines []string
	err := streamNDJSONLines(r, func(line []byte) error {
		lines = append(lines, string(line))
		return nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("streamNDJSONLines() error = %v, want %v", err, wantErr)
	}
	if len(lines) != 1 || lines[0] != "{\"ok\":true}" {
		t.Fatalf("unexpected lines before error: %#v", lines)
	}
}

type failingReader struct {
	first []byte
	err   error
	done  bool
}

func (r *failingReader) Read(p []byte) (int, error) {
	if !r.done {
		r.done = true
		n := copy(p, r.first)
		if n < len(r.first) {
			r.first = r.first[n:]
			r.done = false
		}
		return n, nil
	}
	return 0, r.err
}

var _ io.Reader = (*failingReader)(nil)

func TestGenerateStreamingReturnsAfterResultEvenIfProcessLingers(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-claude.sh")
	body := `#!/bin/sh
/bin/echo '{"type":"system","subtype":"init","session_id":"sess-123"}'
/bin/echo '{"type":"assistant","message":{"content":[{"type":"text","text":"done"}]}}'
/bin/echo '{"type":"result","result":"done","session_id":"sess-123","cost_usd":1.5,"num_turns":2}'
sleep 5
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	client := NewClient(script)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	resp, err := client.GenerateStreaming(ctx, "hello", GenerateOpts{}, nil)
	if err != nil {
		t.Fatalf("GenerateStreaming() error = %v", err)
	}
	if elapsed := time.Since(start); elapsed >= 2*time.Second {
		t.Fatalf("GenerateStreaming() took %v, want it to finish before context timeout", elapsed)
	}
	if resp == nil {
		t.Fatal("GenerateStreaming() response = nil")
	}
	if got, want := resp.Result, "done"; got != want {
		t.Fatalf("GenerateStreaming() result = %q, want %q", got, want)
	}
	if got, want := resp.SessionID, "sess-123"; got != want {
		t.Fatalf("GenerateStreaming() sessionID = %q, want %q", got, want)
	}
}
