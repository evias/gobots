package internal

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/evias/gobots/botfile"
	"github.com/evias/gobots/robot"
)

var (
	duration        string
	defaultDuration = "3s"
)

func NewCmdConnect() *cobra.Command {
	connectCmd := &cobra.Command{
		Use:   "connect <host> [options]",
		Short: "Connect to your gobots with your botfiles.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hostOrDriver = args[0]

			if hostOrDriver == "help" {
				cmd.Usage()
				return nil
			}

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

			if connAttempts > 0 && connAttempts != int(driver.Config().Connection.MaxAttempts) {
				botfile.WithMaxAttempts(uint16(connAttempts))(driver)
			}

			slog.Debug(fmt.Sprintf("Driver: %s", driverFile))
			slog.Debug(fmt.Sprintf("Host: %s", driver.Host()))
			slog.Debug(fmt.Sprintf("Port: %d", driver.Port()))

			// TODO(evias): use device differenciation and/or better state transitions for subcommands.
			// Now connect to the edge device using the driver configuration.
			useRobot = robot.New(driver)
			if err := useRobot.Connect(driver.Config().Connection); err != nil {
				slog.Error(fmt.Sprintf("failed to connect with gobot: %s", err.Error()))
				return err
			}
			defer useRobot.Disconnect()

			if duration != "0" && len(duration) > 0 {
				var (
					execDuration time.Duration
					errDuration  error
				)
				execDuration, errDuration = time.ParseDuration(duration)
				if errDuration != nil {
					slog.Error(fmt.Sprintf("failed to parse keep-alive duration: %s", errDuration.Error()))
					return errDuration
				}

				// TODO(evias): See [Robot#sleepOrQuit], should not use time.Sleep directly.
				time.Sleep(execDuration)
			} else {
				slog.Debug("Connection established; Keep-alive [Ctrl+C to exit]")

				// Runs forever with `--duration=0`
				select {}
			}

			return nil
		},
	}

	// Allow unknown flags to accept DATA/FIELDS being passed to command.
	// e.g. `gobots exec move --speed 10 --direction forward`
	connectCmd.FParseErrWhitelist.UnknownFlags = true

	connectCmd.Flags().StringVarP(&driverFile, "driver", "d", defaultDriver,
		"The gobots driver file for your robot (optional).")
	connectCmd.Flags().IntVarP(&connAttempts, "attempts", "a", 3,
		"The connection tries round, in case connection does not succeed (optional).")
	connectCmd.Flags().StringVarP(&duration, "duration", "t", defaultDuration,
		"The duration to keep the connection alive, set to 0 for long-running process (optional).")
	connectCmd.Flags().BoolVarP(&enableDebug, "debug", "D", false,
		"Sets whether to enable debug mode/logs or not (optional).")

	return connectCmd
}
