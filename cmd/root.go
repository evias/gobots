package main

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/evias/gobots/cmd/connect"
	"github.com/evias/gobots/cmd/runner"
)

var (
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
	rootCmd.AddCommand(runner.NewCmdRun())
	rootCmd.AddCommand(connect.NewCmdConnect())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
