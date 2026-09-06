package robot

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/evias/gobots/botfile"
)

const (
	DefaultConnectionTimeout   = 3 * time.Second
	DefaultReceiveWaitPeriod   = 300 * time.Millisecond
	DefaultHeartbeatDurationMs = 1000

	DefaultReadBufferSize = 1024
)

// XXX
type Robot struct {
	mtx *sync.Mutex

	// Configuration
	driver botfile.Driver
	conns  map[string]net.Conn // under mtx
	logger *slog.Logger

	// Data
	lastHeartbeatRecvTime atomic.Int64
	lastHeartbeatSendTime atomic.Int64
	lastMessageRecvTime   atomic.Int64
	lastMessageSendTime   atomic.Int64

	// Internals
	quit chan struct{}
	errs []error
}

// XXX
type RobotOption func(*Robot)

// Ensure that our implementation satisfies interface.
var _ IRobot = (*Robot)(nil)

// XXX
func New(
	driver botfile.Driver,
	options ...RobotOption,
) *Robot {
	r := &Robot{
		mtx: new(sync.Mutex),

		driver: driver,
		conns:  map[string]net.Conn{}, // under mtx
		logger: slog.Default(),

		quit: make(chan struct{}), // unbuffered
		errs: []error{},
	}

	for _, option := range options {
		option(r)
	}

	return r
}

// XXX
func WithDriver(driver botfile.Driver) RobotOption {
	return func(r *Robot) {
		r.driver = driver
	}
}

// XXX
func WithLogger(logger *slog.Logger, lvl slog.Level) RobotOption {
	return func(r *Robot) {
		r.logger = logger
	}
}

// XXX
func (r *Robot) Name() string {
	return r.driver.Name()
}

// Quit Implements IRobot by returning a quit channel.
func (r *Robot) Quit() <-chan struct{} {
	return r.quit
}

// XXX
func (r *Robot) Connect(ctx context.Context, host string) error {
	return r.TryConnect(ctx, host, 1)
}

// XXX
func (r *Robot) TryConnect(
	connCtx context.Context,
	host string,
	attempts int,
) error {
	var (
		hostWithPort = hostWithPort(host, int(r.driver.Port()))
		conn         net.Conn
		dialer       net.Dialer
		err          error
	)

	for i := 0; i < attempts; i++ {
		at := i + 1
		r.logger.Debug(fmt.Sprintf("Connecting to host %s", hostWithPort),
			"attempts", at,
		)

		dialCtx, cancelFn := context.WithTimeout(context.Background(), DefaultConnectionTimeout)
		defer cancelFn()

		if conn, err = dialer.DialContext(dialCtx, "tcp", hostWithPort); err != nil {
			r.addError(err)
			r.logger.Error("Failed connection attempt",
				"host", hostWithPort, "attempts", at,
				"err", err.Error(),
			)
			continue
		}

		r.logger.Info(fmt.Sprintf("Connected to host %s", hostWithPort))

		r.mtx.Lock()
		r.conns[hostWithPort] = conn
		r.mtx.Unlock()

		// XXX
		go r.receiveRoutine(connCtx, hostWithPort)
		return nil
	}

	return err
}

// XXX
func (r *Robot) Disconnect(hosts ...string) error {
	for _, host := range hosts {
		hwp := hostWithPort(host, int(r.driver.Port()))

		r.mtx.Lock()
		if c, ok := r.conns[hwp]; ok {
			r.logger.Debug(fmt.Sprintf("Disconnecting from host %s", hwp))
			c.Close()
			delete(r.conns, hwp)
		}
		r.mtx.Unlock()

		r.logger.Info(fmt.Sprintf("Disconnected from host %s", hwp))
	}

	return nil
}

// XXX
func (r *Robot) IsConnected(host string) bool {
	hostWithPort := hostWithPort(host, int(r.driver.Port()))
	return r.hasConn(hostWithPort)
}

// XXX
func (r *Robot) Send(host string, msg botfile.Message) error {
	hostWithPort := hostWithPort(host, int(r.driver.Port()))

	if !r.hasConn(hostWithPort) {
		return fmt.Errorf("not connected to %s", host)
	}

	bzSent, err := msg.ToBytes()
	if err != nil {
		return fmt.Errorf("failed to format message: %w", err)
	}
	bzSent = append(bzSent, byte('\n'))

	conn := r.getConn(hostWithPort)
	if _, err := bufio.NewWriter(conn).Write(bzSent); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	r.logger.Debug(fmt.Sprintf("[-> OUT] %v", string(bzSent)), "to", hostWithPort)
	return nil
}

// ----------------------------------------------------------------------------
// Routines

// XXX
func (r *Robot) receiveRoutine(connCtx context.Context, hostWithPort string) {
	if !r.driver.HasCommand("heartbeat") || !r.hasConn(hostWithPort) {
		return
	}

	r.logger.Debug(fmt.Sprintf("Starting receiveRoutine for host %s", hostWithPort))
	defer func() {
		r.logger.Debug(fmt.Sprintf("Stopped receiveRoutine for host %s", hostWithPort))
		r.Disconnect(hostWithPort)
	}()

	for connCtx.Err() == nil {
		conn := r.getConn(hostWithPort)

		bytes := make([]byte, DefaultReadBufferSize) // XXX bufferSize from driver
		num, err := bufio.NewReader(conn).Read(bytes)

		if num == 0 {
			// Check if we must send a heartbeat, i.e. heartbeat frequency.
			elapsedSinceLast := time.Now().UnixNano() - r.lastHeartbeatSendTime.Load()
			if elapsedSinceLast >= 1e6*DefaultHeartbeatDurationMs { // XXX heartbeat frequence from driver
				if err := r.sendHeartbeat(hostWithPort); err != nil {
					r.logger.Error(fmt.Sprintf("Error sending heartbeat to %s", hostWithPort),
						"err", err,
					)
					return
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
			r.logger.Error(fmt.Sprintf("Error reading bytes from %s", hostWithPort),
				"err", err,
			)
			return
		}

		bz := bytes[:num]
		r.logger.Debug(fmt.Sprintf("[<-  IN] %s", string(bz)), "num", num, "from", hostWithPort)

		// Received heartbeat, send heartbeat response.
		if string(bz) == "{Heartbeat}" {
			r.lastHeartbeatRecvTime.Store(time.Now().UnixNano())
			r.sendHeartbeat(hostWithPort)
		}
	}
}

// ----------------------------------------------------------------------------
// Private implementation

// XXX
func (r *Robot) sendHeartbeat(hostWithPort string) error {
	if !r.driver.HasCommand("heartbeat") {
		return nil
	}

	heartbeatCmd := r.driver.WireConfig("heartbeat")
	if err := r.Send(hostWithPort, botfile.NewMessage(heartbeatCmd), nil); err != nil {
		return err
	}

	r.lastHeartbeatSendTime.Store(time.Now().UnixNano())
	return nil
}

// addError stores an error internally.
func (r *Robot) addError(err error) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	r.errs = append(r.errs, err)
}

// hasConn returns whether we have an open connection with hostWithPort.
func (r *Robot) hasConn(hostWithPort string) bool {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	_, hasConn := r.conns[hostWithPort]
	return hasConn
}

// getConn returns the net.Conn instance connected with hostWithPort.
func (r *Robot) getConn(hostWithPort string) net.Conn {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	c := r.conns[hostWithPort]
	return c
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
