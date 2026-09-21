package internal

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	driverFile    string
	connAttempts  int
	enableDebug   bool
	defaultDriver = filepath.Join("drivers", "robot.yaml")

	rootCmd = &cobra.Command{
		Use:   "gobots <command>",
		Short: "gobots, talk with your robots>",
		RunE: func(cmt *cobra.Command, args []string) error {
			slog.Info("Welcome to gobots")
			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(NewCmdRun())
	rootCmd.AddCommand(NewCmdConsole())
}

func Cmd() *cobra.Command {
	return rootCmd
}

// Internal interface for logging
type logger interface {
	Info(msg string, keyvals ...interface{})
}

// TrapSignal catches the SIGTERM/SIGINT and executes cb function, then it exits
// with code 0.
func TrapSignal(logger logger, cb func(), sigs ...os.Signal) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, sigs...)
	go func() {
		for sig := range c {
			logger.Info("signal trapped", "msg", fmt.Sprintf("captured %v, exiting...", sig))
			if cb != nil {
				cb()
			}
			os.Exit(0)
		}
	}()
}

// Kill the running process by sending itself SIGTERM.
func Kill() error {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGTERM)
}
