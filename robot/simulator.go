package robot

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"

	"github.com/evias/gobots/botfile"
	apiconn "github.com/evias/gobots/robot/conn"
)

// Sender describes the contract for structure used as senders of messages.
type Sender interface {
	// Send should send the [botfile.Message], and attach arguments.
	Send(botfile.Wireable) error
}

// Simulator describes the contract for edge device simulator structures.
//
// A simulator acts as a robot- or edge device- simulation instrument which
// may be used to send and receive messages to/from an edge device.
//
// A concrete (private) implementation of this interface is provided with
// the structure [robotSimulator].
type Simulator interface {
	Sender

	// Robot should return a [IRobot] instance.
	Robot() IRobot
	// Start should execute a receiveRoutine.
	Start(context.Context) error
	// Stop should shutdown the receiveRoutine.
	Stop() error
	// Quit should return a channel which is closed when the instance is stopped.
	Quit() <-chan struct{}
	// Frames should return the receive channel (where messages are faked).
	Frames() <-chan Frame
}

// SimulatorOption defines the contract for [Driver] option helpers.
type SimulatorOption func(Simulator)

// ----------------------------------------------------------------------------

// Frame describes one message received from a (simulated) edge device.
//
// TODO(evias): Add fields source, timestamp, etc.
type Frame struct {
	Seq     uint64
	Message botfile.Wireable
}

// robotSimulator is a concrete implementation of the [Simulator] interface.
type robotSimulator struct {
	robot  IRobot
	logger *slog.Logger

	// Internals
	wg        sync.WaitGroup
	ctx       context.Context
	cancelCtx context.CancelFunc
	once      sync.Once
	quit      chan struct{}

	// Data - Guarded
	mtx    *sync.Mutex
	client apiconn.Streamable // under mtx
	server apiconn.Streamable // under mtx
	reader *bufio.Reader      // under mtx
	writer *bufio.Writer      // under mtx
	frames chan Frame

	lastFrameSeq atomic.Int64
}

// Ensure that our implementation satisfies interface.
var _ Simulator = (*robotSimulator)(nil)

// NewSimulator creates a [Simulator] instance around a [IRobot] object using
// options.
//
// To avoid auto-disconnect after [DefaultDisconnectSeconds], see the option
// helper [SimulatorWithContext] which allows external code to share ownership.
func NewSimulator(robot IRobot, options ...SimulatorOption) Simulator {
	cli, srv := net.Pipe()
	sim := &robotSimulator{
		robot:  robot,
		logger: slog.Default(),

		quit:   make(chan struct{}), // unbuffered
		mtx:    new(sync.Mutex),
		client: cli,
		server: srv,
		frames: make(chan Frame, 32),

		// Defaults, overwritten by Start or SimulatorWithContext.
		ctx:       context.Background(),
		cancelCtx: func() {},
	}

	sim.lastFrameSeq.Store(int64(0))

	for _, option := range options {
		option(sim)
	}

	return sim
}

// SimulatorWithLogger implements an option helper to inject a custom [slog.Logger].
func SimulatorWithLogger(logger *slog.Logger) SimulatorOption {
	return func(s Simulator) {
		rs := s.(*robotSimulator)
		rs.logger = logger
	}
}

// SimulatorWithContext implements an option helper to inject a custom [context.Context].
// Note that the owner of the context then shares ownership over timeout/cancellation.
func SimulatorWithContext(ctx context.Context, cancelFn context.CancelFunc) SimulatorOption {
	return func(s Simulator) {
		rs := s.(*robotSimulator)
		rs.ctx = ctx
		rs.cancelCtx = cancelFn
	}
}

// Robot should return a [IRobot] instance.
// Robot implements [Simulator].
func (sim *robotSimulator) Robot() IRobot {
	return sim.robot
}

// Start should execute continuously until shutdown and should process incoming
// messages to mimic a receiveRoutine reading from an opened stream, and which
// sends periodical heartbeat commands every [DefaultHeartbeatDurationMs] when
// the driver configuration contains a heartbeat command.
//
// Continues processing incoming messages and heartbeats until ctx expires,
// or gets cancelled. Additionally, this method will return given a fatal error,
// e.g. unsuccessful heartbeat or reading errors.
//
// Start implements [Simulator].
func (sim *robotSimulator) Start(ctx context.Context) error {
	sim.ctx, sim.cancelCtx = context.WithCancel(ctx)

	sim.writer = bufio.NewWriter(sim.server)
	sim.reader = bufio.NewReader(sim.client)

	sim.wg.Add(1)
	go func() {
		defer sim.wg.Done()
		sim.receiveRoutine(sim.ctx)
	}()

	return nil
}

// Stop should shutdown the receiveRoutine.
func (sim *robotSimulator) Stop() error {
	sim.logger.Debug("Stopping simulator...")

	// Close exactly once
	sim.once.Do(func() {
		// Ignore shutdown errors and continue
		sim.closeStream()
		sim.cancelCtx()
		close(sim.quit)
	})

	// block if closing
	<-sim.quit
	sim.wg.Wait() // waits for receive goroutine to defer
	return nil
}

// Send should send the [botfile.Message], and attach arguments.
func (sim *robotSimulator) Send(msg botfile.Wireable) error {
	sim.mtx.Lock()
	defer sim.mtx.Unlock()

	var (
		payload  []byte
		msgError error
	)
	if payload, msgError = msg.ToBytes(); msgError != nil {
		return msgError
	}

	if _, err := sim.writer.Write(payload); err != nil {
		return err
	}
	return sim.writer.Flush()
}

// Quit should return a channel which is closed when the instance is stopped.
func (sim *robotSimulator) Quit() <-chan struct{} {
	return sim.quit
}

// Frames returns the receive channel. Closed when the simulator stops.
func (sim *robotSimulator) Frames() <-chan Frame { return sim.frames }

// ----------------------------------------------------------------------------
// Routines

// receiveRoutine creates a long-running loop that reads messages from the
// simulated [bufio.Reader] instance, interprets the bytes as [botfile.Message]
// structures to validate their content, and communicates processed frames
// on the receive channel, i.e. see [robotSimulator#Frames].
func (sim *robotSimulator) receiveRoutine(ctx context.Context) {
	defer close(sim.frames) // signal consumers: no more frames

	buf := make([]byte, DefaultReadBufferSize)
	for i := 0; ; i++ {
		n, err := sim.reader.Read(buf)
		if n > 0 {
			rawMessage, err := botfile.LoadMessageFromBytes(buf[:n])
			if err != nil {
				// Invalid payloads are skipped: no frame is emitted and the
				// message sequence does not increment.
				sim.logger.Error("failed to create simulated message", "err", err)
			} else {
				frameSeq := sim.lastFrameSeq.Load() + int64(1)
				frame := Frame{Seq: uint64(frameSeq), Message: rawMessage}
				sim.lastFrameSeq.Store(frameSeq)

				select {
				case sim.frames <- frame:
					sim.logger.Debug(fmt.Sprintf("received frame sim-%.6d (%d bytes): %v", frame.Seq, n, frame.Message.String()))
				case <-ctx.Done():
					sim.logger.Debug("[sim] simulation context cancelled")
					return
				}
			}
		}
		if err != nil {
			return // EOF, timeout, or closed connection
		}
		if ctx.Err() != nil {
			return
		}
	}
}

// ----------------------------------------------------------------------------
// Private helpers implementation

// closeStream closes the client and server [apiconn.Streamable] instances.
func (sim *robotSimulator) closeStream() error {
	if err := sim.client.Close(); err != nil {
		sim.logger.Error("failed to close simulation consumer", "err", err)
		return err
	}
	if err := sim.server.Close(); err != nil {
		sim.logger.Error("failed to close simulation producer", "err", err)
		return err
	}
	return nil
}
