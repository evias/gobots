package driver

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	tui "github.com/evias/gobots/cmd/internal/tui"
)

const (
	defaultIncludePath = "drivers/"
	searchDriversGlob  = "**/*.yaml"
)

var (
	includePaths []string
	consoleState *term.State
	consoleDesc  int
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

			// Set console in raw mode for printing.
			var err error
			consoleDesc = int(os.Stdin.Fd())
			if consoleState, err = term.MakeRaw(consoleDesc); err != nil {
				return fmt.Errorf("terminal state raw mode failure: %w", err)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			slog.SetDefault(&slog.Logger{})
			// Restore console from raw mode after end.
			defer term.Restore(consoleDesc, consoleState)

			if len(includePaths) == 0 {
				includePaths = []string{defaultIncludePath}
			}

			// --include paths in order of appearance.
			driverFiles := []string{driverFile}
			for _, includePath := range includePaths {
				matches, err := filepath.Glob(filepath.Join(includePath, searchDriversGlob))
				if err != nil {
					slog.Error(fmt.Sprintf("failed to browse include path: %s", err.Error()))
					return err
				}

				driverFiles = append(driverFiles, matches...)
			}

			tableRows := make([]tui.TableRow, len(driverFiles))
			for _, driverFile := range driverFiles {
				filename := filepath.Base(driverFile)
				tableRows = append(tableRows, tui.TableRow{
					Values: []string{
						filename,
						driverFile,
					},
				})
			}
			tui.PrintTable(tableRows)

			return nil
		},
	}

	listCmd.Flags().StringArrayVarP(&includePaths, "include", "I", []string{defaultIncludePath},
		"Specify folders to include for the drivers search (optional).")
	listCmd.Flags().StringVarP(&driverFile, "driver", "d", defaultDriver,
		"The gobots driver file for your robot (optional).")

	return listCmd
}
