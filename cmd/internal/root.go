package internal

import (
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	driverFile    string
	connAttempts  int
	connTimeoutMs int
	enableDebug   bool
	defaultDriver = filepath.Join("drivers", "robot.yaml")

	rootCmd = &cobra.Command{
		Use:   "gobots <command>",
		Short: "gobots, talk with your robots>",
	}
)

func init() {
	rootCmd.AddCommand(NewCmdRun())
	rootCmd.AddCommand(NewCmdConsole())
	rootCmd.AddCommand(NewCmdExec())
	rootCmd.AddCommand(NewCmdConnect())
	rootCmd.AddCommand(NewCmdDriver())
}

func Cmd() *cobra.Command {
	return rootCmd
}
