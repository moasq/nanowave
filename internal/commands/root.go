package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time.
var Version = "0.1.12"

var rootCmd = &cobra.Command{
	Use:     "nanowave",
	Short:   "Autonomous Apple platform app builder",
	Long:    "Nanowave builds, edits, and fixes Apple platform apps using Claude Code as the AI backend.",
	Version: Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInteractive(cmd)
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open the project in Xcode",
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := loadProjectService()
		if err != nil {
			return fmt.Errorf("no project found. Run `nanowave` first")
		}
		return svc.Open()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&modelFlag, "model", "", "Claude model to use for code generation (sonnet, opus, haiku)")

	rootCmd.AddCommand(fixCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(chatCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(usageCmd)
}

// modelFlag holds the --model flag value.
var modelFlag string

// ModelFlag returns the current --model flag value.
func ModelFlag() string {
	return modelFlag
}
