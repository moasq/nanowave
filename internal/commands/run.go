package commands

import (
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Build and launch the app",
	Long:  "Build the project and launch it. For multi-platform projects, prompts to select which platform to run.",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := loadProjectService()
		if err != nil {
			printNoProjectFoundCreateFirst()
			return err
		}
		return svc.Run(cmd.Context())
	},
}
