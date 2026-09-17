package robot

import (
	"github.com/evias/gobots/botfile"
	apiconn "github.com/evias/gobots/robot/conn"
)

// IRobot defines the contract for Robot devices.
type IRobot interface {
	// Name should return the device's name as provided by Driver.
	Name() string

	// Quit should return a channel which is closed when the instance is stopped.
	Quit() <-chan struct{}

	// Transport should return a connected transport or nil.
	Transport() apiconn.Transport

	// Connect should attempt to connect using a [botfile.ConnectionConfig] object.
	Connect(conf botfile.ConnectionConfig) error

	// Disconnect should close any opened connection to the device.
	Disconnect() error

	// Send should attempt to send a [botfile.Message] to a connected device.
	Send(msg botfile.Message, args any) error
}
