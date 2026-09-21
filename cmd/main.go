package main

import (
	"os"

	"github.com/spf13/cobra"

	apicmd "github.com/evias/gobots/cmd/internal"
)

var rootCmd *cobra.Command

func main() {
	rootCmd = apicmd.Cmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
