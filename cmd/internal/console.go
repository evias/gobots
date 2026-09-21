package internal

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/mattn/go-shellwords"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/evias/gobots/botfile"
	"github.com/evias/gobots/robot"
)

var (
	hostOrDriver string
	shellParser  = shellwords.NewParser()
)

func NewCmdConsole() *cobra.Command {
	seedCmd := &cobra.Command{
		Use:   "console <host> [options]",
		Short: "Connect to gobots with your botfiles and open an interactive console.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hostOrDriver = args[0]

			// If a driver file is passed as first argument, use it to connect.
			if _, err := os.Stat(hostOrDriver); err == nil {
				driverFile = hostOrDriver
			}

			if _, err := os.Stat(driverFile); err != nil && os.IsNotExist(err) {
				slog.Error(fmt.Sprintf("failed to open driver file: %s", err.Error()))
				return err
			}

			driver, err := botfile.LoadFromConfig(driverFile)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to load driver file: %s", err.Error()))
				return err
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

			var (
				deviceHost string
				devicePort string
				actualPort uint64
				hostErr    error
			)
			if hostOrDriver == driverFile {
				deviceHost = driver.Host()
				devicePort = strconv.Itoa(int(driver.Port()))
			} else if deviceHost, devicePort, hostErr = net.SplitHostPort(hostOrDriver); hostErr != nil {
				slog.Error(fmt.Sprintf("failed to use custom host: %s - %s", hostOrDriver, err.Error()))
				return err
			}

			if actualPort, err = strconv.ParseUint(devicePort, 10, 16); err != nil {
				slog.Error(fmt.Sprintf("invalid port: %s - %s", devicePort, err.Error()))
				return err
			}

			// Overwrite host/port for connection if necessary.
			botfile.WithHostAndPort(deviceHost, uint16(actualPort))(driver)

			slog.Debug(fmt.Sprintf("Driver: %s", driverFile))
			slog.Debug(fmt.Sprintf("Host: %s", driver.Host()))
			slog.Debug(fmt.Sprintf("Port: %d", driver.Port()))

			robot := robot.New(driver)
			slog.Info(fmt.Sprintf("Connecting to gobots driver: %s", robot.Name()))

			if connAttempts > 0 && connAttempts != int(driver.Config().Connection.MaxAttempts) {
				botfile.WithMaxAttempts(uint16(connAttempts))(driver)
			}

			if err := robot.Connect(driver.Config().Connection); err != nil {
				slog.Error(fmt.Sprintf("failed to connect with gobot: %s", err.Error()))
				return err
			}
			defer robot.Disconnect()

			// Stop upon receiving SIGTERM,SIGKILL or CTRL-C.
			// TrapSignal(slog.Default(), func() {
			// 	robot.Disconnect()
			// }, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL)

			p := prompt.New(
				executor,
				completer,
				prompt.OptionTitle("gobots interactive console"),
				prompt.OptionPrefix("bot> "),
				prompt.OptionLivePrefix(livePrefix),
				prompt.OptionSuggestionBGColor(prompt.LightGray),
				prompt.OptionSuggestionTextColor(prompt.Black),
				prompt.OptionDescriptionBGColor(prompt.White),
				prompt.OptionDescriptionTextColor(prompt.Black),
				prompt.OptionMaxSuggestion(4),
			)
			p.Run()
			return nil
		},
	}

	seedCmd.Flags().StringVarP(&driverFile, "driver", "d", defaultDriver,
		"The gobots driver file for your robot (optional).")
	seedCmd.Flags().IntVarP(&connAttempts, "attempts", "a", 3,
		"The connection tries round, in case connection does not succeed (optional).")
	seedCmd.Flags().BoolVarP(&enableDebug, "debug", "D", false,
		"Sets whether to enable debug mode/logs or not (optional).")

	return seedCmd
}

// -----------------------------------------------------------------------------
// Executor — runs a typed line through cobra by rewriting os.Args.
// -----------------------------------------------------------------------------

// executor is called by go-prompt every time the user presses Enter.
func executor(in string) {
	line := strings.TrimSpace(in)
	switch line {
	case "":
		return
	case "exit", "quit":
		fmt.Println("bye!")
		os.Exit(0)
	}

	if err := executeLine(line); err != nil {
		// Cobra already printed the error to stderr; keep REPL alive.
		return
	}
}

// executeLine tokenizes the line and dispatches it to the cobra root command.
// It returns the exit-style error from cobra so the REPL can report it.
func executeLine(line string) error {
	// Shell-like tokenization: handles quotes, e.g. echo "hello world"
	args, err := shellParser.Parse(line)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}
	if len(args) == 0 {
		return nil
	}

	// Persist the parsed line so the completer/executor can read the
	// original tokens if needed.
	saved := os.Args
	defer func() { os.Args = saved }()

	os.Args = append([]string{os.Args[0]}, args...)
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	return rootCmd.Execute()
}

// -----------------------------------------------------------------------------
// Completer — wired to the cobra command tree so tab-completion always
// reflects the real commands, aliases and flags.
// -----------------------------------------------------------------------------

// completerState holds tokens already typed on the current line so we can
// decide whether to suggest subcommands or flags.
func completer(d prompt.Document) []prompt.Suggest {
	args := strings.Fields(d.TextBeforeCursor())
	suggestions := []prompt.Suggest{}

	if len(args) == 0 || d.GetWordBeforeCursor() == "" {
		// Nothing typed yet on this word: suggest top-level commands
		return commandSuggestions(rootCmd)
	}

	// Walk down the cobra tree following the typed args
	cmd, flags, err := rootCmd.Find(args)
	if err != nil || cmd == nil {
		return suggestions
	}

	wordBefore := d.GetWordBeforeCursor()

	// Suggest flags when the word starts with "-"
	if strings.HasPrefix(wordBefore, "-") {
		return flagSuggestions(cmd)
	}

	// If the current command still has subcommands and the last full token
	// is a command name (or we're mid-word on one), suggest its children.
	if cmd.HasAvailableSubCommands() && len(flags) <= 1 {
		return commandSuggestions(cmd)
	}

	return suggestions
}

func commandSuggestions(cmd *cobra.Command) []prompt.Suggest {
	suggestions := []prompt.Suggest{}
	for _, sub := range cmd.Commands() {
		if !sub.IsAvailableCommand() || sub.Hidden {
			continue
		}
		suggestions = append(suggestions, prompt.Suggest{
			Text:        sub.Name(),
			Description: sub.Short,
		})
		// also expose aliases
		for _, alias := range sub.Aliases {
			suggestions = append(suggestions, prompt.Suggest{
				Text:        alias,
				Description: fmt.Sprintf("(alias of %s) %s", sub.Name(), sub.Short),
			})
		}
	}
	// Built-in REPL commands
	suggestions = append(suggestions,
		prompt.Suggest{Text: "exit", Description: "leave the interactive session"},
		prompt.Suggest{Text: "help", Description: "show help for a command"},
	)
	return suggestions
}

func flagSuggestions(cmd *cobra.Command) []prompt.Suggest {
	suggestions := []prompt.Suggest{}
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		suggestions = append(suggestions, prompt.Suggest{
			Text:        "--" + flag.Name,
			Description: flag.Usage,
		})
		if flag.Shorthand != "" {
			suggestions = append(suggestions, prompt.Suggest{
				Text:        "-" + flag.Shorthand,
				Description: flag.Usage,
			})
		}
	})
	return suggestions
}

// livePrefix shows a different prompt after the first command, like bash.
func livePrefix() (string, bool) {
	return "bot> ", true
}
