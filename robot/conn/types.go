package conn

import "net"

// XXX
type ConnectionID string

// XXX
type Transport interface {
	Type() string // "tcp", "serial", "ble"

	Addr() net.Addr
	Conn() net.Conn

	Open() error  // dial / open port / BLE connect
	Close() error // close connection

	Read(p []byte) (int, error)  // stream read
	Write(p []byte) (int, error) // stream write

	String() string
}

// XXX
type TransportOption func(Transport)
