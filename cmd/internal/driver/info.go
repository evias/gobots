package driver

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/evias/gobots/botfile"
	tui "github.com/evias/gobots/cmd/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	driverFiles []string
)

func NewCmdDriverInfo() *cobra.Command {
	infoCmd := &cobra.Command{
		Use:     "info [options]",
		Short:   "Get information for edge device drivers (botfiles).",
		Args:    cobra.NoArgs,
		Aliases: []string{"view", "read", "get"},
		// This PreRunE ensures that the console is in raw mode. This is important
		// for the `cmd.internal.tui` package so that tables render correctly.
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Set console in raw mode for printing.
			var err error
			consoleDesc = int(os.Stdin.Fd())
			if consoleState, err = term.MakeRaw(consoleDesc); err != nil {
				return fmt.Errorf("terminal state raw mode failure: %w", err)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			slog.SetDefault(&slog.Logger{})
			// Restore console from raw mode after end.
			defer term.Restore(consoleDesc, consoleState)
			// Restore arguments, to permit multiple calls from interactive console.
			defer func() { driverFiles = []string{} }()

			if len(driverFiles) == 0 {
				return errors.New("at least one driver file must be provided (--driver).")
			}

			// Interprets/Accepts multiple --driver options.
			driverFiles = sliceUnique(driverFiles)

			// Load each of the passed driver files into [botfile.Driver] instances.
			for i, driverFile := range driverFiles {
				var (
					driver botfile.Driver
					err    error
				)
				if driver, err = loadDriver(driverFile); err != nil {
					return err
				}

				tui.PrintDriver(driverFile, driver)
				if i < len(driverFiles)-1 {
					fmt.Print("\r\n")
				}
			}

			return nil
		},
	}

	infoCmd.Flags().StringArrayVarP(&driverFiles, "driver", "d", nil,
		"The gobots driver file(s) for your edge devices.")

	return infoCmd
}

// -----------------------------------------------------------------------------
// Driver — determines the connection, commands and fields for a device.
// -----------------------------------------------------------------------------

// loadDriver returns a [botfile.Driver] after reading driverFile.
func loadDriver(driverFile string) (botfile.Driver, error) {
	if _, err := os.Stat(driverFile); err != nil && os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to open driver file: %w", err)
	}

	driver, err := botfile.LoadFromConfig(driverFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load driver file: %w", err)
	}

	return driver, nil
}
