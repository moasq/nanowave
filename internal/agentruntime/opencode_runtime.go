package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type OpenCodeRuntime struct {
	path string
}

func NewOpenCode(path string) *OpenCodeRuntime {
	return &OpenCodeRuntime{path: path}
}

func (r *OpenCodeRuntime) Kind() Kind {
	return KindOpenCode
}

func (r *OpenCodeRuntime) DisplayName() string {
	return DescriptorForKind(KindOpenCode).DisplayName
}

func (r *OpenCodeRuntime) DefaultModel(_ Phase) string {
	return openCodeModelCatalog().Default
}

func (r *OpenCodeRuntime) SuggestedModels() []ModelOption {
	return openCodeModelCatalog().Models
}

func (r *OpenCodeRuntime) SupportsInteractive() bool {
	return false
}

func (r *OpenCodeRuntime) Generate(ctx context.Context, userMessage string, opts GenerateOpts) (*Response, error) {
	return r.GenerateStreaming(ctx, userMessage, opts, nil)
}

func (r *OpenCodeRuntime) GenerateStreaming(ctx context.Context, userMessage string, opts GenerateOpts, onEvent func(StreamEvent)) (*Response, error) {
	prompt := buildPromptWithSystem(userMessage, opts)
	args := buildOpenCodeExecArgs(prompt, opts, r.DefaultModel(PhaseBuild))

	cmd := exec.CommandContext(ctx, r.path, args...)
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}
	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start opencode: %w", err)
	}

	var sessionID string
	var assistantText strings.Builder
	var stdoutRaw strings.Builder
	var stderrText strings.Builder
	var lastErrMsg string

	var stdoutErr error
	var stderrErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		stdoutErr = streamTextLines(stdout, func(line string) error {
			appendCapturedLine(&stdoutRaw, line)
			payload := decodeJSONLine([]byte(line))
			if payload == nil {
				return nil
			}

			update := translateOpenCodePayload(payload)
			if update.SessionID != "" {
				sessionID = update.SessionID
			}
			if update.LastError != "" {
				lastErrMsg = update.LastError
			}
			if update.DeltaText != "" {
				assistantText.WriteString(update.DeltaText)
			}
			if update.FullText != "" {
				assistantText.Reset()
				assistantText.WriteString(update.FullText)
			}
			if onEvent != nil {
				for _, ev := range update.Events {
					onEvent(ev)
				}
			}
			return nil
		})
	}()
	go func() {
		defer wg.Done()
		stderrErr = streamTextLines(stderr, func(line string) error {
			appendCapturedLine(&stderrText, line)
			return nil
		})
	}()

	waitErr := cmd.Wait()
	wg.Wait()
	if stdoutErr != nil {
		return nil, fmt.Errorf("failed to read opencode stream: %w", stdoutErr)
	}
	if stderrErr != nil {
		return nil, fmt.Errorf("failed to read opencode logs: %w", stderrErr)
	}
	if waitErr != nil {
		if stderrText.Len() > 0 {
			return nil, fmt.Errorf("opencode command failed: %w\nstderr: %s", waitErr, stderrText.String())
		}
		if lastErrMsg != "" {
			return nil, fmt.Errorf("opencode command failed: %w\n%s", waitErr, lastErrMsg)
		}
		return nil, fmt.Errorf("opencode command failed: %w", waitErr)
	}

	result := strings.TrimSpace(assistantText.String())
	if result == "" {
		result = strings.TrimSpace(stdoutRaw.String())
	}
	if result == "" {
		result = strings.TrimSpace(stderrText.String())
	}
	if sessionID == "" {
		sessionID = opts.SessionID
	}
	return &Response{Result: result, SessionID: sessionID}, nil
}

func buildOpenCodeExecArgs(prompt string, opts GenerateOpts, defaultModel string) []string {
	args := []string{"run", "--format", "json"}
	if opts.SessionID != "" {
		args = append(args, "--session", opts.SessionID)
	}
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = defaultModel
	}
	if model != "" {
		args = append(args, "-m", model)
	}
	for _, img := range opts.Images {
		args = append(args, "-f", img)
	}
	args = append(args, prompt)
	return args
}

func (r *OpenCodeRuntime) RunInteractive(_ context.Context, _ string, _ InteractiveOpts, _ func(StreamEvent), _ func(question string) string) (*Response, error) {
	return nil, fmt.Errorf("interactive sessions are not supported by the OpenCode adapter yet")
}

func CheckOpenCodeAuth(path string) *AuthStatus {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	cmd := exec.Command(path, "auth", "list")
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil && text == "" {
		return nil
	}
	status := &AuthStatus{Detail: summarizeCommandOutput(text)}
	if text != "" && !strings.Contains(strings.ToLower(text), "no credentials") {
		status.LoggedIn = true
	}
	return status
}

func OpenCodeVersion(path string) string {
	cmd := exec.Command(path, "--version")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func openCodeModelCatalog() discoveredModelCatalog {
	return discoverJSONModelCatalog(openCodeConfigPaths()...)
}

func openCodeConfigPaths() []string {
	paths := make([]string, 0, 6)
	if configPath := strings.TrimSpace(os.Getenv("OPENCODE_CONFIG")); configPath != "" {
		paths = append(paths, configPath)
	}
	if home := strings.TrimSpace(os.Getenv("OPENCODE_HOME")); home != "" {
		paths = append(paths,
			filepath.Join(home, "config.json"),
			filepath.Join(home, "config.local.json"),
		)
	}
	userHome, err := os.UserHomeDir()
	if err == nil && userHome != "" {
		paths = append(paths,
			filepath.Join(userHome, ".opencode", "config.json"),
			filepath.Join(userHome, ".opencode", "config.local.json"),
			filepath.Join(userHome, ".config", "opencode", "config.json"),
			filepath.Join(userHome, ".config", "opencode", "config.local.json"),
		)
	}
	return existingPaths(paths...)
}

type openCodeStreamUpdate struct {
	Events    []StreamEvent
	DeltaText string
	FullText  string
	SessionID string
	LastError string
}

func translateOpenCodePayload(payload map[string]any) openCodeStreamUpdate {
	update := openCodeStreamUpdate{}
	update.SessionID = firstNonEmpty(
		stringValue(payload["sessionID"]),
		stringValue(payload["session_id"]),
		stringValue(mapValue(payload["session"])["id"]),
	)
	if update.SessionID != "" {
		update.Events = append(update.Events, StreamEvent{Type: "system", Subtype: "init", SessionID: update.SessionID})
	}

	eventType := strings.ToLower(stringValue(payload["type"]))
	part := mapValue(payload["part"])
	switch {
	case eventType == "error":
		update.LastError = firstNonEmpty(
			extractString(part, "text", "message", "content"),
			extractString(payload, "message", "text", "content", "error"),
		)
		if update.LastError != "" {
			update.Events = append(update.Events, StreamEvent{Type: "assistant", Text: update.LastError})
		}
	case openCodeLooksLikeToolEvent(eventType, part, payload):
		toolName, toolInput, completed := openCodeToolEvent(eventType, part, payload)
		if toolName != "" {
			if completed {
				update.Events = append(update.Events, StreamEvent{Type: "tool_use", ToolName: toolName, ToolInput: toolInput})
			} else {
				update.Events = append(update.Events, StreamEvent{Type: "tool_use_start", ToolName: toolName})
				if len(toolInput) > 0 {
					update.Events = append(update.Events, StreamEvent{Type: "tool_input_delta", ToolName: toolName, Text: string(toolInput)})
				}
			}
		}
		if isFailedOpenCodePart(part, payload) {
			update.LastError = firstNonEmpty(
				extractString(part, "text", "message", "content"),
				extractString(mapValue(part["state"]), "text", "message", "content"),
				extractString(payload, "message", "text", "content"),
			)
			if update.LastError != "" {
				update.Events = append(update.Events, StreamEvent{Type: "assistant", Text: update.LastError})
			}
		}
	default:
		text := firstNonEmpty(
			extractString(part, "text", "message", "content", "result", "output"),
			extractString(payload, "text", "message", "content", "result", "output"),
		)
		if text == "" {
			return update
		}
		if strings.Contains(eventType, "delta") || eventType == "text" {
			update.DeltaText = text
			update.Events = append(update.Events, StreamEvent{Type: "content_block_delta", Text: text})
		} else {
			update.FullText = text
			update.Events = append(update.Events, StreamEvent{Type: "assistant", Text: text})
		}
	}

	return update
}

func openCodeLooksLikeToolEvent(eventType string, part map[string]any, payload map[string]any) bool {
	if eventType == "tool_use" || strings.HasPrefix(eventType, "tool") || strings.HasPrefix(eventType, "step_") {
		return true
	}
	if strings.EqualFold(stringValue(part["type"]), "tool") {
		return true
	}
	state := mapValue(part["state"])
	tool := mapValue(state["tool"])
	if len(tool) > 0 {
		return true
	}
	return normalizeRuntimeToolName(firstNonEmpty(
		stringValue(payload["tool"]),
		stringValue(part["name"]),
	)) != ""
}

func openCodeToolEvent(eventType string, part map[string]any, payload map[string]any) (string, json.RawMessage, bool) {
	state := mapValue(part["state"])
	tool := mapValue(state["tool"])
	if len(tool) == 0 {
		tool = mapValue(part["tool"])
	}
	if len(tool) == 0 {
		tool = mapValue(payload["tool"])
	}

	toolName := normalizeRuntimeToolName(firstNonEmpty(
		stringValue(tool["name"]),
		stringValue(part["name"]),
		stringValue(payload["name"]),
		stringValue(payload["tool"]),
	))
	if toolName == "" {
		switch {
		case firstNonEmpty(
			stringValue(tool["command"]),
			stringValue(part["command"]),
			stringValue(payload["command"]),
			stringValue(tool["cmd"]),
		) != "":
			toolName = "Bash"
		case firstNonEmpty(
			stringValue(tool["query"]),
			stringValue(part["query"]),
			stringValue(payload["query"]),
		) != "":
			toolName = "WebSearch"
		case firstNonEmpty(
			stringValue(tool["path"]),
			stringValue(part["path"]),
			stringValue(payload["path"]),
			stringValue(tool["file_path"]),
		) != "":
			toolName = "Edit"
		}
	}
	if toolName == "" {
		return "", nil, false
	}

	toolInput := rawJSON(firstNonNil(
		tool["parameters"],
		tool["input"],
		part["input"],
		payload["input"],
	))
	if len(toolInput) == 0 {
		switch toolName {
		case "Bash":
			toolInput = rawJSON(map[string]any{
				"command": firstNonEmpty(
					stringValue(tool["command"]),
					stringValue(part["command"]),
					stringValue(payload["command"]),
					stringValue(tool["cmd"]),
				),
			})
		case "WebSearch":
			toolInput = rawJSON(map[string]any{
				"query": firstNonEmpty(
					stringValue(tool["query"]),
					stringValue(part["query"]),
					stringValue(payload["query"]),
				),
			})
		case "Read", "Write", "Edit":
			toolInput = rawJSON(map[string]any{
				"file_path": firstNonEmpty(
					stringValue(tool["path"]),
					stringValue(part["path"]),
					stringValue(payload["path"]),
					stringValue(tool["file_path"]),
				),
			})
		}
	}

	status := strings.ToLower(firstNonEmpty(
		stringValue(state["status"]),
		stringValue(part["status"]),
		stringValue(payload["status"]),
	))
	completed := eventType == "step_finish" || strings.Contains(status, "complete") || strings.Contains(status, "finish") || strings.Contains(status, "done")
	if eventType == "step_start" || strings.Contains(status, "run") || strings.Contains(status, "start") || strings.Contains(status, "pending") {
		completed = false
	}
	if eventType == "tool_use" && status == "" {
		completed = true
	}

	return toolName, toolInput, completed
}

func isFailedOpenCodePart(part map[string]any, payload map[string]any) bool {
	status := strings.ToLower(firstNonEmpty(
		stringValue(mapValue(part["state"])["status"]),
		stringValue(part["status"]),
		stringValue(payload["status"]),
	))
	return strings.Contains(status, "fail") || strings.Contains(status, "error")
}
