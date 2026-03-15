package commands

import (
	"fmt"
	"os"

	"github.com/moasq/nanowave/internal/nwtool"
	"github.com/spf13/cobra"
)

var toolListFlag bool
var toolFormatFlag string

var toolCmd = &cobra.Command{
	Use:   "tool [name]",
	Short: "Run a nanowave tool directly (reads JSON from stdin, writes JSON to stdout)",
	Long: `Execute a nanowave tool by name. Input is read as JSON from stdin.

Examples:
  echo '{"project_dir":"/tmp/test","app_name":"MyApp"}' | nanowave tool nw_setup_workspace
  nanowave tool --list
  nanowave tool --list --format md`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if toolListFlag {
			if toolFormatFlag == "md" {
				return nwtool.ListToolsMarkdown()
			}
			return nwtool.ListToolsJSON()
		}
		if len(args) == 0 {
			return fmt.Errorf("tool name required. Use --list to see available tools")
		}
		return nwtool.RunCLI(cmd.Context(), args[0], os.Stdin)
	},
}

func init() {
	toolCmd.Flags().BoolVar(&toolListFlag, "list", false, "List all available tools")
	toolCmd.Flags().StringVar(&toolFormatFlag, "format", "json", "Output format for --list: json or md")
}
