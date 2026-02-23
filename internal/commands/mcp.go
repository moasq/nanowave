package commands

import (
	"github.com/moasq/nanowave/internal/xcodegenserver"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:    "mcp",
	Short:  "Run MCP servers (used internally by Claude Code)",
	Hidden: true,
}

var mcpXcodegenCmd = &cobra.Command{
	Use:   "xcodegen",
	Short: "Run the XcodeGen MCP server",
	Long:  "Starts the XcodeGen MCP server over stdio. Used by Claude Code to manage Xcode project configuration via typed tool calls.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return xcodegenserver.Run(cmd.Context())
	},
}

func init() {
	mcpCmd.AddCommand(mcpXcodegenCmd)
}
