package internal

import (
	"log/slog"
	"path/filepath"

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
	rootCmd.AddCommand(NewCmdExec())
}

func Cmd() *cobra.Command {
	return rootCmd
}
