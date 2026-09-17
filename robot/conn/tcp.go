package conn

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/evias/gobots/botfile"
	apierr "github.com/evias/gobots/errors"
)

const (
	DefaultConnectionTimeoutMs = 3000
	DefaultConnectionAttempts  = 3
)

// XXX
type TCPTransport struct {
	conf   botfile.ConnectionConfig
	logger *slog.Logger

	mtx  *sync.Mutex
	conn net.Conn // under mtx
}

// Ensure that our implementation satisfies interface.
var _ Transport = (*TCPTransport)(nil)

// XXX
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

// XXX
func WithConnectionConfig(cfg botfile.ConnectionConfig) TransportOption {
	return func(t Transport) {
		tcpt := t.(*TCPTransport)
		tcpt.conf = cfg
	}
}

// XXX
func WithLogger(logger *slog.Logger, lvl slog.Level) TransportOption {
	return func(t Transport) {
		tcpt := t.(*TCPTransport)
		tcpt.logger = logger
	}
}

// XXX
func (tcpt *TCPTransport) Addr() net.Addr {
	conn := tcpt.Conn()
	if conn == nil {
		return nil
	}

	return conn.RemoteAddr()
}

// XXX
func (tcpt *TCPTransport) Conn() net.Conn {
	tcpt.mtx.Lock()
	defer tcpt.mtx.Unlock()

	return tcpt.conn
}

// XXX
func (*TCPTransport) Type() string {
	return "tcp"
}

// XXX
func (tcpt *TCPTransport) String() string {
	protocol := tcpt.Type()
	host := tcpt.conf.Host
	port := tcpt.conf.Port

	return fmt.Sprintf("%s://%s:%d", protocol, host, port)
}

// XXX
func (tcpt *TCPTransport) Open() error {
	if tcpt.conf.TimeoutMs == 0 {
		tcpt.conf.TimeoutMs = DefaultConnectionTimeoutMs
	}

	if tcpt.conf.MaxAttempts == 0 {
		tcpt.conf.MaxAttempts = DefaultConnectionAttempts
	}

	if len(tcpt.conf.Host) == 0 || tcpt.conf.Port == 0 {
		return &apierr.AppError{
			Code:    apierr.ErrInvalidConnection,
			Message: "TCP transport requires host and port to be non-empty",
			Cause:   nil,
		}
	}

	var (
		hostWithPort = hostWithPort(tcpt.conf.Host, int(tcpt.conf.Port))
		conn         net.Conn
		dialer       net.Dialer
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

		if conn, err = dialer.DialContext(dialCtx, "tcp", hostWithPort); err != nil {
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

// XXX
func (tcpt *TCPTransport) Close() error {
	tcpt.mtx.Lock()
	defer tcpt.mtx.Unlock()

	if tcpt.conn == nil {
		return nil // conn already closed
	}

	return tcpt.conn.Close()
}

// XXX
func (tcpt *TCPTransport) Read(
	bytes []byte,
) (int, error) {
	conn := tcpt.Conn()
	stream := bufio.NewReader(conn)

	return stream.Read(bytes)
}

// XXX
func (tcpt *TCPTransport) Write(p []byte) (int, error) {
	conn := tcpt.Conn()
	stream := bufio.NewWriter(conn)

	return stream.Write(p)
}

// ----------------------------------------------------------------------------
// PRIVATE HELPERS

// XXX
func hostWithPort(host string, port int) string {
	if !strings.Contains(host, ":"+strconv.Itoa(port)) {
		host += ":" + strconv.Itoa(port)
	}

	return host
}
