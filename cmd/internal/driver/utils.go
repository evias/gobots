package driver

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

var (
	driverFile  string
	enableDebug bool

	defaultDriver = filepath.Join("drivers", "robot.yaml")
)

// initLogs is a private helper to overwrite the default log handler when
// necessary to adapt the log level to DEBUG. By default, log level is INFO.
func initLogs() {
	// --debug enables debug messages in default logger.
	logLevel := new(slog.LevelVar)
	if enableDebug {
		logLevel.Set(slog.LevelDebug)
		handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: logLevel,
		})
		slog.SetDefault(slog.New(handler))
	}
}

// sliceUnique returns a slice where s only has unique entries.
func sliceUnique(s []string) (u []string) {
	u = []string{}                      // dynamic alloc
	m := make(map[string]uint8, len(s)) // static alloc, max all keys
	for _, k := range s {
		if _, e := m[k]; e {
			continue
		}

		m[k] = uint8(1)
		u = append(u, k)
	}
	return // u
}

// ensureIncludePaths ensures that --include options are valid by returning an
// error if an entry is not a directory, or if it doesn't exist.
func ensureIncludePaths(paths []string) error {
	for _, dir := range paths {
		info, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("--include %q: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("--include %q: not a directory", dir)
		}
	}
	return nil
}
