package conn

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/evias/gobots/botfile"
	apierr "github.com/evias/gobots/errors"
)

const (
	DefaultConnectionTimeoutMs = 3000
	DefaultConnectionAttempts  = 3
)

// TCPTransport implements the [Transport] interface to provide a TCP layer
// for connection and communication.
//
// Notable method implementations include:
// - [TCPTransport#Open]: Connect to a remote device over TCP.
// - [TCPTransport#Close]: Disconnect from a connected device.
// - [TCPTransport#Read]: Read raw bytes from a connected device.
// - [TCPTransport#Write]: Send raw bytes to a connection device.
//
// This implementation is thread-safe. An internal mutex lock is taken
// for all state transitions, thereby protecting the internal conn instance.
type TCPTransport struct {
	conf   botfile.ConnectionConfig
	logger *slog.Logger

	mtx    *sync.Mutex
	conn   net.Conn // under mtx
	dialFn DialFunc
}

// Ensure that our implementation satisfies interface.
var _ Transport = (*TCPTransport)(nil)

// NewTCPTransport creates a new Transport instance around a [botfile.ConnectionConfig].
func NewTCPTransport(
	cfg botfile.ConnectionConfig,
	options ...TransportOption,
) Transport {
	tcpt := &TCPTransport{
		mtx: new(sync.Mutex),

		conf:   cfg,
		logger: slog.Default(),
	}

	for _, option := range options {
		option(tcpt)
	}

	return tcpt
}

// WithConnectionConfig implements an option helper to inject a custom connection configuration.
func WithConnectionConfig(cfg botfile.ConnectionConfig) TransportOption {
	return func(t Transport) {
		tcpt := t.(*TCPTransport)
		tcpt.conf = cfg
	}
}

// WithLogger implements an option helper to inject a custom [slog.Logger].
func WithLogger(logger *slog.Logger, lvl slog.Level) TransportOption {
	return func(t Transport) {
		tcpt := t.(*TCPTransport)
		tcpt.logger = logger
	}
}

// WithDialer implements an option helper to inject a custom [DialFunc].
func WithDialer(fn DialFunc) TransportOption {
	return func(t Transport) {
		tcpt := t.(*TCPTransport)
		tcpt.dialFn = fn
	}
}

// Type returns the transport type, e.g. "tcp", "serial", "ble".
func (*TCPTransport) Type() string {
	return "tcp"
}

// Dialer should return a dial function, or [net.Dialer#DialContext]
func (tcpt *TCPTransport) Dialer() DialFunc {
	// use custom dialer process as injected
	if tcpt.dialFn != nil {
		return tcpt.dialFn
	}

	// fallback to net.Dialer implementation
	return (&net.Dialer{
		Timeout: DefaultConnectionTimeoutMs * time.Millisecond,
	}).DialContext
}

// Addr returns the remote address or nil, i.e. [net.Conn#RemoteAddr].
func (tcpt *TCPTransport) Addr() net.Addr {
	conn := tcpt.Conn()
	if conn == nil {
		return nil
	}

	return conn.RemoteAddr()
}

// Conn returns a [net.Conn] instance or nil.
func (tcpt *TCPTransport) Conn() net.Conn {
	tcpt.mtx.Lock()
	defer tcpt.mtx.Unlock()

	return tcpt.conn
}

// String returns a string representation of the instance.
func (tcpt *TCPTransport) String() string {
	return net.JoinHostPort(tcpt.conf.Host, strconv.Itoa(int(tcpt.conf.Port)))
}

// Open dials the remote address, i.e. connect to the remote.
func (tcpt *TCPTransport) Open() error {
	if tcpt.conf.TimeoutMs == 0 {
		tcpt.conf.TimeoutMs = DefaultConnectionTimeoutMs
	}

	if tcpt.conf.MaxAttempts == 0 {
		tcpt.conf.MaxAttempts = DefaultConnectionAttempts
	}

	if len(tcpt.conf.Host) == 0 {
		tcpt.conf.Host = "127.0.0.1"
	}

	var (
		hostWithPort = tcpt.String()
		dialerFn     = tcpt.Dialer()
		conn         net.Conn
		err          error
	)

	for i := 0; i < int(tcpt.conf.MaxAttempts); i++ {
		at := i + 1
		tcpt.logger.Debug(fmt.Sprintf("Connecting to host %s", hostWithPort),
			"attempts", at,
		)

		timeoutDuration := time.Duration(tcpt.conf.TimeoutMs) * time.Millisecond
		dialCtx, cancelFn := context.WithTimeout(context.Background(), timeoutDuration)
		defer cancelFn()

		// XXX dialerFn should be called in a goroutine to avoid blocking main thread.
		if conn, err = dialerFn(dialCtx, "tcp", hostWithPort); err != nil {
			// r.addError(err)
			tcpt.logger.Error("Failed connection attempt",
				"host", hostWithPort, "attempts", at,
				"err", err.Error(),
			)
			continue
		}
	}

	if err != nil {
		return &apierr.AppError{
			Code:    apierr.ErrInvalidConnection,
			Message: fmt.Sprintf("Connection failed after %d attempts", tcpt.conf.MaxAttempts),
			Cause:   err,
		}
	}

	tcpt.mtx.Lock()
	defer tcpt.mtx.Unlock()

	tcpt.conn = conn
	return nil
}

// Close closes the connection to the remote.
func (tcpt *TCPTransport) Close() error {
	tcpt.mtx.Lock()
	defer tcpt.mtx.Unlock()

	if tcpt.conn == nil {
		return nil // conn already closed
	}

	return tcpt.conn.Close()
}

// Read reads bytes from the stream, p must be pre-allocated.
func (tcpt *TCPTransport) Read(
	bytes []byte,
) (int, error) {
	conn := tcpt.Conn()
	stream := bufio.NewReader(conn)

	return stream.Read(bytes)
}

// Write sends bytes to the stream, p must be pre-allocated.
func (tcpt *TCPTransport) Write(p []byte) (int, error) {
	conn := tcpt.Conn()
	stream := bufio.NewWriter(conn)

	return stream.Write(p)
}
