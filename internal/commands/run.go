package commands

import (
	"github.com/moasq/nanowave/internal/service"
	"github.com/moasq/nanowave/internal/terminal"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Build and launch in the iOS Simulator",
	Long:  "Build the project for the iOS Simulator and launch it.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfigWithProject()
		if err != nil {
			terminal.Error("No project found.")
			terminal.Info("Run `nanowave` first to create a project.")
			return err
		}

		svc, err := service.NewService(cfg)
		if err != nil {
			return err
		}

		return svc.Run(cmd.Context())
	},
}
