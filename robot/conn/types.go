package conn

import (
	"context"
	"io"
)

// Socket defines the contract for connection sockets, notably it embeds
// the [io.ReadWriteCloser] interface to require: Read(), Write(), Close().
type Socket interface {
	io.ReadWriteCloser
}

// DialFunc defines the predicate for dialer functions around a context, network
// and address. For more details around parameters: [net.Dialer#DialContext].
type DialFunc func(ctx context.Context, network, address string) (Socket, error)

// Streamable defines the contract for stream structures from/to which we may
// read/write, and which may be closed.
type Streamable interface {
	// Close should close the connection to the remote.
	Close() error

	// Read should read bytes from the stream, p must be pre-allocated.
	Read(p []byte) (int, error)
	// Write should send bytes to the stream, p must be pre-allocated.
	Write(p []byte) (int, error)
}

// Transport defines the contract for robot connection wrappers.
type Transport interface {
	Streamable

	// Type should return the transport type, e.g. "tcp", "serial", "ble".
	Type() string

	// Socket should return the opened socket instance, see io.ReadWriteCloser.
	Socket() Socket

	// Addr should return the remote address or nil, i.e. [net.Conn#RemoteAddr].
	Addr() string

	// Open should dial the remote address, i.e. connect to the remote.
	Open() error

	// String should return a string representation of the instance.
	String() string
}

// TransportOption defines the contract for [Transport] option helpers.
type TransportOption func(Transport)
