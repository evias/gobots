package robot

import (
	"context"

	"github.com/evias/gobots/botfile"
)

// XXX
type IRobot interface {
	Name() string

	// Quit returns a channel which is closed when the instance is stopped.
	Quit() <-chan struct{}

	Connect(ctx context.Context, host string) error
	TryConnect(ctx context.Context, host string, attempts int) error

	Disconnect(hosts ...string) error

	Send(host string, msg botfile.Message, args any) error
}
