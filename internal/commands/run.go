package commands

import (
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Build and launch in the iOS Simulator",
	Long:  "Build the project for the iOS Simulator and launch it.",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := loadProjectService()
		if err != nil {
			printNoProjectFoundCreateFirst()
			return err
		}
		return svc.Run(cmd.Context())
	},
}
