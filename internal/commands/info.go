package commands

import (
	"github.com/moasq/nanowave/internal/service"
	"github.com/moasq/nanowave/internal/terminal"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show project status",
	Long:  "Display information about the current project.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfigWithProject()
		if err != nil {
			terminal.Info("No projects yet. Run `nanowave` to create one.")
			return nil
		}

		svc, err := service.NewService(cfg)
		if err != nil {
			return err
		}

		return svc.Info()
	},
}
