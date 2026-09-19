package robot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/evias/gobots/botfile"
	apierr "github.com/evias/gobots/errors"
	apiconn "github.com/evias/gobots/robot/conn"
)

const (
	// DefaultConnectionTimeout contains the number of Seconds before connection attempt(s) time out.
	DefaultConnectionTimeout = 3 * time.Second
	// DefaultDisconnectSeconds contains the maximum number of Seconds a connection is kept alive.
	DefaultDisconnectSeconds = 10 * time.Second
	// DefaultReceiveWaitPeriod contains the number of milliseconds wait time between reading processes.
	DefaultReceiveWaitPeriod = 300 * time.Millisecond
	// DefaultHeartbeatDurationMs contains the number of milliseconds between heartbeat runs.
	DefaultHeartbeatDurationMs = 1000

	// DefaultReadBufferSize contains the size of the read buffer, allocated before read.
	DefaultReadBufferSize = 1024
)

// Robot provides a connection wrapper and communication channel for connected
// edge devices. This implementation is agnostic to the actual edge devices'
// hardware and installed firmware.
//
// A communication channel may be opened using supported protocols, including
// WiFi, BLE, Serial connections. And messages are sent using the [botfile.Wire]
// messaging protocol as defined in this package.
//
// Notably, this implementation makes it possible to create runnable flows,
// which automate the communication with edge devices, using human-readable
// YAML configuration files.
type Robot struct {
	// Configuration
	driver botfile.Driver
	logger *slog.Logger

	// Data
	lastHeartbeatRecvTime atomic.Int64
	lastHeartbeatSendTime atomic.Int64
	lastMessageRecvTime   atomic.Int64
	lastMessageSendTime   atomic.Int64
	autoDisconnectAfter   atomic.Int64

	// Internals
	ctx       context.Context
	cancelCtx context.CancelFunc
	quit      chan struct{}

	// Connection (Guarded)
	mtx           *sync.Mutex
	hostWithPort  string
	remoteAddress net.Addr
	connTransport apiconn.Transport
}

// RobotOption defines a [Robot] option helper.
type RobotOption func(*Robot)

// Ensure that our implementation satisfies interface.
var _ IRobot = (*Robot)(nil)

// New creates a new Robot instance around a [botfile.Driver].
func New(
	driver botfile.Driver,
	options ...RobotOption,
) *Robot {
	r := &Robot{
		driver: driver,
		logger: slog.Default(),

		quit: make(chan struct{}), // unbuffered
		mtx:  new(sync.Mutex),
	}

	r.autoDisconnectAfter.Store(int64(DefaultDisconnectSeconds / time.Second))

	for _, option := range options {
		option(r)
	}

	autoDisconnectAfter := r.autoDisconnectAfter.Load()
	if autoDisconnectAfter != int64(0) {
		// auto-disconnect never exceeds max connectivity ("keep-alive").
		r.ctx, r.cancelCtx = context.WithTimeout(context.Background(), time.Duration(r.autoDisconnectAfter.Load()))
	} else {
		r.ctx = context.Background()
	}

	return r
}

// WithDriver implements an option helper to inject a custom [botfile.Driver].
func WithDriver(driver botfile.Driver) RobotOption {
	return func(r *Robot) {
		r.driver = driver
	}
}

// WithLogger implements an option helper to inject a custom [slog.Logger].
func WithLogger(logger *slog.Logger, lvl slog.Level) RobotOption {
	return func(r *Robot) {
		r.logger = logger
	}
}

// WithAutoDisconnect implements an option helper to inject a custom auto-disconnect period.
// Set to 0 to disable the auto-disconnect feature.
func WithAutoDisconnect(d time.Duration) RobotOption {
	return func(r *Robot) {
		if int64(d) <= 0 {
			r.autoDisconnectAfter.Store(int64(0))
		} else {
			r.autoDisconnectAfter.Store(int64(d / time.Second))
		}
	}
}

// Name returns the device's name as provided by Driver.
// Name implements IRobot.
func (r *Robot) Name() string {
	return r.driver.Name()
}

// Quit returns a channel which is closed when the instance is stopped.
// Quit implements IRobot.
//
// TODO(evias): Currently useless, stop/Shutdown should close(r.quit).
func (r *Robot) Quit() <-chan struct{} {
	return r.quit
}

// Transport returns a connected [apiconn.Transport] or nil.
// Transport implements IRobot.
func (r *Robot) Transport() apiconn.Transport {
	return r.connTransport
}

// Connect attempts to connect using a [botfile.ConnectionConfig] object.
// Upon successful connection, this method spawns a long-living goroutine.
//
// Returns an error given an unsuccessful call to [apiconn.Transport#Open],
// otherwise returns nil.
//
// Connect implements IRobot.
//
// TODO(evias): Make receiveRoutine configurable where necessary/possible.
func (r *Robot) Connect(
	conf botfile.ConnectionConfig,
) error {
	r.mtx.Lock()
	transport := apiconn.NewTransport(conf)
	r.mtx.Unlock()

	if err := transport.Open(); err != nil {
		return &apierr.AppError{
			Code:    apierr.ErrInvalidConnection,
			Message: "Connection failed",
			Cause:   err,
		}
	}

	r.mtx.Lock()
	r.hostWithPort = net.JoinHostPort(conf.Host, strconv.Itoa(int(conf.Port)))
	r.connTransport = transport
	r.remoteAddress = transport.Addr()
	r.mtx.Unlock()

	r.logger.Info(fmt.Sprintf("Connected to host %s", r.hostWithPort))

	// Robot is responsible for the receiving routine which listens to messages
	// from an connected peer. This routine continuously reads messages until
	// the context expires or gets cancelled.
	go r.receiveRoutine(r.ctx, transport)
	return nil
}

// Disconnect closes any opened connection to the device.
// Returns an error given an unsuccessful call to [apiconn.Transport#Close],
// otherwise returns nil.
//
// Disconnect implements IRobot.
func (r *Robot) Disconnect() error {
	if !r.IsConnected() {
		return nil // disconnecting or already disconnected
	}

	r.cancelCtx()
	if err := r.connTransport.Close(); err != nil {
		return &apierr.AppError{
			Code:    apierr.ErrNotConnected,
			Message: "Failed to disconnect",
			Cause:   err,
		}
	}

	r.mtx.Lock()
	defer r.mtx.Unlock()
	close(r.quit)

	r.logger.Info(fmt.Sprintf("Disconnected from host %s", r.hostWithPort))
	return nil
}

// IsConnected returns true given an existing opened [net.Conn] and [apiconn.Transport].
func (r *Robot) IsConnected() bool {
	return r.connTransport != nil && r.connTransport.Conn() != nil
}

// Send attempts to send a [botfile.Message] to a connected device.
// Returns an error if the instance is not connected, or returns an error
// given unsuccessful call to [bufio.Writer#Write], otherwise returns nil.
//
// Send implements IRobot.
//
// TODO(evias): Check for presence of EOF byte before sending.
func (r *Robot) Send(msg botfile.Message, args any) error {
	if r.ctx.Err() != nil || !r.IsConnected() {
		return &apierr.AppError{
			Code:    apierr.ErrNotConnected,
			Message: "Failed to send message",
			Cause:   nil,
		}
	}

	bzSent, err := msg.ToBytes(args)
	if err != nil {
		return fmt.Errorf("failed to format message: %w", err)
	}
	bzSent = append(bzSent, byte('\n')) // XXX extract EOF byte

	num, err := r.connTransport.Write(bzSent)
	if err != nil {
		r.logger.Error(fmt.Sprintf("Error sending bytes to %s", r.hostWithPort),
			"err", err,
		)
		return &apierr.AppError{
			Code:    apierr.ErrWriteFailure,
			Message: "Failed to send message",
			Cause:   err,
		}
	}

	r.logger.Debug(fmt.Sprintf("[-> OUT] %v", string(bzSent)), "num", num, "to", r.hostWithPort)
	return nil
}

// ----------------------------------------------------------------------------
// Routines

// receiveRoutine continuously reads from an opened [apiconn.Transport], and
// sends periodical heartbeat commands every [DefaultHeartbeatDurationMs].
//
// Continues processing incoming messages and heartbeats until connCtx expires,
// or gets cancelled. Additionally, this method will return given a fatal error,
// e.g. unsuccessful heartbeat or reading errors.
//
// TODO(evias): Read buffer size may be overwritten by driver.
// TODO(evias): Heartbeat frequence may be overwritten by driver.
// TODO(evias): Disconnect concurrency, disconnect should be graceful.
func (r *Robot) receiveRoutine(connCtx context.Context, transport apiconn.Transport) {
	if connCtx.Err() != nil || transport.Conn() == nil {
		return
	}

	r.logger.Debug(fmt.Sprintf("Starting receiveRoutine for host %s", r.hostWithPort))
	defer func() {
		r.logger.Debug(fmt.Sprintf("Stopped receiveRoutine for host %s", r.hostWithPort))
		r.Disconnect() // XXX concurrency
	}()

	for connCtx.Err() == nil {
		bytes := make([]byte, DefaultReadBufferSize) // XXX bufferSize from driver
		num, err := r.connTransport.Read(bytes)

		if num == 0 {
			// Check if we must send a heartbeat, i.e. heartbeat frequency.
			if r.driver.HasCommand("heartbeat") {
				elapsedSinceLast := time.Now().UnixNano() - r.lastHeartbeatSendTime.Load()
				if elapsedSinceLast >= 1e6*DefaultHeartbeatDurationMs { // XXX heartbeat frequence from driver
					if err := r.sendHeartbeat(); err != nil {
						r.logger.Error(fmt.Sprintf("Error sending heartbeat to %s", r.hostWithPort),
							"err", err,
						)
						return
					}
				}
			}

			// Otherwise relax a little... then read again.
			if ok := r.sleepOrQuit(connCtx, DefaultReceiveWaitPeriod); !ok {
				return
			}
			continue // to read
		}

		if err != nil && errors.Is(err, io.EOF) {
			if ok := r.sleepOrQuit(connCtx, DefaultReceiveWaitPeriod); !ok {
				return
			}
			continue // to read
		} else if err != nil {
			r.logger.Error(fmt.Sprintf("Error reading bytes from %s", r.hostWithPort),
				"err", err,
			)
			return
		}

		bz := bytes[:num]
		r.logger.Debug(fmt.Sprintf("[<-  IN] %s", string(bz)), "num", num, "from", r.hostWithPort)

		// Received heartbeat command request, send heartbeat response.
		if string(bz) == "{Heartbeat}" {
			r.lastHeartbeatRecvTime.Store(time.Now().UnixNano())
			r.sendHeartbeat()
		}
	}
}

// ----------------------------------------------------------------------------
// Private implementation

// sendHeartbeat formats a heartbeat [botfile.Wire] message and sends it.
// Skipped when no heartbeat command is configured.
// Returns an error given an unsuccessful message sending operation.
func (r *Robot) sendHeartbeat() error {
	if !r.driver.HasCommand("heartbeat") {
		return nil
	}

	heartbeatCmd := r.driver.WireConfig("heartbeat", nil)
	if err := r.Send(botfile.NewMessage(heartbeatCmd), nil); err != nil {
		return err
	}

	r.lastHeartbeatSendTime.Store(time.Now().UnixNano())
	return nil
}

// sleepOrQuit sleeps but listens to potential shutdown.
// It returns true if duration passed, false given shutdown while waiting.
func (r *Robot) sleepOrQuit(ctx context.Context, duration time.Duration) bool {
	select {
	case <-time.After(duration):
		return true

	case <-ctx.Done():
		return false

	case <-r.Quit():
		return false
	}
}
