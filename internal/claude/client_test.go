package claude

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

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
