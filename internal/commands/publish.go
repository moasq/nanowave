package commands

import (
	"fmt"
	"os/signal"
	"syscall"

	"github.com/moasq/nanowave/internal/service"
	"github.com/moasq/nanowave/internal/terminal"
	"github.com/spf13/cobra"
)

var publishCmd = &cobra.Command{
	Use:   "publish [target]",
	Short: "Publish app to TestFlight or App Store",
	Long:  "Runs the full App Store Connect publishing flow in the terminal. Default target is testflight.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := "testflight"
		if len(args) > 0 {
			target = args[0]
		}

		svc, err := loadProjectService(service.ServiceOpts{Model: ModelFlag()})
		if err != nil {
			printNoProjectFoundCreateFirst()
			return err
		}

		var prompt string
		switch target {
		case "testflight":
			prompt = "Submit this app to TestFlight for beta testing."
		case "appstore":
			prompt = "Submit this app to the App Store for review."
		default:
			prompt = fmt.Sprintf("Publish this app: %s", target)
		}

		ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		if err := svc.ASC(ctx, prompt); err != nil {
			terminal.Error(fmt.Sprintf("Publish failed: %v", err))
			return err
		}
		return nil
	},
}
