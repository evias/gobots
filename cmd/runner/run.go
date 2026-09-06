package runner

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
)

var (
	flow string
)

func NewCmdRun() *cobra.Command {
	seedCmd := &cobra.Command{
		Use:   "run <flow> [options]",
		Short: "Run gobots flows with your botfiles.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			flow = args[0]
			slog.Info(fmt.Sprintf("Running the flow: %s", flow))

			return nil
		},
	}

	seedCmd.Flags().StringVarP(&flow, "flow", "f", "", "The path to a .yaml botfile.")
	return seedCmd
}
