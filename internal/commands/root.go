package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time.
var Version = "0.35.2"

var rootCmd = &cobra.Command{
	Use:     "nanowave",
	Short:   "Autonomous Apple platform app builder",
	Long:    "Nanowave builds, edits, and fixes Apple platform apps using a selectable AI agent runtime.",
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
	rootCmd.PersistentFlags().StringVar(&agentFlag, "agent", "", "AI runtime to use (claude, codex, opencode)")
	rootCmd.PersistentFlags().StringVar(&modelFlag, "model", "", "Model to use for code generation inside the selected runtime")
	rootCmd.PersistentFlags().BoolVar(&agenticFlag, "agentic", false, "Use agentic mode: LLM drives the build via tool calling")
	rootCmd.PersistentFlags().BoolVar(&runtimeLogsFlag, "runtime-logs", false, "Show raw AI runtime logs while the agent is running")

	rootCmd.AddCommand(fixCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(chatCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(usageCmd)
	rootCmd.AddCommand(integrationsCmd)
	rootCmd.AddCommand(publishCmd)
	rootCmd.AddCommand(toolCmd)
}

// modelFlag holds the --model flag value.
var modelFlag string

// agentFlag holds the --agent flag value.
var agentFlag string

// agenticFlag holds the --agentic flag value.
var agenticFlag bool

// runtimeLogsFlag holds the --runtime-logs flag value.
var runtimeLogsFlag bool

// ModelFlag returns the current --model flag value.
func ModelFlag() string {
	return modelFlag
}

// AgentFlag returns the current --agent flag value.
func AgentFlag() string {
	return agentFlag
}

// AgenticFlag returns the current --agentic flag value.
func AgenticFlag() bool {
	return agenticFlag
}

// RuntimeLogsFlag returns the current --runtime-logs flag value.
func RuntimeLogsFlag() bool {
	return runtimeLogsFlag
}

func applyRuntimeLogsFlag() {
	if runtimeLogsFlag {
		_ = os.Setenv("NANOWAVE_RUNTIME_LOGS", "1")
	}
}
