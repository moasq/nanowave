package agentruntime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestParseCodexModelCatalog(t *testing.T) {
	data := []byte(`
model = "gpt-5.4"
model_reasoning_effort = "xhigh"

[profiles.reasoning]
model = "o3"

[notice.model_migrations]
"gpt-5.3-codex" = "gpt-5.4"
`)

	catalog := parseCodexModelCatalog(data)
	if catalog.Default != "gpt-5.4" {
		t.Fatalf("Default = %q, want %q", catalog.Default, "gpt-5.4")
	}

	want := []string{"gpt-5.4", "o3", "gpt-5.3-codex"}
	if len(catalog.Models) != len(want) {
		t.Fatalf("len(Models) = %d, want %d", len(catalog.Models), len(want))
	}
	for i, id := range want {
		if catalog.Models[i].ID != id {
			t.Fatalf("Models[%d] = %q, want %q", i, catalog.Models[i].ID, id)
		}
	}
}

func TestParseCodexConfigNormalizesReasoningEffort(t *testing.T) {
	data := []byte(`
model = "gpt-5.4"
model_reasoning_effort = "xhigh"
`)

	cfg := parseCodexConfig(data)
	if cfg.ReasoningEffort != "high" {
		t.Fatalf("ReasoningEffort = %q, want %q", cfg.ReasoningEffort, "high")
	}
}

func TestCodexRuntimeReadsModelsFromCodexHome(t *testing.T) {
	codexHome := t.TempDir()
	configPath := filepath.Join(codexHome, "config.toml")
	config := `model = "gpt-5.4"

[notice.model_migrations]
"gpt-5.3-codex" = "gpt-5.4"
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("CODEX_HOME", codexHome)

	runtime := NewCodex("codex")
	if got := runtime.DefaultModel(PhaseBuild); got != "gpt-5.4" {
		t.Fatalf("DefaultModel() = %q, want %q", got, "gpt-5.4")
	}

	models := runtime.SuggestedModels()
	if len(models) != 2 {
		t.Fatalf("len(SuggestedModels()) = %d, want 2", len(models))
	}
	if models[0].ID != "gpt-5.4" || models[1].ID != "gpt-5.3-codex" {
		t.Fatalf("SuggestedModels() = %#v", models)
	}
}

func TestBuildCodexExecArgsResumeUsesFullAutoAndTopLevelCD(t *testing.T) {
	args := buildCodexExecArgs("/tmp/out.txt", GenerateOpts{
		SessionID: "session-123",
		WorkDir:   "/tmp/workspace",
	})

	if !slices.Contains(args, "resume") {
		t.Fatalf("buildCodexExecArgs() = %#v, want resume", args)
	}
	if !slices.Contains(args, "--full-auto") {
		t.Fatalf("buildCodexExecArgs() = %#v, want --full-auto", args)
	}
	cdIndex := slices.Index(args, "-C")
	execIndex := slices.Index(args, "exec")
	if cdIndex < 0 || execIndex < 0 || cdIndex > execIndex {
		t.Fatalf("buildCodexExecArgs() = %#v, want top-level -C before exec", args)
	}
	if args[cdIndex+1] != "/tmp/workspace" {
		t.Fatalf("buildCodexExecArgs() workdir = %q, want %q", args[cdIndex+1], "/tmp/workspace")
	}
	if got := args[len(args)-2:]; len(got) != 2 || got[0] != "-o" || got[1] != "/tmp/out.txt" {
		t.Fatalf("buildCodexExecArgs() tail = %#v, want output flag", got)
	}
}

func TestTranslateCodexPayloadCommandExecution(t *testing.T) {
	payload := map[string]any{
		"type": "item.started",
		"item": map[string]any{
			"type":    "command_execution",
			"command": "xcodegen generate",
		},
	}

	update := translateCodexPayload(payload)
	if len(update.Events) != 2 {
		t.Fatalf("len(Events) = %d, want 2", len(update.Events))
	}
	if update.Events[0].Type != "tool_use_start" || update.Events[0].ToolName != "Bash" {
		t.Fatalf("Events[0] = %#v", update.Events[0])
	}
	if update.Events[1].Type != "tool_input_delta" {
		t.Fatalf("Events[1] = %#v", update.Events[1])
	}
	var input map[string]any
	if err := json.Unmarshal([]byte(update.Events[1].Text), &input); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if input["command"] != "xcodegen generate" {
		t.Fatalf("command = %#v, want %q", input["command"], "xcodegen generate")
	}
}

func TestTranslateCodexPayloadCommandExecutionUnderscoreEvent(t *testing.T) {
	payload := map[string]any{
		"type": "item_started",
		"item": map[string]any{
			"type":    "command_execution",
			"command": "xcodegen generate",
		},
	}

	update := translateCodexPayload(payload)
	if len(update.Events) != 2 {
		t.Fatalf("len(Events) = %d, want 2", len(update.Events))
	}
	if update.Events[0].Type != "tool_use_start" || update.Events[0].ToolName != "Bash" {
		t.Fatalf("Events[0] = %#v", update.Events[0])
	}
}

func TestTranslateCodexPayloadAgentMessageDelta(t *testing.T) {
	payload := map[string]any{
		"type": "agent_message_delta",
		"text": "{\"app_name\":",
	}

	update := translateCodexPayload(payload)
	if update.DeltaText != "{\"app_name\":" {
		t.Fatalf("DeltaText = %q", update.DeltaText)
	}
	if len(update.Events) != 1 || update.Events[0].Type != "content_block_delta" {
		t.Fatalf("Events = %#v", update.Events)
	}
}

func TestTranslateCodexPayloadEventMsgCommentary(t *testing.T) {
	payload := map[string]any{
		"type": "event_msg",
		"payload": map[string]any{
			"type":    "agent_message",
			"message": "Inspecting runtime wiring. Then patching.",
			"phase":   "commentary",
		},
	}

	update := translateCodexPayload(payload)
	if update.FullText != "" {
		t.Fatalf("FullText = %q, want empty for commentary", update.FullText)
	}
	if len(update.Events) != 1 {
		t.Fatalf("len(Events) = %d, want 1", len(update.Events))
	}
	if update.Events[0].Type != "assistant" || update.Events[0].Subtype != "commentary" {
		t.Fatalf("Events[0] = %#v", update.Events[0])
	}
	if update.Events[0].Text != "Inspecting runtime wiring. Then patching." {
		t.Fatalf("Events[0].Text = %q", update.Events[0].Text)
	}
}

func TestTranslateCodexPayloadResponseItemCommentary(t *testing.T) {
	payload := map[string]any{
		"type": "response_item",
		"payload": map[string]any{
			"type":  "message",
			"role":  "assistant",
			"phase": "commentary",
			"content": []any{
				map[string]any{
					"type": "output_text",
					"text": "Inspecting runtime wiring. Then patching.",
				},
			},
		},
	}

	update := translateCodexPayload(payload)
	if update.FullText != "" {
		t.Fatalf("FullText = %q, want empty for commentary", update.FullText)
	}
	if len(update.Events) != 1 {
		t.Fatalf("len(Events) = %d, want 1", len(update.Events))
	}
	if update.Events[0].Type != "assistant" || update.Events[0].Subtype != "commentary" {
		t.Fatalf("Events[0] = %#v", update.Events[0])
	}
}

func TestTranslateCodexPayloadThreadStarted(t *testing.T) {
	payload := map[string]any{
		"type":      "thread.started",
		"thread_id": "thread-123",
	}

	update := translateCodexPayload(payload)
	if update.SessionID != "thread-123" {
		t.Fatalf("SessionID = %q, want %q", update.SessionID, "thread-123")
	}
	if len(update.Events) != 1 || update.Events[0].SessionID != "thread-123" || update.Events[0].Type != "system" {
		t.Fatalf("Events = %#v", update.Events)
	}
}

func TestTranslateCodexPayloadFileChangeFromChangesArray(t *testing.T) {
	payload := map[string]any{
		"type": "item.completed",
		"item": map[string]any{
			"type": "file_change",
			"changes": []any{
				map[string]any{
					"path": "/tmp/test.txt",
					"kind": "update",
				},
			},
			"status": "completed",
		},
	}

	update := translateCodexPayload(payload)
	if len(update.Events) != 1 {
		t.Fatalf("len(Events) = %d, want 1", len(update.Events))
	}
	if update.Events[0].Type != "tool_use" || update.Events[0].ToolName != "Edit" {
		t.Fatalf("Events[0] = %#v", update.Events[0])
	}
	var input map[string]any
	if err := json.Unmarshal(update.Events[0].ToolInput, &input); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if input["file_path"] != "/tmp/test.txt" {
		t.Fatalf("file_path = %#v, want %q", input["file_path"], "/tmp/test.txt")
	}
}
