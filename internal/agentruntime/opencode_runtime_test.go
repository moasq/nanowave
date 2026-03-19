package agentruntime

import "testing"

func TestTranslateOpenCodePayloadText(t *testing.T) {
	payload := map[string]any{
		"type":      "text",
		"sessionID": "session-42",
		"part": map[string]any{
			"text": "{\"app_name\":",
		},
	}

	update := translateOpenCodePayload(payload)
	if update.SessionID != "session-42" {
		t.Fatalf("SessionID = %q, want %q", update.SessionID, "session-42")
	}
	if update.DeltaText != "{\"app_name\":" {
		t.Fatalf("DeltaText = %q", update.DeltaText)
	}
	if len(update.Events) != 2 {
		t.Fatalf("len(Events) = %d, want 2", len(update.Events))
	}
	if update.Events[0].Type != "system" || update.Events[1].Type != "content_block_delta" {
		t.Fatalf("Events = %#v", update.Events)
	}
}

func TestTranslateOpenCodePayloadToolUse(t *testing.T) {
	payload := map[string]any{
		"type": "tool_use",
		"part": map[string]any{
			"type": "tool",
			"state": map[string]any{
				"status": "completed",
				"tool": map[string]any{
					"name": "bash",
					"parameters": map[string]any{
						"command": "pwd",
					},
				},
			},
		},
	}

	update := translateOpenCodePayload(payload)
	if len(update.Events) != 1 {
		t.Fatalf("len(Events) = %d, want 1", len(update.Events))
	}
	if update.Events[0].Type != "tool_use" || update.Events[0].ToolName != "Bash" {
		t.Fatalf("Events[0] = %#v", update.Events[0])
	}
}

func TestBuildOpenCodeExecArgsOmitsPrintLogs(t *testing.T) {
	args := buildOpenCodeExecArgs("hello", GenerateOpts{
		SessionID: "session-42",
		Model:     "anthropic/test",
		Images:    []string{"/tmp/mockup.png"},
	}, "fallback-model")

	for _, arg := range args {
		if arg == "--print-logs" {
			t.Fatalf("args = %#v, should not include --print-logs", args)
		}
	}
	if len(args) == 0 || args[0] != "run" {
		t.Fatalf("args = %#v, want run command", args)
	}
}
