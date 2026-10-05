package conn

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"go.bug.st/serial"

	"github.com/evias/gobots/botfile"
	apierr "github.com/evias/gobots/robot/errors"
)

const (
	DefaultSerialTimeoutMs = 3000
	DefaultSerialAttempts  = 3
	DefaultSerialPort      = "/dev/ttyUSB0"
)

// SerialTransport implements the [Transport] interface to provide a Serial
// communication layer.
//
// Notable method implementations include:
// - [SerialTransport#Open]: Connect to a device over serial ports.
// - [SerialTransport#Close]: Disconnect from a connected device.
// - [SerialTransport#Read]: Read raw bytes from a connected device.
// - [SerialTransport#Write]: Send raw bytes to a connection device.
//
// This implementation is thread-safe. An internal mutex lock is taken
// for all state transitions, thereby protecting the internal conn instance.
type SerialTransport struct {
	connConf botfile.ConnectionConfig
	portConf botfile.SerialConfig
	logger   *slog.Logger

	mtx    *sync.Mutex
	port   serial.Port   // under mtx
	reader *bufio.Reader // under mtx
	writer *bufio.Writer // under mtx
	dialFn DialFunc
}

// Ensure that our implementation satisfies interface.
var _ Transport = (*SerialTransport)(nil)

// NewSerialTransport creates a new Transport instance around a [botfile.ConnectionConfig].
func NewSerialTransport(
	cfg botfile.ConnectionConfig,
	options ...TransportOption,
) Transport {
	st := &SerialTransport{
		mtx: new(sync.Mutex),

		connConf: cfg,
		logger:   slog.Default(),
	}

	for _, option := range options {
		option(st)
	}

	return st
}

// WithSerialConfig implements an option helper to inject a custom connection configuration.
func WithSerialConfig(cfg botfile.SerialConfig) TransportOption {
	return func(t Transport) {
		st := t.(*SerialTransport)
		st.portConf = cfg
	}
}

// WithSerialLogger implements an option helper to inject a custom [slog.Logger].
func WithSerialLogger(logger *slog.Logger, lvl slog.Level) TransportOption {
	return func(t Transport) {
		st := t.(*SerialTransport)
		st.logger = logger
	}
}

// WithSerialDialer implements an option helper to inject a custom [DialFunc].
func WithSerialDialer(fn DialFunc) TransportOption {
	return func(t Transport) {
		st := t.(*SerialTransport)
		st.dialFn = fn
	}
}

// Type returns the transport type, e.g. "tcp", "serial", "bluetooth".
func (*SerialTransport) Type() string {
	return "serial"
}

// Socket returns the opened socket instance, see io.ReadWriteCloser.
func (st *SerialTransport) Socket() Socket {
	st.mtx.Lock()
	defer st.mtx.Unlock()

	return st.port
}

// Dialer returns a dial function, or a custom dialer that uses the
// underlying Serial implementation from [serial#Open].
func (st *SerialTransport) Dialer() DialFunc {
	// use custom dialer process as injected
	if st.dialFn != nil {
		return st.dialFn
	}

	// fallback to [serial#Open] implementation
	return func(ctx context.Context, network, addr string) (Socket, error) {
		// TODO(evias): Take account of ctx, currently ignoring context deadline.
		return serial.Open(addr, st.Mode())
	}
}

// Mode returns a [serial.Mode] instance with defaults set to 9600_N81.
// Use [SerialTransport#portConf] or [WithSerialConfig] to overwrite.
func (st *SerialTransport) Mode() *serial.Mode {
	baudRate := 9600
	if st.portConf.BaudRate > 0 {
		baudRate = int(st.portConf.BaudRate)
	}

	parity := serial.NoParity
	if st.portConf.Parity > 0 {
		parity = serial.Parity(st.portConf.Parity)
	}

	dataBits := 8
	if st.portConf.DataBits > 0 {
		dataBits = int(st.portConf.DataBits)
	}

	stopBits := serial.OneStopBit
	if st.portConf.StopBits > 0 {
		stopBits = serial.StopBits(st.portConf.StopBits)
	}

	return &serial.Mode{
		BaudRate: baudRate,
		Parity:   parity,
		DataBits: dataBits,
		StopBits: stopBits,
	}
}

// Addr returns the remote address or nil, i.e. [net.Conn#RemoteAddr].
func (st *SerialTransport) Addr() string {
	return st.connConf.Host
}

// String returns a string representation of the instance.
func (st *SerialTransport) String() string {
	return st.Addr()
}

// Open dials the remote address, i.e. connect to the remote.
func (st *SerialTransport) Open() error {
	if st.connConf.TimeoutMs == 0 {
		st.connConf.TimeoutMs = DefaultSerialTimeoutMs
	}

	if st.connConf.MaxAttempts == 0 {
		st.connConf.MaxAttempts = DefaultSerialAttempts
	}

	if len(st.connConf.Host) == 0 {
		st.connConf.Host = DefaultSerialPort
	}

	var (
		hostWithPort = st.String()
		dialerFn     = st.Dialer()
		socket       Socket
		port         serial.Port
		err          error
	)

	for i := 0; i < int(st.connConf.MaxAttempts); i++ {
		at := i + 1
		st.logger.Debug(fmt.Sprintf("Connecting to serial port %s", hostWithPort),
			"attempts", at,
		)

		timeoutDuration := time.Duration(st.connConf.TimeoutMs) * time.Millisecond
		dialCtx, cancelFn := context.WithTimeout(context.Background(), timeoutDuration)

		// XXX dialerFn should be called in a goroutine to avoid blocking main thread.
		socket, err = dialerFn(dialCtx, "serial", hostWithPort)
		cancelFn() // cancel context directly after sync-call of dialer func

		if err == nil {
			port = socket.(serial.Port)
			break
		}

		st.logger.Error("Failed connection attempt",
			"host", hostWithPort, "attempts", at,
			"err", err.Error(),
		)
		// continue
	}

	if err != nil {
		return &apierr.AppError{
			Code:    apierr.ErrInvalidConnection,
			Message: fmt.Sprintf("Connection failed after %d attempts", st.connConf.MaxAttempts),
			Cause:   err,
		}
	}

	st.mtx.Lock()
	defer st.mtx.Unlock()

	st.port = port
	st.reader = bufio.NewReader(port)
	st.writer = bufio.NewWriter(port)
	return nil
}

// Close closes the connection to the remote.
func (st *SerialTransport) Close() error {
	st.mtx.Lock()
	defer st.mtx.Unlock()

	if st.port == nil {
		return nil // conn already closed
	}

	st.reader = nil
	st.writer = nil
	return st.port.Close()
}

// Read reads bytes from the stream, p must be pre-allocated.
func (st *SerialTransport) Read(
	bytes []byte,
) (int, error) {
	st.mtx.Lock()
	defer st.mtx.Unlock()

	if st.reader == nil {
		return 0, net.ErrClosed
	}
	return st.reader.Read(bytes)
}

// Write sends bytes to the stream, p must be pre-allocated.
func (st *SerialTransport) Write(p []byte) (int, error) {
	st.mtx.Lock()
	defer st.mtx.Unlock()

	if st.writer == nil {
		return 0, net.ErrClosed
	}
	if _, err := st.writer.Write(p); err != nil {
		return 0, err
	}
	if err := st.writer.Flush(); err != nil {
		return 0, err
	}
	return len(p), nil
}
