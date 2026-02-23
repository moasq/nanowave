package terminal

import (
	"strings"
	"testing"
)

func assertFriendlyStructuredStatus(t *testing.T, got string) {
	t.Helper()
	if strings.TrimSpace(got) == "" {
		t.Fatalf("expected non-empty structured status")
	}
	if strings.HasPrefix(got, "JSON: ") {
		t.Fatalf("expected friendly structured status, got raw JSON preview %q", got)
	}
	if strings.Contains(got, "\n") || strings.Contains(got, "\t") {
		t.Fatalf("expected one-line status without control whitespace, got %q", got)
	}
	if n := len([]rune(got)); n > maxStatusWidth {
		t.Fatalf("expected status length <= %d, got %d (%q)", maxStatusWidth, n, got)
	}
}

func TestExtractStreamingPreviewStructuredShowsJSONContent(t *testing.T) {
	got := extractStreamingPreview("{\n  \"app\": \"PulseTrack\"\n}", "plan")
	assertFriendlyStructuredStatus(t, got)
}

func TestExtractStreamingPreviewStructuredWorksWithoutNewlineAndShowsLatestTail(t *testing.T) {
	raw := `{"app":"PulseTrack","files":[{"path":"` + strings.Repeat("Feature", 30) + `-END.swift"}]}`
	got := extractStreamingPreview(raw, "plan")
	assertFriendlyStructuredStatus(t, got)
}

func TestExtractStreamingPreviewStructuredReturnsFriendlyLabel(t *testing.T) {
	raw := "{\n\t\"files\": [\n\t\t{\"path\": \"A.swift\"},\n\t\t{\"path\": \"" + strings.Repeat("VeryLongFileName", 12) + "\"}\n\t]\n}"
	got := extractStreamingPreview(raw, "analyze")
	assertFriendlyStructuredStatus(t, got)
}

func TestExtractStreamingPreviewNonStructuredStillHidesJSONLines(t *testing.T) {
	got := extractStreamingPreview("{\n\"files\": []\n}", "build")
	if len(got) != 0 {
		t.Fatalf("expected empty preview for build-mode JSON-like stream, got %q", got)
	}
}

func TestOnStreamingTextPlanModeUpdatesStatusWithJSON(t *testing.T) {
	pd := NewProgressDisplay("plan", 0)
	pd.OnStreamingText("{")
	pd.OnStreamingText(`"files":[{"path":"Models/HeartRate.swift"}]}`)
	assertFriendlyStructuredStatus(t, pd.statusText)
}

func TestOnStreamingTextBuildModeJSONBehaviorUnchanged(t *testing.T) {
	pd := NewProgressDisplay("build", 0)
	pd.OnStreamingText("{\n")
	pd.OnStreamingText("\"files\": []\n}")

	if len(pd.statusText) != 0 {
		t.Fatalf("expected empty status for build-mode JSON-like stream, got %q", pd.statusText)
	}
}

func TestOnAssistantTextPlanModeKeepsStructuredPreview(t *testing.T) {
	pd := NewProgressDisplay("plan", 0)
	pd.OnAssistantText("{\n  \"models\": []\n}")
	assertFriendlyStructuredStatus(t, pd.statusText)
}
