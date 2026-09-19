package conn

import (
	"context"
	"net"
)

// DialFunc defines the predicate for dialer functions around a context, network
// and address. For more details around parameters: [net.Dialer#DialContext].
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// Transport defines the contract for robot connection wrappers.
type Transport interface {
	// Type should return the transport type, e.g. "tcp", "serial", "ble".
	Type() string

	// Dialer should return a dial function, or [net.Dialer#DialContext]
	Dialer() DialFunc

	// Addr should return the remote address or nil, i.e. [net.Conn#RemoteAddr].
	Addr() net.Addr
	// Conn should return a [net.Conn] instance or nil.
	Conn() net.Conn

	// Open should dial the remote address, i.e. connect to the remote.
	Open() error
	// Close should close the connection to the remote.
	Close() error

	// Read should read bytes from the stream, p must be pre-allocated.
	Read(p []byte) (int, error)
	// Write should send bytes to the stream, p must be pre-allocated.
	Write(p []byte) (int, error)

	// String should return a string representation of the instance.
	String() string
}

// TransportOption defines the contract for [Transport] option helpers.
type TransportOption func(Transport)
