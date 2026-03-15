// Package nwtool exposes nanowave capabilities as tools that any LLM runtime
// can invoke — either via MCP (Claude Code) or via CLI (Codex/OpenCode).
// Tools wrap existing orchestration functions; no domain logic lives here.
package nwtool

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Tool describes a single nanowave capability that an LLM can invoke.
type Tool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
	Handler     func(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
}

// Registry holds all registered tools, keyed by name.
type Registry struct {
	tools map[string]*Tool
	order []string
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]*Tool)}
}

// Register adds a tool. Panics on duplicate names.
func (r *Registry) Register(t *Tool) {
	if _, exists := r.tools[t.Name]; exists {
		panic(fmt.Sprintf("nwtool: duplicate tool name %q", t.Name))
	}
	r.tools[t.Name] = t
	r.order = append(r.order, t.Name)
}

// Get returns the tool with the given name, or nil.
func (r *Registry) Get(name string) *Tool {
	return r.tools[name]
}

// All returns all tools in registration order.
func (r *Registry) All() []*Tool {
	tools := make([]*Tool, 0, len(r.order))
	for _, name := range r.order {
		tools = append(tools, r.tools[name])
	}
	return tools
}

// Names returns all tool names in sorted order.
func (r *Registry) Names() []string {
	names := make([]string, len(r.order))
	copy(names, r.order)
	sort.Strings(names)
	return names
}

// ToolDescriptionsMarkdown returns a markdown description of all tools
// suitable for injecting into system prompts for non-MCP runtimes
// (Codex, OpenCode) that invoke tools via `nanowave tool <name>`.
func (r *Registry) ToolDescriptionsMarkdown() string {
	var b strings.Builder
	b.WriteString("\n\n## Nanowave Tools\n\n")
	b.WriteString("These tools are available via CLI. Invoke them by piping JSON to stdin:\n")
	b.WriteString("```\necho '{\"key\":\"value\"}' | nanowave tool <tool_name>\n```\n\n")

	for _, t := range r.All() {
		b.WriteString(fmt.Sprintf("### %s\n\n", t.Name))
		b.WriteString(t.Description)
		b.WriteString("\n\n")
		if len(t.InputSchema) > 0 {
			b.WriteString("Input schema:\n```json\n")
			b.Write(t.InputSchema)
			b.WriteString("\n```\n\n")
		}
	}
	return b.String()
}

// jsonOK marshals a success result.
func jsonOK(v any) (json.RawMessage, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return data, nil
}

// jsonError returns a JSON error result.
func jsonError(msg string) (json.RawMessage, error) {
	result := map[string]any{"success": false, "error": msg}
	data, _ := json.Marshal(result)
	return data, nil
}
