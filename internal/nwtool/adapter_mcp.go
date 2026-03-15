package nwtool

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpTextOutput is the standard text output type for MCP tool results.
type mcpTextOutput struct {
	Message string `json:"message"`
}

// mcpGenericInput accepts any JSON input — the handler re-marshals from req.Params.Arguments.
type mcpGenericInput struct{}

// RunMCPServer starts an MCP server exposing all nanowave tools over stdio.
func RunMCPServer(ctx context.Context) error {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "nanowave-tools", Version: "v1.0.0"},
		nil,
	)

	reg := NewDefaultRegistry()
	for _, t := range reg.All() {
		handler := t.Handler
		mcp.AddTool(server, &mcp.Tool{
			Name:        t.Name,
			Description: t.Description,
		}, func(ctx context.Context, req *mcp.CallToolRequest, _ mcpGenericInput) (*mcp.CallToolResult, mcpTextOutput, error) {
			inputJSON, err := json.Marshal(req.Params.Arguments)
			if err != nil {
				return &mcp.CallToolResult{IsError: true}, mcpTextOutput{Message: "invalid input: " + err.Error()}, nil
			}
			result, err := handler(ctx, inputJSON)
			if err != nil {
				return &mcp.CallToolResult{IsError: true}, mcpTextOutput{Message: "tool error: " + err.Error()}, nil
			}
			return nil, mcpTextOutput{Message: string(result)}, nil
		})
	}

	return server.Run(ctx, &mcp.StdioTransport{})
}
