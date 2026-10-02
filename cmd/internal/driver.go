package internal

import (
	"github.com/spf13/cobra"

	apidrv "github.com/evias/gobots/cmd/internal/driver"
)

func NewCmdDriver() *cobra.Command {
	driverCmd := &cobra.Command{
		Use:   "driver <subcommand> [options]",
		Short: "List/Read/Write edge device drivers with botfiles.",
	}

	driverCmd.AddCommand(
		apidrv.NewCmdDriverList(),
		apidrv.NewCmdDriverInfo(),
		apidrv.NewCmdDriverTest(),
	)

	return driverCmd
}
