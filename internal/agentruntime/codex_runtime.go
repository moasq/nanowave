package agentruntime

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type CodexRuntime struct {
	path string
}

func NewCodex(path string) *CodexRuntime {
	return &CodexRuntime{path: path}
}

func (r *CodexRuntime) Kind() Kind {
	return KindCodex
}

func (r *CodexRuntime) DisplayName() string {
	return DescriptorForKind(KindCodex).DisplayName
}

func (r *CodexRuntime) Descriptor() Descriptor {
	return DescriptorForKind(KindCodex)
}

func (r *CodexRuntime) BinaryPath() string {
	return r.path
}

func (r *CodexRuntime) Version() string {
	return CodexVersion(r.path)
}

func (r *CodexRuntime) AuthStatus() *AuthStatus {
	return CheckCodexAuth(r.path)
}

func (r *CodexRuntime) DefaultModel(_ Phase) string {
	return codexModelCatalog().Default
}

func (r *CodexRuntime) SuggestedModels() []ModelOption {
	return codexModelCatalog().Models
}

func (r *CodexRuntime) SupportsInteractive() bool {
	return false
}

func (r *CodexRuntime) Generate(ctx context.Context, userMessage string, opts GenerateOpts) (*Response, error) {
	return r.GenerateStreaming(ctx, userMessage, opts, nil)
}

func (r *CodexRuntime) GenerateStreaming(ctx context.Context, userMessage string, opts GenerateOpts, onEvent func(StreamEvent)) (*Response, error) {
	prompt := buildPromptWithSystem(userMessage, opts)

	outputFile, err := os.CreateTemp("", "nanowave-codex-output-*.txt")
	if err != nil {
		return nil, err
	}
	outputPath := outputFile.Name()
	_ = outputFile.Close()
	defer os.Remove(outputPath)

	args := buildCodexExecArgs(outputPath, opts)
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = r.DefaultModel(PhaseBuild)
	}
	if model != "" {
		args = append(args, "-m", model)
	}
	for _, img := range opts.Images {
		args = append(args, "-i", img)
	}
	if opts.JSONSchema != "" {
		schemaFile, err := os.CreateTemp("", "nanowave-codex-schema-*.json")
		if err != nil {
			return nil, err
		}
		if _, err := schemaFile.WriteString(opts.JSONSchema); err != nil {
			_ = schemaFile.Close()
			return nil, err
		}
		_ = schemaFile.Close()
		defer os.Remove(schemaFile.Name())
		args = append(args, "--output-schema", schemaFile.Name())
	}
	args = append(args, "-")

	cmd := exec.CommandContext(ctx, r.path, args...)
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}
	cmd.Stdin = strings.NewReader(prompt)
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
		return nil, fmt.Errorf("failed to start codex: %w", err)
	}

	var sessionID string
	var assistantText strings.Builder
	var lastErrMsg string
	var stderrText strings.Builder
	var totalCostUSD float64
	var usage Usage

	var stderrErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		stderrErr = streamTextLines(stderr, func(line string) error {
			appendCapturedLine(&stderrText, line)
			return nil
		})
	}()

	streamErr := streamNDJSONLines(stdout, func(line []byte) error {
		payload := decodeJSONLine(line)
		if payload == nil {
			return nil
		}
		update := translateCodexPayload(payload)
		if update.SessionID != "" {
			sessionID = update.SessionID
		}
		if update.LastError != "" {
			lastErrMsg = update.LastError
		}
		if update.HasUsage {
			usage = update.Usage
		}
		if update.TotalCostUSD > 0 {
			totalCostUSD = update.TotalCostUSD
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

	waitErr := cmd.Wait()
	wg.Wait()
	if streamErr != nil {
		return nil, fmt.Errorf("failed to read codex stream: %w", streamErr)
	}
	if stderrErr != nil {
		return nil, fmt.Errorf("failed to read codex logs: %w", stderrErr)
	}
	if waitErr != nil {
		if stderrText.Len() > 0 {
			return nil, fmt.Errorf("codex command failed: %w\nstderr: %s", waitErr, stderrText.String())
		}
		if lastErrMsg != "" {
			return nil, fmt.Errorf("codex command failed: %w\n%s", waitErr, lastErrMsg)
		}
		return nil, fmt.Errorf("codex command failed: %w", waitErr)
	}

	result := parseFinalOutputFile(outputPath)
	if result == "" {
		result = strings.TrimSpace(assistantText.String())
	}
	return &Response{
		Result:       result,
		SessionID:    sessionID,
		TotalCostUSD: totalCostUSD,
		Usage:        usage,
	}, nil
}

func (r *CodexRuntime) RunInteractive(_ context.Context, _ string, _ InteractiveOpts, _ func(StreamEvent), _ func(question string) string) (*Response, error) {
	return nil, fmt.Errorf("interactive sessions are not supported by the Codex adapter yet")
}

func CheckCodexAuth(path string) *AuthStatus {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	cmd := exec.Command(path, "login", "status")
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil && text == "" {
		return nil
	}
	lower := strings.ToLower(text)
	status := &AuthStatus{Detail: summarizeCommandOutput(text, "logged in", "authenticated")}
	if strings.Contains(lower, "logged in") && !strings.Contains(lower, "not logged in") {
		status.LoggedIn = true
	}
	if strings.Contains(lower, "authenticated") {
		status.LoggedIn = true
	}
	return status
}

func CodexVersion(path string) string {
	cmd := exec.Command(path, "--version")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func ParseCodexJSONEvent(line []byte) map[string]any {
	var payload map[string]any
	_ = json.Unmarshal(line, &payload)
	return payload
}

type codexStreamUpdate struct {
	Events       []StreamEvent
	DeltaText    string
	FullText     string
	SessionID    string
	LastError    string
	TotalCostUSD float64
	Usage        Usage
	HasUsage     bool
}

func translateCodexPayload(payload map[string]any) codexStreamUpdate {
	update := codexStreamUpdate{}
	eventType := normalizeCodexEventType(stringValue(payload["type"]))

	switch eventType {
	case "event_msg":
		return translateCodexEventMsg(mapValue(payload["payload"]))
	case "response_item":
		return translateCodexResponseItem(mapValue(payload["payload"]))
	case "thread_started":
		update.SessionID = firstNonEmpty(
			stringValue(payload["thread_id"]),
			stringValue(payload["session_id"]),
		)
		if update.SessionID != "" {
			update.Events = append(update.Events, StreamEvent{Type: "system", Subtype: "init", SessionID: update.SessionID})
		}
	case "turn_completed":
		update.TotalCostUSD = floatValue(payload["total_cost_usd"])
		if usage := parseUsageMap(mapValue(payload["usage"])); usage != nil {
			update.Usage = *usage
			update.HasUsage = true
		}
	case "turn_failed", "error":
		update.LastError = firstNonEmpty(
			extractString(payload, "message", "text", "content"),
			extractString(mapValue(payload["error"]), "message", "text", "content"),
		)
		if update.LastError != "" {
			update.Events = append(update.Events, StreamEvent{Type: "assistant", Text: update.LastError})
		}
	case "agent_message_delta", "assistant_message_delta", "agent_message_content_delta":
		update.DeltaText = firstNonEmpty(
			stringValue(payload["delta"]),
			extractString(payload, "text", "message", "content"),
		)
		if update.DeltaText != "" {
			update.Events = append(update.Events, StreamEvent{Type: "content_block_delta", Text: update.DeltaText})
		}
	case "agent_message", "assistant_message", "assistant":
		update.FullText = extractString(payload, "text", "message", "content")
		if update.FullText != "" {
			update.Events = append(update.Events, StreamEvent{Type: "assistant", Text: update.FullText})
		}
	case "item_started", "item_completed":
		itemUpdate := translateCodexItem(eventType, mapValue(payload["item"]))
		update.Events = append(update.Events, itemUpdate.Events...)
		update.DeltaText = itemUpdate.DeltaText
		update.FullText = itemUpdate.FullText
		update.LastError = itemUpdate.LastError
	default:
		text := firstNonEmpty(
			stringValue(payload["delta"]),
			extractString(payload, "message", "text", "content"),
		)
		if text == "" {
			text = extractString(mapValue(payload["item"]), "message", "text", "content")
		}
		if text != "" {
			if strings.Contains(eventType, "delta") || eventType == "text" {
				update.DeltaText = text
				update.Events = append(update.Events, StreamEvent{Type: "content_block_delta", Text: text})
			} else if strings.Contains(eventType, "message") || strings.Contains(eventType, "assistant") {
				update.FullText = text
				update.Events = append(update.Events, StreamEvent{Type: "assistant", Text: text})
			}
		}
	}

	return update
}

func translateCodexItem(eventType string, item map[string]any) codexStreamUpdate {
	update := codexStreamUpdate{}
	if len(item) == 0 {
		return update
	}

	eventType = normalizeCodexEventType(eventType)
	itemType := normalizeCodexEventType(firstNonEmpty(
		stringValue(item["type"]),
		stringValue(item["kind"]),
	))
	switch itemType {
	case "command_execution", "local_shell_call", "shell":
		tool := codexToolProgress(eventType, "Bash", map[string]any{
			"command": firstNonEmpty(
				stringValue(item["command"]),
				extractString(mapValue(item["input"]), "command", "cmd"),
			),
		})
		update.Events = append(update.Events, tool...)
		if isFailedStatus(item) {
			update.LastError = firstNonEmpty(
				extractString(item, "aggregated_output", "message", "text", "content"),
				extractString(mapValue(item["result"]), "message", "text", "content"),
			)
			if update.LastError != "" {
				update.Events = append(update.Events, StreamEvent{Type: "assistant", Text: update.LastError})
			}
		}
	case "web_search", "web_search_call":
		update.Events = append(update.Events, codexToolProgress(eventType, "WebSearch", map[string]any{
			"query": firstNonEmpty(
				stringValue(item["query"]),
				extractString(mapValue(item["input"]), "query"),
			),
		})...)
	case "file_change":
		path := firstNonEmpty(
			stringValue(item["path"]),
			stringValue(item["file_path"]),
			stringValue(item["target_path"]),
		)
		if path == "" {
			if changes, ok := item["changes"].([]any); ok && len(changes) > 0 {
				change := mapValue(changes[0])
				path = firstNonEmpty(
					stringValue(change["path"]),
					stringValue(change["file_path"]),
					stringValue(change["target_path"]),
				)
			}
		}
		action := strings.ToLower(firstNonEmpty(
			stringValue(item["action"]),
			stringValue(item["change_type"]),
			stringValue(item["operation"]),
		))
		if action == "" {
			if changes, ok := item["changes"].([]any); ok && len(changes) > 0 {
				change := mapValue(changes[0])
				action = strings.ToLower(firstNonEmpty(
					stringValue(change["kind"]),
					stringValue(change["action"]),
				))
			}
		}
		toolName := "Edit"
		switch action {
		case "create", "write", "add", "new":
			toolName = "Write"
		}
		update.Events = append(update.Events, codexToolProgress(eventType, toolName, map[string]any{
			"file_path": path,
		})...)
	case "todo_list", "todo", "todowrite":
		update.Events = append(update.Events, codexToolProgress(eventType, "TodoWrite", nil)...)
	case "reasoning":
		text := extractString(item, "text", "summary", "message", "content")
		if text != "" {
			update.Events = append(update.Events, StreamEvent{Type: "assistant", Text: text})
		}
	case "agent_message", "assistant_message", "message":
		update.FullText = extractString(item, "text", "message", "content")
		if update.FullText != "" {
			update.Events = append(update.Events, StreamEvent{Type: "assistant", Text: update.FullText})
		}
	case "error":
		update.LastError = extractString(item, "message", "text", "content", "aggregated_output")
		if update.LastError != "" {
			update.Events = append(update.Events, StreamEvent{Type: "assistant", Text: update.LastError})
		}
	default:
		name := normalizeRuntimeToolName(firstNonEmpty(
			stringValue(item["name"]),
			itemType,
		))
		if name != "" {
			input := rawJSON(firstNonNil(item["input"], item["arguments"], item["parameters"]))
			if eventType == "item_started" {
				update.Events = append(update.Events, StreamEvent{Type: "tool_use_start", ToolName: name})
				if len(input) > 0 {
					update.Events = append(update.Events, StreamEvent{Type: "tool_input_delta", ToolName: name, Text: string(input)})
				}
			} else {
				update.Events = append(update.Events, StreamEvent{Type: "tool_use", ToolName: name, ToolInput: input})
			}
		} else if text := extractString(item, "text", "summary", "message", "content"); text != "" {
			update.Events = append(update.Events, StreamEvent{Type: "assistant", Text: text})
		}
	}

	return update
}

func codexToolProgress(eventType string, toolName string, input map[string]any) []StreamEvent {
	toolName = normalizeRuntimeToolName(toolName)
	if toolName == "" {
		return nil
	}
	encoded := rawJSON(input)
	if normalizeCodexEventType(eventType) == "item_started" {
		events := []StreamEvent{{Type: "tool_use_start", ToolName: toolName}}
		if len(encoded) > 0 {
			events = append(events, StreamEvent{Type: "tool_input_delta", ToolName: toolName, Text: string(encoded)})
		}
		return events
	}
	return []StreamEvent{{
		Type:      "tool_use",
		ToolName:  toolName,
		ToolInput: encoded,
	}}
}

func parseUsageMap(m map[string]any) *Usage {
	if len(m) == 0 {
		return nil
	}
	return &Usage{
		InputTokens:              intValue(firstNonNil(m["input_tokens"], m["inputTokens"])),
		OutputTokens:             intValue(firstNonNil(m["output_tokens"], m["outputTokens"])),
		CacheCreationInputTokens: intValue(firstNonNil(m["cache_creation_input_tokens"], m["cacheCreationInputTokens"])),
		CacheReadInputTokens:     intValue(firstNonNil(m["cache_read_input_tokens"], m["cacheReadInputTokens"])),
	}
}

func isFailedStatus(item map[string]any) bool {
	status := strings.ToLower(firstNonEmpty(
		stringValue(item["status"]),
		stringValue(mapValue(item["result"])["status"]),
	))
	if strings.Contains(status, "fail") || strings.Contains(status, "error") {
		return true
	}
	exitCode := intValue(firstNonNil(item["exit_code"], mapValue(item["result"])["exit_code"]))
	return exitCode != 0
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) == "" {
				continue
			}
		case json.RawMessage:
			if len(bytes.TrimSpace(typed)) == 0 {
				continue
			}
		case []byte:
			if len(bytes.TrimSpace(typed)) == 0 {
				continue
			}
		}
		return value
	}
	return nil
}

func normalizeRuntimeToolName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "reasoning", "message", "assistant_message", "agent_message", "error", "tool_use", "step_start", "step_finish":
		return ""
	case "bash", "shell", "command_execution", "command", "local_shell_call":
		return "Bash"
	case "read", "read_file", "file_read", "cat":
		return "Read"
	case "write", "write_file", "create_file":
		return "Write"
	case "edit", "replace", "patch", "apply_patch", "file_change":
		return "Edit"
	case "grep", "search", "file_search", "file_search_call":
		return "Grep"
	case "glob":
		return "Glob"
	case "websearch", "web_search", "web_search_call":
		return "WebSearch"
	case "todo", "todowrite", "todo_list":
		return "TodoWrite"
	default:
		return strings.TrimSpace(name)
	}
}

func normalizeCodexEventType(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, "/", "_")
	return name
}

func translateCodexEventMsg(event map[string]any) codexStreamUpdate {
	update := codexStreamUpdate{}
	if len(event) == 0 {
		return update
	}

	switch normalizeCodexEventType(stringValue(event["type"])) {
	case "agent_message":
		text := firstNonEmpty(
			stringValue(event["message"]),
			extractString(event, "text", "message", "content"),
		)
		if text == "" {
			return update
		}
		subtype := strings.ToLower(strings.TrimSpace(stringValue(event["phase"])))
		if subtype == "commentary" {
			update.Events = append(update.Events, StreamEvent{Type: "assistant", Subtype: subtype, Text: text})
			return update
		}
		update.FullText = text
		update.Events = append(update.Events, StreamEvent{Type: "assistant", Subtype: subtype, Text: text})
	case "task_complete", "turn_complete":
		update.TotalCostUSD = floatValue(event["total_cost_usd"])
		if usage := parseUsageMap(mapValue(event["usage"])); usage != nil {
			update.Usage = *usage
			update.HasUsage = true
		}
		if text := strings.TrimSpace(stringValue(event["last_agent_message"])); text != "" {
			update.FullText = text
		}
	case "token_count":
		info := mapValue(event["info"])
		if usage := parseUsageMap(mapValue(info["last_token_usage"])); usage != nil {
			update.Usage = *usage
			update.HasUsage = true
		}
	}

	return update
}

func translateCodexResponseItem(item map[string]any) codexStreamUpdate {
	update := codexStreamUpdate{}
	if len(item) == 0 {
		return update
	}

	switch normalizeCodexEventType(firstNonEmpty(
		stringValue(item["type"]),
		stringValue(item["kind"]),
	)) {
	case "message":
		role := strings.ToLower(strings.TrimSpace(stringValue(item["role"])))
		if role != "" && role != "assistant" && role != "agent" {
			return update
		}
		text := extractString(item, "text", "message", "content")
		if text == "" {
			return update
		}
		subtype := strings.ToLower(strings.TrimSpace(stringValue(item["phase"])))
		if subtype == "commentary" {
			update.Events = append(update.Events, StreamEvent{Type: "assistant", Subtype: subtype, Text: text})
			return update
		}
		update.FullText = text
		update.Events = append(update.Events, StreamEvent{Type: "assistant", Subtype: subtype, Text: text})
	case "reasoning":
		text := extractString(item, "text", "summary", "message", "content")
		if text != "" {
			update.Events = append(update.Events, StreamEvent{Type: "assistant", Text: text})
		}
	}

	return update
}

func buildCodexExecArgs(outputPath string, opts GenerateOpts) []string {
	args := []string{"--full-auto"}
	if workDir := strings.TrimSpace(opts.WorkDir); workDir != "" {
		args = append(args, "-C", workDir)
	}
	if effort := codexReasoningEffort(); effort != "" {
		args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%q", effort))
	}
	if strings.TrimSpace(opts.SessionID) != "" {
		args = append(args,
			"exec", "resume", opts.SessionID,
			"--json",
			"--skip-git-repo-check",
			"-o", outputPath,
		)
		return args
	}
	args = append(args,
		"exec",
		"--json",
		"--skip-git-repo-check",
		"-o", outputPath,
	)
	return args
}

type codexCatalog struct {
	Default string
	Models  []ModelOption
}

type codexConfig struct {
	codexCatalog
	ReasoningEffort string
}

func codexModelCatalog() codexCatalog {
	return codexLocalConfig().codexCatalog
}

func codexReasoningEffort() string {
	return codexLocalConfig().ReasoningEffort
}

func codexLocalConfig() codexConfig {
	configPath := codexConfigPath()
	if configPath == "" {
		return codexConfig{}
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return codexConfig{}
	}
	return parseCodexConfig(data)
}

func codexConfigPath() string {
	if home := strings.TrimSpace(os.Getenv("CODEX_HOME")); home != "" {
		return filepath.Join(expandUserPath(home), "config.toml")
	}
	userHome, err := os.UserHomeDir()
	if err != nil || userHome == "" {
		return ""
	}
	return filepath.Join(userHome, ".codex", "config.toml")
}

func parseCodexModelCatalog(data []byte) codexCatalog {
	return parseCodexConfig(data).codexCatalog
}

func parseCodexConfig(data []byte) codexConfig {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	section := ""
	defaultModel := ""
	reasoningEffort := ""
	var models []ModelOption
	addModel := func(id string, description string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		models = append(models, ModelOption{
			ID:          id,
			Description: strings.TrimSpace(description),
		})
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		key, value, ok := parseSimpleTOMLAssignment(line)
		if !ok {
			continue
		}
		switch {
		case key == "model":
			if defaultModel == "" {
				defaultModel = value
			}
			addModel(value, "Discovered from Codex config")
		case key == "model_reasoning_effort" && section == "":
			reasoningEffort = normalizeCodexReasoningEffort(value)
		case section == "notice.model_migrations":
			addModel(key, "Migrated Codex model")
			addModel(value, "Migrated Codex model")
		}
	}

	merged := MergeModelOptions(models)
	defaultModel = strings.TrimSpace(defaultModel)
	if defaultModel == "" && len(merged) > 0 {
		defaultModel = merged[0].ID
	}
	return codexConfig{
		codexCatalog: codexCatalog{
			Default: defaultModel,
			Models:  merged,
		},
		ReasoningEffort: reasoningEffort,
	}
}

func normalizeCodexReasoningEffort(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low", "medium", "high":
		return strings.ToLower(strings.TrimSpace(value))
	case "xhigh":
		return "high"
	case "xlow":
		return "low"
	default:
		return ""
	}
}

func parseSimpleTOMLAssignment(line string) (string, string, bool) {
	idx := strings.Index(line, "=")
	if idx < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	if key == "" || value == "" {
		return "", "", false
	}

	key = trimTOMLStringToken(key)
	if key == "" {
		return "", "", false
	}
	value = parseTOMLValueToken(value)
	if value == "" {
		return "", "", false
	}
	return key, value, true
}

func trimTOMLStringToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if unquoted, err := strconv.Unquote(token); err == nil {
		return strings.TrimSpace(unquoted)
	}
	return strings.Trim(token, `"'`)
}

func parseTOMLValueToken(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if raw[0] == '"' || raw[0] == '\'' {
		quote := raw[0]
		escaped := false
		for i := 1; i < len(raw); i++ {
			ch := raw[i]
			if quote == '"' && ch == '\\' && !escaped {
				escaped = true
				continue
			}
			if ch == quote && !escaped {
				if unquoted, err := strconv.Unquote(raw[:i+1]); err == nil {
					return strings.TrimSpace(unquoted)
				}
				return strings.Trim(raw[:i+1], `"'`)
			}
			escaped = false
		}
		return strings.Trim(raw, `"'`)
	}
	if idx := strings.Index(raw, "#"); idx >= 0 {
		raw = raw[:idx]
	}
	return strings.TrimSpace(raw)
}
