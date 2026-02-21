package commands

import (
	"fmt"
	"strings"

	"github.com/moasq/nanowave/internal/storage"
	"github.com/moasq/nanowave/internal/terminal"
	"github.com/spf13/cobra"
)

var usageDays int

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show usage and cost history",
	Long:  "Display daily usage statistics including cost, tokens, and request counts.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfigWithProject()
		if err != nil {
			terminal.Info("No projects yet. Run `nanowave` to create one.")
			return nil
		}

		usageStore := storage.NewUsageStore(cfg.NanowaveDir)

		// Current session
		current := usageStore.Current()
		terminal.Header("Usage")
		terminal.Detail("Session cost", fmt.Sprintf("$%.4f", current.TotalCostUSD))
		terminal.Detail("Session tokens", fmt.Sprintf("%s in / %s out",
			storage.FormatTokenCount(current.InputTokens),
			storage.FormatTokenCount(current.OutputTokens)))
		terminal.Detail("Session requests", fmt.Sprintf("%d", current.Requests))

		// Daily history
		history := usageStore.History(usageDays)
		if len(history) == 0 {
			fmt.Println()
			terminal.Info("No daily usage history yet.")
			return nil
		}

		fmt.Println()
		terminal.Header("Daily History")

		// Table header
		fmt.Printf("  %-12s %10s %10s %10s %8s\n", "Date", "Cost", "Input", "Output", "Reqs")
		fmt.Printf("  %s\n", strings.Repeat("-", 54))

		var totalCost float64
		var totalInput, totalOutput, totalReqs int
		for _, day := range history {
			fmt.Printf("  %-12s %10s %10s %10s %8d\n",
				day.Date,
				fmt.Sprintf("$%.4f", day.TotalCostUSD),
				storage.FormatTokenCount(day.InputTokens),
				storage.FormatTokenCount(day.OutputTokens),
				day.Requests,
			)
			totalCost += day.TotalCostUSD
			totalInput += day.InputTokens
			totalOutput += day.OutputTokens
			totalReqs += day.Requests
		}

		fmt.Printf("  %s\n", strings.Repeat("-", 54))
		fmt.Printf("  %-12s %10s %10s %10s %8d\n",
			"Total",
			fmt.Sprintf("$%.4f", totalCost),
			storage.FormatTokenCount(totalInput),
			storage.FormatTokenCount(totalOutput),
			totalReqs,
		)

		return nil
	},
}

func init() {
	usageCmd.Flags().IntVar(&usageDays, "days", 7, "Number of days of history to show")
}
