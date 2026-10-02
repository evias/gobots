package driver

import (
	"log/slog"

	"github.com/spf13/cobra"
)

func NewCmdDriverTest() *cobra.Command {
	testCmd := &cobra.Command{
		Use:     "test [options]",
		Short:   "Test your edge device drivers (botfiles).",
		Args:    cobra.NoArgs,
		Aliases: []string{"diagnostic"},
		RunE: func(cmd *cobra.Command, args []string) error {
			initLogs()
			slog.Debug("running driver test", "driverFile", driverFile)

			return nil
		},
	}

	testCmd.Flags().StringVarP(&driverFile, "driver", "d", defaultDriver,
		"The gobots driver file for your robot (optional).")
	testCmd.Flags().BoolVarP(&enableDebug, "debug", "D", false,
		"Sets whether to enable debug mode/logs or not (optional).")

	return testCmd
}
