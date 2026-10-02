package driver

import (
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
