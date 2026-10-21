package robot

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/evias/gobots/botfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRobot is a stub [IRobot] for simulator tests. All methods panic when
// called, as the simulator must not invoke them.
type fakeRobot struct {
	IRobot
}

// Ensure that our stub satisfies interface.
var _ IRobot = (*fakeRobot)(nil)

// newTestSimulator creates a started simulator with a clean logger.
func newTestSimulator(t *testing.T, ctx context.Context) Simulator {
	t.Helper()

	sim := NewSimulator(&fakeRobot{}, SimulatorWithLogger(slog.New(slog.DiscardHandler)))
	require.NoError(t, sim.Start(ctx))
	t.Cleanup(func() {
		_ = sim.Stop()
	})

	return sim
}

// recvFrame reads one frame from the frames channel with a timeout.
func recvFrame(t *testing.T, ch <-chan Frame, timeout time.Duration) Frame {
	t.Helper()

	select {
	case f, ok := <-ch:
		require.Truef(t, ok, "frames channel closed before a frame was received")
		return f
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for frame after %s", timeout)
		return Frame{}
	}
}

// errWireable is a stub [botfile.Wireable] which fails to marshal.
type errWireable struct{}

var _ botfile.Wireable = (*errWireable)(nil)

func (errWireable) ToBytes() ([]byte, error)          { return nil, errors.New("marshal failed") }
func (errWireable) String() string                    { return "" }
func (errWireable) WriteTo(io.Writer) (int64, error)  { return 0, errors.New("write failed") }
func (errWireable) ReadFrom(io.Reader) (int64, error) { return 0, errors.New("read failed") }

// TestNewSimulator tests construction defaults.
func TestNewSimulator(t *testing.T) {
	robot := &fakeRobot{}

	sim := NewSimulator(robot)

	require.NotNil(t, sim)
	assert.Equal(t, robot, sim.Robot())
	assert.NotNil(t, sim.Quit())
	assert.NotNil(t, sim.Frames())

	// Neither quit nor frames should be closed initially.
	select {
	case <-sim.Quit():
		t.Error("quit channel should not be closed initially")
	case <-sim.Frames():
		t.Error("frames channel should not be closed initially")
	default:
	}

	// Channels must not leak goroutines: Stop on a never-started simulator.
	require.NoError(t, sim.Stop())
}

// TestSimulator_WithLogger tests the logger option helper.
func TestSimulator_WithLogger(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	sim := NewSimulator(&fakeRobot{}, SimulatorWithLogger(logger))

	simLogger, ok := sim.(*robotSimulator)
	require.True(t, ok, "expected *robotSimulator")
	assert.Equal(t, logger, simLogger.logger)
}

// TestSimulator_WithContext tests the context option helper.
func TestSimulator_WithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	sim := NewSimulator(&fakeRobot{}, SimulatorWithContext(ctx, cancel))

	simCtx, ok := sim.(*robotSimulator)
	require.True(t, ok, "expected *robotSimulator")
	assert.Equal(t, ctx, simCtx.ctx)
	assert.NotNil(t, simCtx.cancelCtx)
}

// TestSimulator_SendReceiveRoundTrip tests sending messages through the
// simulator and receiving them as frames.
func TestSimulator_SendReceiveRoundTrip(t *testing.T) {
	sim := newTestSimulator(t, context.Background())

	for i := 1; i <= 3; i++ {
		msg, err := botfile.NewMessage(botfile.WireConfig{Format: "{Stop}"}, nil)
		require.NoError(t, err)

		require.NoError(t, sim.Send(msg))

		frame := recvFrame(t, sim.Frames(), 2*time.Second)
		assert.Equal(t, uint64(i), frame.Seq, "frame sequence must increment")
		assert.Equal(t, "{Stop}", frame.Message.String())
	}
}

// TestSimulator_Send_ArgumentsRoundTrip tests that rendered template
// arguments survive the round trip.
func TestSimulator_Send_ArgumentsRoundTrip(t *testing.T) {
	sim := newTestSimulator(t, context.Background())

	args := struct {
		Speed     int
		Direction string
	}{Speed: 10, Direction: "forward"}

	msg, err := botfile.NewMessage(botfile.WireConfig{
		Format: `{"N":102,"D1":{{.Speed}},"D2":"{{.Direction}}"}`,
	}, args)
	require.NoError(t, err)

	require.NoError(t, sim.Send(msg))

	frame := recvFrame(t, sim.Frames(), 2*time.Second)
	assert.Equal(t, uint64(1), frame.Seq)
	assert.Equal(t, `{"N":102,"D1":10,"D2":"forward"}`, frame.Message.String())
}

// TestSimulator_Send_MarshalError tests that marshalling failures are
// returned without writing to the stream.
func TestSimulator_Send_MarshalError(t *testing.T) {
	sim := newTestSimulator(t, context.Background())

	err := sim.Send(&errWireable{})

	assert.ErrorContains(t, err, "marshal failed")

	// The stream is untouched: a valid message still round-trips.
	msg, err := botfile.NewMessage(botfile.WireConfig{Format: "{Stop}"}, nil)
	require.NoError(t, err)
	require.NoError(t, sim.Send(msg))

	frame := recvFrame(t, sim.Frames(), 2*time.Second)
	assert.Equal(t, "{Stop}", frame.Message.String())
}

// rawWireable is a stub [botfile.Wireable] sending raw, non size-prefixed
// bytes.
type rawWireable string

var _ botfile.Wireable = (*rawWireable)(nil)

func (r rawWireable) ToBytes() ([]byte, error) { return []byte(r), nil }
func (r rawWireable) String() string           { return string(r) }
func (r rawWireable) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write([]byte(r))
	return int64(n), err
}
func (r rawWireable) ReadFrom(io.Reader) (int64, error) { return 0, errors.New("read failed") }

// TestSimulator_Stop_Idempotent tests that Stop may be called multiple times.
func TestSimulator_Stop_Idempotent(t *testing.T) {
	sim := newTestSimulator(t, context.Background())

	require.NoError(t, sim.Stop())
	require.NoError(t, sim.Stop()) // must not panic nor block

	// Quit channel is closed after Stop.
	select {
	case <-sim.Quit():
	default:
		t.Error("quit channel should be closed after Stop")
	}
}

// TestSimulator_Stop_ClosesFrames tests that the frames channel is closed
// after Stop, allowing consumers to drain remaining frames.
func TestSimulator_Stop_ClosesFrames(t *testing.T) {
	sim := newTestSimulator(t, context.Background())

	msg, err := botfile.NewMessage(botfile.WireConfig{Format: "{Stop}"}, nil)
	require.NoError(t, err)
	require.NoError(t, sim.Send(msg))

	frame := recvFrame(t, sim.Frames(), 2*time.Second)
	require.NotNil(t, frame.Message)

	require.NoError(t, sim.Stop())

	// Frames channel is closed after the receive routine exits.
	_, ok := <-sim.Frames()
	assert.False(t, ok, "frames channel should be closed after Stop")
}

// TestSimulator_Send_AfterStop tests that sending on a stopped simulator
// returns an error.
func TestSimulator_Send_AfterStop(t *testing.T) {
	sim := newTestSimulator(t, context.Background())

	require.NoError(t, sim.Stop())

	msg, err := botfile.NewMessage(botfile.WireConfig{Format: "{Stop}"}, nil)
	require.NoError(t, err)

	assert.Error(t, sim.Send(msg))
}
