package internal

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/evias/gobots/botfile"
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

			if connAttempts > 0 && connAttempts != int(driver.Config().Connection.MaxAttempts) {
				botfile.WithMaxAttempts(uint16(connAttempts))(driver)
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
			messageArgs := parseDataArgs(driver, command, dataArgs)

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

			// Check if there is a teardown process configured for the executed command.
			executedCmd := driver.CommandConfig(command)
			if len(executedCmd.Shutdown) > 0 {
				// Run every shutdown command sequentially.
				for i := 0; i < len(executedCmd.Shutdown); i++ {
					shutdownCmd := executedCmd.Shutdown[i]

					// Send automatic shutdown message(s).
					stopMessage := driver.WireConfig(shutdownCmd, nil)
					if err := useRobot.Send(botfile.NewMessage(stopMessage), nil); err != nil {
						slog.Error(fmt.Sprintf("failed to send shutdown '%s' command: %s", shutdownCmd, err.Error()))
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
	execCmd.Flags().BoolVarP(&enableDebug, "debug", "D", false,
		"Sets whether to enable debug mode/logs or not (optional).")

	return execCmd
}

// -----------------------------------------------------------------------------
// Parser — parses custom data arguments passed to gobots driver commands.
// -----------------------------------------------------------------------------

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	r, i := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[i:]
}

// sliceToMap converts a slice of bash-style arguments, e.g. "--speed", into a
// key-value map with string keys and values.
func sliceToMap(args []string) map[string]string {
	m := make(map[string]string)
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			// detects --, but nothing to do
			continue
		}
		if strings.HasPrefix(args[i], "--") {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				m[args[i]] = args[i+1]
				i++ // consume the value
			} else if i+1 < len(args) && strings.Contains(args[i], "=") {
				nv := strings.SplitN(args[i], "=", 2)
				m[nv[0]] = nv[1]
			} else {
				m[args[i]] = "" // flag with no value (or boolean-style flag)
			}
		}
	}
	return m
}

// parseDataArgs parses data arguments, typically passed through os.Args in a
// format similar to e.g. `--speed 10` or `--direction=forward`.
//
// Returns a map[string]string with keys from [botfile.CommandConfig#Fields]
// and [botfile.CommandConfig#Params].
func parseDataArgs(
	driver botfile.Driver,
	command string,
	args []string,
) (data map[string]string) {
	wireCommand := driver.Config().Commands[command]

	// v contains "--sleep", "--direction" keys.
	v := sliceToMap(args)

	// data contains "sleep", "direction" keys.
	data = make(map[string]string, len(args))

	// Make sure we have all fields (required).
	for _, field := range wireCommand.Fields {
		f := "--" + strings.ToLower(field)
		a, ok := v[f]

		// Golang text/template expects uppercase-first keys.
		key := upperFirst(field)

		data[key] = ""
		if ok {
			data[key] = a
		}
	}

	// Encode the content of params, i.e. "forward" becomes 1.
	// Empty/Non-present parameters are ignored (optional).
	for param, paramValues := range wireCommand.Params {
		if len(paramValues) == 0 {
			continue
		}

		p := "--" + strings.ToLower(string(param))
		a, ok := v[string(p)]
		if !ok {
			continue
		}

		// Golang text/template expects uppercase-first keys.
		key := upperFirst(string(param))

		for pvn, pv := range paramValues {
			if a != string(pvn) {
				continue
			}

			if v, ok := pv.(uint64); ok {
				data[key] = strconv.FormatUint(v, 10)
				break
			} else if v, ok := pv.(int); ok {
				data[key] = strconv.Itoa(v)
				break
			} else if v, ok := pv.(string); ok {
				data[key] = v
				break
			}
		}
	}

	return /* data */
}
