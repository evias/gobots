package driver

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const (
	defaultIncludePath = "drivers/"
	searchDriversGlob  = "**/*.yaml"
)

var (
	includePaths []string
)

func NewCmdDriverList() *cobra.Command {
	listCmd := &cobra.Command{
		Use:     "list [options]",
		Short:   "List available edge device drivers (botfiles).",
		Args:    cobra.NoArgs,
		Aliases: []string{"ls", "search"},
		// This PreRunE ensures that --include options are used with actual
		// folder paths, to avoid any work to happen before this validation.
		PreRunE: func(cmd *cobra.Command, args []string) error {
			for _, dir := range includePaths {
				info, err := os.Stat(dir)
				if err != nil {
					return fmt.Errorf("--include %q: %w", dir, err)
				}
				if !info.IsDir() {
					return fmt.Errorf("--include %q: not a directory", dir)
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// handle --debug flag
			initLogs()

			if len(includePaths) == 0 {
				includePaths = []string{defaultIncludePath}
			}

			driverFiles := []string{driverFile}
			for _, includePath := range includePaths {
				matches, err := filepath.Glob(filepath.Join(includePath, searchDriversGlob))
				if err != nil {
					slog.Error(fmt.Sprintf("failed to browse include path: %s", err.Error()))
					return err
				}

				driverFiles = append(driverFiles, matches...)
			}

			slog.Debug(fmt.Sprintf("Found %d driver files across %d include paths",
				len(driverFiles),
				len(includePaths)))

			return nil
		},
	}

	listCmd.Flags().StringArrayVarP(&includePaths, "include", "I", []string{defaultIncludePath},
		"Specify folders to include for the drivers search (optional).")
	listCmd.Flags().StringVarP(&driverFile, "driver", "d", defaultDriver,
		"The gobots driver file for your robot (optional).")
	listCmd.Flags().BoolVarP(&enableDebug, "debug", "D", false,
		"Sets whether to enable debug mode/logs or not (optional).")

	return listCmd
}
