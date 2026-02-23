package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Client wraps the Claude Code CLI for LLM calls.
type Client struct {
	claudePath string
	model      string // default model override (empty = let claude decide)
}

// NewClient creates a new Claude Code client.
func NewClient(claudePath string) *Client {
	return &Client{
		claudePath: claudePath,
	}
}

// WithModel returns a copy of the client with a specific model.
func (c *Client) WithModel(model string) *Client {
	return &Client{
		claudePath: c.claudePath,
		model:      model,
	}
}

// GenerateOpts holds options for a Generate call.
type GenerateOpts struct {
	SystemPrompt       string
	AppendSystemPrompt string   // --append-system-prompt (added to auto-discovered CLAUDE.md)
	JSONSchema         string   // JSON schema for structured output
	MaxTurns           int      // Max agentic turns (default 1)
	AllowedTools       []string // MCP tools to allow
	MCPConfig          string   // Path to MCP config file
	Model              string   // Model override for this call
	WorkDir            string   // Working directory for the claude process
	SessionID          string   // Resume a previous session
	Images             []string // Absolute paths to image files to include in the prompt
}

// Usage holds token usage data from a Claude response.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// Response represents the parsed response from Claude Code.
type Response struct {
	Result       string          `json:"result"`
	RawJSON      json.RawMessage `json:"-"`
	TotalCostUSD float64         `json:"total_cost_usd"`
	SessionID    string          `json:"session_id"`
	NumTurns     int             `json:"num_turns"`
	Usage        Usage           `json:"usage"`
}

// buildImageContext appends image file references to the user message.
func buildImageContext(userMessage string, images []string) string {
	if len(images) == 0 {
		return userMessage
	}
	var sb strings.Builder
	sb.WriteString(userMessage)
	sb.WriteString("\n\n[Attached images — read each file to view the image:]\n")
	for i, img := range images {
		sb.WriteString(fmt.Sprintf("- Image %d: %s\n", i+1, img))
	}
	return sb.String()
}

// streamNDJSONLines reads newline-delimited JSON from r and calls onLine for each line.
// It does not impose bufio.Scanner token limits and also processes a final line without
// a trailing newline.
func streamNDJSONLines(r io.Reader, onLine func([]byte) error) error {
	br := bufio.NewReader(r)

	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			line = bytes.TrimRight(line, "\r\n")
			if len(bytes.TrimSpace(line)) > 0 {
				if onErr := onLine(line); onErr != nil {
					return onErr
				}
			}
		}

		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// Generate sends a prompt to Claude Code and returns the response.
func (c *Client) Generate(ctx context.Context, userMessage string, opts GenerateOpts) (*Response, error) {
	userMessage = buildImageContext(userMessage, opts.Images)
	args := []string{"-p"}

	maxTurns := opts.MaxTurns
	if maxTurns == 0 {
		maxTurns = 1
	}
	args = append(args, "--max-turns", fmt.Sprintf("%d", maxTurns))

	if opts.SystemPrompt != "" {
		args = append(args, "--system-prompt", opts.SystemPrompt)
	}

	if opts.AppendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", opts.AppendSystemPrompt)
	}

	if opts.SessionID != "" {
		args = append(args, "--resume", opts.SessionID)
	}

	if opts.JSONSchema != "" {
		args = append(args, "--output-format", "json", "--json")
	} else {
		args = append(args, "--output-format", "json")
	}

	// Model selection: per-call override > client default
	model := opts.Model
	if model == "" {
		model = c.model
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	if opts.MCPConfig != "" {
		args = append(args, "--mcp-config", opts.MCPConfig)
	}

	for _, tool := range opts.AllowedTools {
		args = append(args, "--allowedTools", tool)
	}

	cmd := exec.CommandContext(ctx, c.claudePath, args...)
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}

	// Strip CLAUDECODE env var to allow nested sessions
	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	// Pass user message via stdin to avoid argument length limits
	cmd.Stdin = strings.NewReader(userMessage)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude command failed: %w\nstderr: %s", err, stderr.String())
	}

	return parseResponse(stdout.Bytes())
}

// StreamEvent represents a parsed event from Claude Code's stream-json output.
type StreamEvent struct {
	Type    string // "assistant", "tool_use", "tool_result", "result", "system", "content_block_delta"
	Subtype string // e.g. "init"

	// For tool_use events
	ToolName  string
	ToolInput json.RawMessage

	// For assistant text events and content_block_delta events
	Text string

	// For result events
	Result    string
	SessionID string
	CostUSD   float64
	NumTurns  int
	IsError   bool
	Usage     Usage
}

// GenerateStreaming sends a prompt and streams events via callback.
// The callback is called for each meaningful event (tool calls, results).
// Returns the final Response when complete.
func (c *Client) GenerateStreaming(ctx context.Context, userMessage string, opts GenerateOpts, onEvent func(StreamEvent)) (*Response, error) {
	userMessage = buildImageContext(userMessage, opts.Images)
	args := []string{"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages"}

	maxTurns := opts.MaxTurns
	if maxTurns == 0 {
		maxTurns = 1
	}
	args = append(args, "--max-turns", fmt.Sprintf("%d", maxTurns))

	if opts.SystemPrompt != "" {
		args = append(args, "--system-prompt", opts.SystemPrompt)
	}

	if opts.AppendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", opts.AppendSystemPrompt)
	}

	if opts.SessionID != "" {
		args = append(args, "--resume", opts.SessionID)
	}

	model := opts.Model
	if model == "" {
		model = c.model
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	if opts.MCPConfig != "" {
		args = append(args, "--mcp-config", opts.MCPConfig)
	}

	for _, tool := range opts.AllowedTools {
		args = append(args, "--allowedTools", tool)
	}

	cmd := exec.CommandContext(ctx, c.claudePath, args...)
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}

	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")
	cmd.Stdin = strings.NewReader(userMessage)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start claude: %w", err)
	}

	var lastResponse *Response
	var sessionID string
	var assistantText strings.Builder // accumulate text from assistant events

	streamErr := streamNDJSONLines(stdout, func(line []byte) error {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			return nil
		}
		if !json.Valid(trimmed) {
			return fmt.Errorf("invalid JSON stream event (%d bytes)", len(trimmed))
		}

		ev := parseStreamEvent(trimmed)
		if ev == nil {
			return nil
		}

		if ev.Type == "system" && ev.Subtype == "init" && ev.SessionID != "" {
			sessionID = ev.SessionID
		}

		// Accumulate assistant text from deltas and full messages
		if ev.Type == "content_block_delta" && ev.Text != "" {
			assistantText.WriteString(ev.Text)
		} else if ev.Type == "assistant" && ev.Text != "" {
			// Full message arrived — use it as authoritative text (replaces deltas)
			assistantText.Reset()
			assistantText.WriteString(ev.Text)
		}

		if ev.Type == "result" {
			result := ev.Result
			if result == "" {
				result = assistantText.String()
			}
			lastResponse = &Response{
				Result:       result,
				TotalCostUSD: ev.CostUSD,
				SessionID:    ev.SessionID,
				NumTurns:     ev.NumTurns,
				Usage:        ev.Usage,
			}
			if ev.IsError {
				return fmt.Errorf("claude returned error: %s", ev.Result)
			}
		}

		if onEvent != nil {
			onEvent(*ev)
		}
		return nil
	})
	if streamErr != nil {
		_ = cmd.Wait()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("failed to read claude stream: %w", streamErr)
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if stderrMsg := stderrBuf.String(); stderrMsg != "" {
			return nil, fmt.Errorf("claude command failed: %w\nstderr: %s", err, stderrMsg)
		}
		return nil, fmt.Errorf("claude command failed: %w", err)
	}

	if lastResponse != nil {
		if lastResponse.SessionID == "" {
			lastResponse.SessionID = sessionID
		}
		return lastResponse, nil
	}

	// No result event — still return accumulated text if any
	result := assistantText.String()
	return &Response{Result: result, SessionID: sessionID}, nil
}

// parseStreamEvent parses a single NDJSON line from stream-json output.
func parseStreamEvent(line []byte) *StreamEvent {
	// Claude Code stream-json emits various event shapes.
	// We parse what we need and ignore the rest.
	var raw struct {
		Type      string  `json:"type"`
		Subtype   string  `json:"subtype"`
		SessionID string  `json:"session_id"`
		Result    string  `json:"result"`
		CostUSD   float64 `json:"cost_usd"`
		NumTurns  int     `json:"num_turns"`
		IsError   bool    `json:"is_error"`
		Usage     Usage   `json:"usage"`

		// For assistant messages (type: "assistant", message.content[])
		Message struct {
			Content []struct {
				Type  string          `json:"type"`
				Name  string          `json:"name"`
				Text  string          `json:"text"`
				Input json.RawMessage `json:"input"`
			} `json:"content"`
		} `json:"message"`

		// For stream_event (type: "stream_event", --include-partial-messages)
		Event struct {
			Type  string `json:"type"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		} `json:"event"`
	}

	if err := json.Unmarshal(line, &raw); err != nil {
		return nil
	}

	ev := &StreamEvent{
		Type:      raw.Type,
		Subtype:   raw.Subtype,
		SessionID: raw.SessionID,
		Result:    raw.Result,
		CostUSD:   raw.CostUSD,
		NumTurns:  raw.NumTurns,
		IsError:   raw.IsError,
		Usage:     raw.Usage,
	}

	// Handle stream_event with content_block_delta (token-by-token text)
	if raw.Type == "stream_event" && raw.Event.Type == "content_block_delta" && raw.Event.Delta.Type == "text_delta" {
		ev.Type = "content_block_delta"
		ev.Text = raw.Event.Delta.Text
		return ev
	}

	// Extract tool_use or text from assistant messages
	if raw.Type == "assistant" {
		for _, c := range raw.Message.Content {
			if c.Type == "tool_use" && c.Name != "" {
				ev.Type = "tool_use"
				ev.ToolName = c.Name
				ev.ToolInput = c.Input
				return ev
			}
		}
		for _, c := range raw.Message.Content {
			if c.Type == "text" && c.Text != "" {
				ev.Text = c.Text
				return ev
			}
		}
	}

	// Only return events we care about
	switch raw.Type {
	case "result", "system":
		return ev
	case "tool_use":
		return ev
	case "assistant":
		if ev.Text != "" {
			return ev
		}
	}

	return nil
}

// filterEnv returns env with the named variable removed.
func filterEnv(env []string, name string) []string {
	prefix := name + "="
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// parseResponse extracts the result from Claude Code's JSON output.
// Claude Code can output either:
// 1. A single JSON object: {"result": "...", "cost_usd": ..., ...}
// 2. A JSON array of events: [{"type":"system",...}, ..., {"type":"result","result":"..."}]
// 3. JSONL (newline-delimited): {"type":"system"...}\n{"type":"result"...}\n
func parseResponse(data []byte) (*Response, error) {
	trimmed := bytes.TrimSpace(data)

	// Try single object first (most common without --verbose)
	var single struct {
		Result       string  `json:"result"`
		TotalCostUSD float64 `json:"cost_usd"`
		SessionID    string  `json:"session_id"`
		NumTurns     int     `json:"num_turns"`
		IsError      bool    `json:"is_error"`
		Usage        Usage   `json:"usage"`
	}
	if err := json.Unmarshal(trimmed, &single); err == nil && single.Result != "" {
		if single.IsError {
			return nil, fmt.Errorf("claude returned error: %s", single.Result)
		}
		return &Response{
			Result:       single.Result,
			RawJSON:      data,
			TotalCostUSD: single.TotalCostUSD,
			SessionID:    single.SessionID,
			NumTurns:     single.NumTurns,
			Usage:        single.Usage,
		}, nil
	}

	// Try JSON array of events (verbose mode)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var events []json.RawMessage
		if err := json.Unmarshal(trimmed, &events); err == nil {
			return extractResultFromEvents(events, data)
		}
	}

	// Try JSONL (newline-delimited JSON objects)
	lines := bytes.Split(trimmed, []byte("\n"))
	if len(lines) > 1 {
		var events []json.RawMessage
		for _, line := range lines {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			events = append(events, json.RawMessage(line))
		}
		if len(events) > 0 {
			return extractResultFromEvents(events, data)
		}
	}

	// Fallback: treat as plain text
	return &Response{
		Result:  strings.TrimSpace(string(data)),
		RawJSON: data,
	}, nil
}

// extractResultFromEvents finds the result event in a stream of Claude Code events.
func extractResultFromEvents(events []json.RawMessage, rawData []byte) (*Response, error) {
	type eventBase struct {
		Type      string  `json:"type"`
		Subtype   string  `json:"subtype"`
		SessionID string  `json:"session_id"`
		Result    string  `json:"result"`
		CostUSD   float64 `json:"cost_usd"`
		NumTurns  int     `json:"num_turns"`
		IsError   bool    `json:"is_error"`
		Usage     Usage   `json:"usage"`
	}

	var sessionID string
	var lastResult *Response

	for _, raw := range events {
		var ev eventBase
		if err := json.Unmarshal(raw, &ev); err != nil {
			continue
		}

		// Capture session ID from init event
		if ev.Type == "system" && ev.Subtype == "init" && ev.SessionID != "" {
			sessionID = ev.SessionID
		}

		// Look for result event
		if ev.Type == "result" || ev.Result != "" {
			lastResult = &Response{
				Result:       ev.Result,
				RawJSON:      rawData,
				TotalCostUSD: ev.CostUSD,
				SessionID:    ev.SessionID,
				NumTurns:     ev.NumTurns,
				Usage:        ev.Usage,
			}
			if ev.IsError {
				return nil, fmt.Errorf("claude returned error: %s", ev.Result)
			}
		}
	}

	if lastResult != nil {
		if lastResult.SessionID == "" {
			lastResult.SessionID = sessionID
		}
		return lastResult, nil
	}

	// No result event found, return session ID at least
	return &Response{
		Result:    "",
		RawJSON:   rawData,
		SessionID: sessionID,
	}, nil
}
