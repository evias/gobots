package connect

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/evias/gobots/botfile"
	"github.com/evias/gobots/robot"
)

var (
	host         string
	driverFile   string
	connAttempts int
	enableDebug  bool

	defaultDriver = filepath.Join("drivers", "robot.yaml")
)

func NewCmdConnect() *cobra.Command {
	seedCmd := &cobra.Command{
		Use:   "connect <host> [options]",
		Short: "Connect to gobots with your botfiles.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host = args[0]
			slog.Info(fmt.Sprintf("Using gobot driver: %s", driverFile))

			if _, err := os.Stat(driverFile); err != nil && os.IsNotExist(err) {
				slog.Error(fmt.Sprintf("failed to open driver file: %s", err.Error()))
				return err
			}

			driver, err := botfile.LoadFromConfig(driverFile)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to load driver file: %s", err.Error()))
				return err
			}

			logLevel := new(slog.LevelVar)
			if enableDebug {
				logLevel.Set(slog.LevelDebug)
				handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
					Level: logLevel,
				})
				slog.SetDefault(slog.New(handler))
			}

			robot := robot.New(driver)
			slog.Info(fmt.Sprintf("Gobots driver name: %s", robot.Name()))

			// 10 seconds max connectivity ("keep-alive").
			connCtx, cancelCtx := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancelCtx()

			if err := robot.TryConnect(connCtx, host, 3); err != nil {
				slog.Error(fmt.Sprintf("failed to connect with gobot: %s", err.Error()))
				return err
			}
			defer robot.Disconnect(host)

			select {
			case <-time.After(2 * time.Second):
			}

			// Send MOVE, then wait 2 seconds, then STOP
			type MoveRequest struct {
				Direction int
				Speed     int
			}

			moveWire := driver.WireConfig("move")
			moveArgs := MoveRequest{Direction: 1, Speed: 50}
			if err := robot.Send(host, botfile.NewMessage(moveWire), moveArgs); err != nil {
				slog.Error(fmt.Sprintf("failed to send MOVE command: %s", err.Error()))
				return err
			}

			select {
			case <-time.After(2 * time.Second):
			}

			stopWire := driver.WireConfig("stop")
			if err := robot.Send(host, botfile.NewMessage(stopWire), nil); err != nil {
				slog.Error(fmt.Sprintf("failed to send STOP command: %s", err.Error()))
				return err
			}

			select {
			case <-connCtx.Done():
				slog.Info(fmt.Sprintf("Connection context expired"))

			case <-robot.Quit():
				slog.Info(fmt.Sprintf("Robot is going to sleep..."))
			}

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
