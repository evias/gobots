package internal

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/evias/gobots/botfile"
	"github.com/evias/gobots/cmd/internal/tui"
	"github.com/evias/gobots/robot"
)

var (
	command  string
	useRobot *robot.Robot
)

// TODO(evias): hostOrDriver is currently unset when running `gobots exec`.
// TODO(evias): Usage of the exec command should return commands by driver.
// TODO(evias): Flags suggestions should contain command fields/params from driver.
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

			// --attempts
			if connAttempts > 0 && connAttempts != int(driver.Config().Connection.MaxAttempts) {
				botfile.WithMaxAttempts(uint16(connAttempts))(driver)
			}
			// --timeout
			if connTimeoutMs > 0 && connTimeoutMs != int(driver.Config().Connection.TimeoutMs) {
				botfile.WithTimeoutMs(uint32(connTimeoutMs))(driver)
			}

			if command == "help" || !driver.HasCommand(command) {
				cmd.Usage()
				return nil
			}

			slog.Debug(fmt.Sprintf("Driver: %s", driverFile))
			slog.Debug(fmt.Sprintf("Host: %s", driver.Host()))
			slog.Debug(fmt.Sprintf("Port: %d", driver.Port()))

			// e.g. everything after "move" in: `gobots exec move --speed=10`
			dataArgs := os.Args[3:]
			commandArgv := command + " " + strings.Join(dataArgs, " ")
			slog.Debug(fmt.Sprintf("Command: %s", commandArgv))

			// TODO(evias): use device differenciation and/or better state transitions for subcommands.
			// Now connect to the edge device using the driver configuration.
			useRobot = robot.New(driver)
			if err := useRobot.Connect(driver.Config().Connection); err != nil {
				slog.Error(fmt.Sprintf("failed to connect with gobot: %s", err.Error()))
				return err
			}
			defer useRobot.Disconnect()

			// Convert the command arguments to an actual [botfile.Message].
			wireMessage := driver.WireConfig(command, nil)
			messageArgs := tui.ParseDataArgs(driver, command, dataArgs)

			// Default exec duration of 1 second to avoid looping forever.
			execDuration := time.Duration(1 * time.Second)
			if duration, ok := messageArgs["Duration"]; ok && len(duration) > 0 {
				var err error
				execDuration, err = time.ParseDuration(duration)
				if err != nil {
					slog.Error(fmt.Sprintf("failed to parse command duration: %s", err.Error()))
					return err
				}
				delete(messageArgs, "Duration") // Duration is not forwarded to command
			}

			// Underlying call to [botfile.Message#ToBytes] fills template with messageArgs.
			if err := useRobot.Send(botfile.NewMessage(wireMessage), messageArgs); err != nil {
				slog.Error(fmt.Sprintf("failed to send '%s' command: %s", command, err.Error()))
				return err
			}

			// TODO(evias): See [Robot#sleepOrQuit], should not use time.Sleep directly.
			time.Sleep(execDuration)

			// TODO(evias): Refactor to [Robot#Shutdown()].
			// Check if there is a teardown process configured for the executed command.
			executedCmd := driver.CommandConfig(command)
			if len(executedCmd.Shutdown) > 0 {
				// Run every shutdown command sequentially.
				for i := 0; i < len(executedCmd.Shutdown); i++ {
					shutdownCmd := executedCmd.Shutdown[i]

					// Send automatic shutdown message(s).
					stopMessage := driver.WireConfig(shutdownCmd, nil)
					if err := useRobot.Send(botfile.NewMessage(stopMessage), nil); err != nil {
						slog.Error(fmt.Sprintf("failed to send command shutdown '%s': %s", shutdownCmd, err.Error()))
						return err
					}
				}
			}

			// Check if there is a teardown process configured for the driver.
			driverCfg := driver.Config()
			if len(driverCfg.Shutdown) > 0 {
				// Run every shutdown command sequentially.
				for i := 0; i < len(driverCfg.Shutdown); i++ {
					shutdownCmd := driverCfg.Shutdown[i]

					// Send automatic shutdown message(s).
					stopMessage := driver.WireConfig(shutdownCmd, nil)
					if err := useRobot.Send(botfile.NewMessage(stopMessage), nil); err != nil {
						slog.Error(fmt.Sprintf("failed to send driver shutdown '%s': %s", shutdownCmd, err.Error()))
						return err
					}
				}
			}

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
	execCmd.Flags().IntVarP(&connTimeoutMs, "timeout", "T", 3000,
		"The number of milliseconds until connection timeout (optional).")
	execCmd.Flags().BoolVarP(&enableDebug, "debug", "D", false,
		"Sets whether to enable debug mode/logs or not (optional).")

	return execCmd
}
