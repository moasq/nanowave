package nwtool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// RunCLI executes a single tool by name, reading JSON input from r
// and writing the JSON result to stdout.
func RunCLI(ctx context.Context, toolName string, r io.Reader) error {
	reg := NewDefaultRegistry()
	tool := reg.Get(toolName)
	if tool == nil {
		return fmt.Errorf("unknown tool: %s\nUse 'nanowave tool --list' to see available tools", toolName)
	}

	input, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	if len(input) == 0 {
		input = []byte("{}")
	}

	result, err := tool.Handler(ctx, json.RawMessage(input))
	if err != nil {
		return fmt.Errorf("tool %s failed: %w", toolName, err)
	}

	var pretty json.RawMessage
	if err := json.Unmarshal(result, &pretty); err == nil {
		if formatted, err := json.MarshalIndent(pretty, "", "  "); err == nil {
			result = formatted
		}
	}

	_, err = os.Stdout.Write(result)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout)
	return nil
}

// ListToolsJSON prints all tool schemas as a JSON array to stdout.
func ListToolsJSON() error {
	reg := NewDefaultRegistry()
	type toolInfo struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"input_schema,omitempty"`
	}
	var tools []toolInfo
	for _, t := range reg.All() {
		tools = append(tools, toolInfo{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema})
	}
	data, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout)
	return nil
}

// ListToolsMarkdown prints tool descriptions as markdown to stdout.
func ListToolsMarkdown() error {
	reg := NewDefaultRegistry()
	_, err := fmt.Fprint(os.Stdout, reg.ToolDescriptionsMarkdown())
	return err
}
