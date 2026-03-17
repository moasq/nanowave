package terminal

import (
	"strings"
	"testing"
	"time"
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

func TestOnAgentCommentaryAddsActivityAndDeduplicates(t *testing.T) {
	pd := NewProgressDisplay("agentic", 0)

	pd.OnAgentCommentary("Inspecting runtime wiring. Then patching.")
	if len(pd.activities) != 1 {
		t.Fatalf("len(activities) = %d, want 1", len(pd.activities))
	}
	if pd.activities[0].text != "Inspecting runtime wiring" {
		t.Fatalf("activities[0].text = %q", pd.activities[0].text)
	}

	pd.OnAgentCommentary("Inspecting runtime wiring. More detail.")
	if len(pd.activities) != 1 {
		t.Fatalf("len(activities) after duplicate = %d, want 1", len(pd.activities))
	}

	pd.OnAgentCommentary("Patching the Codex adapter.")
	if len(pd.activities) != 2 {
		t.Fatalf("len(activities) after new message = %d, want 2", len(pd.activities))
	}
	if !pd.activities[0].done {
		t.Fatal("expected first commentary activity to be marked done")
	}
	if pd.activities[1].text != "Patching the Codex adapter" {
		t.Fatalf("activities[1].text = %q", pd.activities[1].text)
	}
}

func TestBuildPhaseHeaderIncludesRuntimeLabel(t *testing.T) {
	pd := NewProgressDisplay("plan", 0)
	got := pd.buildPhaseHeader(PhasePlanning, "Codex", 5, 0, 0, false, "•", 7*time.Second)

	if !strings.Contains(got, "[Codex]") {
		t.Fatalf("buildPhaseHeader() = %q, want runtime label", got)
	}
}
