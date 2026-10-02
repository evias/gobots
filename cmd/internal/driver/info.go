package driver

import (
	"log/slog"

	"github.com/spf13/cobra"
)

func NewCmdDriverInfo() *cobra.Command {
	infoCmd := &cobra.Command{
		Use:     "info [options]",
		Short:   "Get information for edge device drivers (botfiles).",
		Args:    cobra.NoArgs,
		Aliases: []string{"view", "read", "get"},
		RunE: func(cmd *cobra.Command, args []string) error {
			initLogs()
			slog.Debug("running driver info", "driverFile", driverFile)

			return nil
		},
	}

	infoCmd.Flags().StringVarP(&driverFile, "driver", "d", defaultDriver,
		"The gobots driver file for your robot (optional).")
	infoCmd.Flags().BoolVarP(&enableDebug, "debug", "D", false,
		"Sets whether to enable debug mode/logs or not (optional).")

	return infoCmd
}
