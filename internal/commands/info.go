package commands

import (
	"github.com/moasq/nanowave/internal/terminal"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show project status",
	Long:  "Display information about the current project.",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := loadProjectService()
		if err != nil {
			terminal.Info("No projects yet. Run `nanowave` to create one.")
			return nil
		}
		return svc.Info()
	},
}
