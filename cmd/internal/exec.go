package internal

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/evias/gobots/botfile"
)

var (
	command string
)

func NewCmdExec() *cobra.Command {
	execCmd := &cobra.Command{
		Use:   "exec <command> [options]",
		Short: "Send comprehensive commands to your gobots.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			command = strings.ToLower(args[0])

			// --debug enables debug messages in default logger.
			logLevel := new(slog.LevelVar)
			if enableDebug {
				logLevel.Set(slog.LevelDebug)
				handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
					Level: logLevel,
				})
				slog.SetDefault(slog.New(handler))
			}

			// Load the device driver configuration file (YAML) into a [botfile.Driver].
			var (
				driver botfile.Driver
				err    error
			)
			if driver, err = loadDriver(hostOrDriver, driverFile); err != nil {
				return err
			}

			if command == "help" || !driver.HasCommand(command) {
				cmd.Usage()
				return nil
			}

			slog.Debug(fmt.Sprintf("Driver: %s", driverFile))
			slog.Debug(fmt.Sprintf("Host: %s", driver.Host()))
			slog.Debug(fmt.Sprintf("Port: %d", driver.Port()))
			slog.Debug(fmt.Sprintf("Command: %s", command))

			// e.g. everything after "move" in: `gobots exec move --speed=10`
			cmdArgs := os.Args[3:]
			slog.Debug(fmt.Sprintf("os.Args: %v", cmdArgs))

			return nil
		},
	}

	// Allow unknown flags to accept DATA/FIELDS being passed to command.
	// e.g. `gobots exec move --speed 10 --direction forward`
	execCmd.FParseErrWhitelist.UnknownFlags = true

	execCmd.Flags().StringVarP(&driverFile, "driver", "d", defaultDriver,
		"The gobots driver file for your robot (optional).")
	execCmd.Flags().IntVarP(&connAttempts, "attempts", "a", 3,
		"The connection tries round, in case connection does not succeed (optional).")
	execCmd.Flags().BoolVarP(&enableDebug, "debug", "D", false,
		"Sets whether to enable debug mode/logs or not (optional).")

	return execCmd
}
