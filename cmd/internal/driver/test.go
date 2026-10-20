package driver

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/evias/gobots/botfile"
	tui "github.com/evias/gobots/cmd/internal/tui"
	"github.com/evias/gobots/robot"
)

const (
	DefaultSimulationSeconds  = 5 * time.Second
	DefaultSimulationDebounce = 100 * time.Millisecond
	DefaultSimulatedMessages  = 10
)

var (
	command     string
	numMessages int
	msgInterval string
)

func NewCmdDriverTest() *cobra.Command {
	testCmd := &cobra.Command{
		Use:     "test <command> [options]",
		Short:   "Test your edge device drivers' commands (botfiles).",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"diagnostic"},
		RunE: func(cmd *cobra.Command, args []string) error {
			command = strings.ToLower(args[0])
			initLogs()

			// Load the device driver configuration file (YAML) into a [botfile.Driver].
			var (
				useRobot *robot.Robot
				driver   botfile.Driver
				err      error
			)
			if driver, err = loadDriver(driverFile); err != nil {
				return err
			}

			if command == "help" || !driver.HasCommand(command) {
				cmd.Usage()
				return nil
			}

			if numMessages <= 0 {
				numMessages = DefaultSimulatedMessages
			}

			slog.Debug(fmt.Sprintf("Driver: %s", driverFile))
			slog.Debug(fmt.Sprintf("Host: %s", driver.Host()))
			slog.Debug(fmt.Sprintf("Port: %d", driver.Port()))

			// e.g. everything after "move" in: `gobots driver test move --speed=10`
			dataArgs := os.Args[4:]
			commandArgv := command + " " + strings.Join(dataArgs, " ")
			slog.Debug(fmt.Sprintf("Command: %s", commandArgv))

			// Convert the command arguments to uppercased structure fields.
			// i.e. --duration becomes Duration, --speed becomes Speed.
			messageArgs := tui.ParseDataArgs(driver, command, dataArgs)

			execInterval := time.Duration(DefaultSimulationDebounce)
			if len(msgInterval) > 0 {
				var err error
				execInterval, err = time.ParseDuration(msgInterval)
				if err != nil {
					slog.Error(fmt.Sprintf("failed to parse debounce interval (duration): %s", err.Error()))
					return err
				}
			}

			// Fields not forwarded to driver command.
			delete(messageArgs, "Interval")
			delete(messageArgs, "Duration")
			delete(messageArgs, "Driver")
			delete(messageArgs, "Debug")

			// Create a [robot.Robot] to interpret the [botfile.Driver],
			// and create a [robot.Simulator] around it.
			useRobot = robot.New(driver)
			simulator := robot.NewSimulator(useRobot)

			simulationCtx, cancelCtx := context.WithCancel(cmd.Context())
			defer cancelCtx()

			// Simulate a connection to an edge device.
			if err := simulator.Start(simulationCtx); err != nil {
				slog.Error(fmt.Sprintf("failed to connect with gobot: %s", err.Error()))
				return err
			}

			// GOROUTINE 1: simulation writer (messaging) process.
			// ----------------------------------------------------------------
			// Feed the simulator with message frames in parallel, each pass of
			// this goroutine makes the below main-thread blocking loop progress.
			// And execute the shutdown process when all messages have been sent.
			go func() {
				slog.Debug(fmt.Sprintf("Now simulating %d messages for command: %s", numMessages, command))

				// Shutdown simulation after we have written all simulated messages.
				// This releases deadlocks for the main-thread blocking loop below.
				defer simulator.Stop()

				// Send messages through the [robot.Simulator] instance.
				// Messages are NOT sent to the edge device.
				for i := 0; i < numMessages; i++ {
					commandCfg := driver.WireConfig(command, nil)
					wireMessage, err := botfile.NewMessage(commandCfg, messageArgs)
					if err != nil {
						slog.Error(fmt.Sprintf("failed to prepare simulated message: %s", err.Error()))
						return
					}

					if err := simulator.Send(wireMessage); err != nil {
						slog.Error(fmt.Sprintf("failed to simulate sending '%s' command: %s", command, err.Error()))
						return
					}

					// TODO(evias): use something else than sleep.
					time.Sleep(execInterval)
				}
			}()

			// BLOCKING main-thread intentionally
			// ----------------------------------------------------------------
			// We wait for all messages to be received and forwarded onto the
			// receive channel, i.e. consumed as message frames sequentially.
			for frame := range simulator.Frames() {
				bzMsg, err := frame.Message.ToBytes()
				if err != nil {
					slog.Error(fmt.Sprintf("failed to read frame: %s", err.Error()))
					return err
				}

				fmt.Printf("received simulated frame sim-%.6d (%d bytes)\n", frame.Seq, len(bzMsg))
			}

			return nil
		},
	}

	// Allow unknown flags to accept DATA/FIELDS being passed to command.
	// e.g. `gobots driver test move --speed 10 --direction forward`
	testCmd.FParseErrWhitelist.UnknownFlags = true

	testCmd.Flags().StringVarP(&driverFile, "driver", "d", defaultDriver,
		"The gobots driver file for your robot (optional).")
	testCmd.Flags().IntVarP(&numMessages, "count", "c", DefaultSimulatedMessages,
		"The number of messages to simulate (optional).")
	testCmd.Flags().StringVarP(&msgInterval, "interval", "i", DefaultSimulationDebounce.String(),
		"Sets the debounce interval between messages, defaults to 100ms (optional).")
	testCmd.Flags().BoolVarP(&enableDebug, "debug", "D", false,
		"Sets whether to enable debug mode/logs or not (optional).")

	return testCmd
}
