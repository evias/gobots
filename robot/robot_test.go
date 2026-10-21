package robot

import (
	"context"
	"encoding/binary"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/evias/gobots/botfile"
	apiconn "github.com/evias/gobots/robot/conn"
	apierr "github.com/evias/gobots/robot/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRobotDriver provides a [botfile.Driver] fixture without a heartbeat
// command, so the receive routine stays passive.
func testRobotDriver() botfile.Driver {
	return botfile.NewDriver(botfile.DriverConfig{
		Name: "Test Device",
		Connection: botfile.ConnectionConfig{
			Type: "tcp",
			Host: "127.0.0.1",
			Port: 1234,
		},
	})
}

// testHeartbeatDriver provides a [botfile.Driver] fixture with a heartbeat
// command configured.
func testHeartbeatDriver() botfile.Driver {
	return botfile.NewDriver(botfile.DriverConfig{
		Name: "Heartbeat Device",
		Connection: botfile.ConnectionConfig{
			Type: "tcp",
			Host: "127.0.0.1",
			Port: 1234,
		},
		Commands: map[string]botfile.CommandConfig{
			"heartbeat": {
				Wire: map[string]botfile.WireConfig{
					"string": {Format: "{Heartbeat}"},
				},
			},
		},
	})
}

// startTestListener starts a TCP listener on a random local port.
func startTestListener(t *testing.T) (net.Listener, string) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	return listener, listener.Addr().String()
}

// acceptTestConn accepts one connection with a timeout.
func acceptTestConn(t *testing.T, listener net.Listener) net.Conn {
	t.Helper()

	listener.(*net.TCPListener).SetDeadline(time.Now().Add(2 * time.Second))
	conn, err := listener.Accept()
	require.NoError(t, err)

	return conn
}

// readTestConn reads n bytes from conn with a deadline.
func readTestConn(t *testing.T, conn net.Conn, n int) []byte {
	t.Helper()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))

	buf := make([]byte, n)
	_, err := conn.Read(buf)
	require.NoError(t, err)

	return buf
}

// TestNewRobot tests construction defaults.
func TestNewRobot(t *testing.T) {
	driver := testRobotDriver()

	r := New(driver)

	require.NotNil(t, r)
	assert.Equal(t, "Test Device", r.Name())
	assert.Equal(t, driver, r.Driver())
	assert.NotNil(t, r.Quit())
	assert.Nil(t, r.Transport())
	assert.False(t, r.IsConnected())

	// Internal defaults: default logger and auto-disconnect period.
	assert.Equal(t, slog.Default(), r.logger)
	assert.Equal(t, int64(DefaultDisconnectSeconds/time.Second), r.autoDisconnectAfter.Load())
	assert.NoError(t, r.ctx.Err(), "context should not be expired with the default period")
}

// TestRobot_QuitChannelOpen tests that the quit channel stays open while the
// robot is running.
func TestRobot_QuitChannelOpen(t *testing.T) {
	r := New(testRobotDriver())

	select {
	case <-r.Quit():
		t.Error("quit channel should not be closed while running")
	default:
	}
}

// TestRobot_WithDriver tests the driver option helper.
func TestRobot_WithDriver(t *testing.T) {
	driver := testHeartbeatDriver()

	r := New(testRobotDriver(), WithDriver(driver))

	assert.Equal(t, driver, r.Driver())
	assert.Equal(t, "Heartbeat Device", r.Name())
}

// TestRobot_WithLogger tests the logger option helper.
// TODO: the slog.Level argument of WithLogger is currently ignored.
func TestRobot_WithLogger(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	r := New(testRobotDriver(), WithLogger(logger, slog.LevelDebug))

	assert.Equal(t, logger, r.logger)
}

// TestRobot_WithAutoDisconnect tests the auto-disconnect option helper.
func TestRobot_WithAutoDisconnect(t *testing.T) {
	testCases := []struct {
		name     string
		duration time.Duration
		expected int64
	}{
		// TODO: sub-second durations truncate to 0, silently disabling
		// the auto-disconnect feature.
		{name: "sub_second_disables", duration: 500 * time.Millisecond, expected: 0},
		{name: "zero_disables", duration: 0, expected: 0},
		{name: "five_seconds", duration: 5 * time.Second, expected: 5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := New(testRobotDriver(), WithAutoDisconnect(tc.duration))

			assert.Equal(t, tc.expected, r.autoDisconnectAfter.Load())
		})
	}
}

// TestRobot_WithAutoDisconnect_Zero tests that 0 disables the auto-disconnect
// context deadline entirely.
func TestRobot_WithAutoDisconnect_Zero(t *testing.T) {
	r := New(testRobotDriver(), WithAutoDisconnect(0))

	assert.Equal(t, context.Background(), r.ctx)
}

// TestRobot_AutoDisconnectExpires tests that the robot context expires after
// the configured auto-disconnect period.
func TestRobot_AutoDisconnectExpires(t *testing.T) {
	r := New(testRobotDriver(), WithAutoDisconnect(1*time.Second))

	require.Eventually(t, func() bool {
		return r.ctx.Err() != nil
	}, 3*time.Second, 50*time.Millisecond, "context should expire after 1s")
}

// TestRobot_ConnectDisconnect tests connecting to a local TCP listener and
// disconnecting.
func TestRobot_ConnectDisconnect(t *testing.T) {
	_, addr := startTestListener(t)
	host, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := net.LookupPort("tcp", portStr)
	require.NoError(t, err)

	r := New(testRobotDriver())
	defer func() {
		if r.IsConnected() {
			_ = r.Disconnect()
		}
	}()

	err = r.Connect(botfile.ConnectionConfig{
		Type: "tcp",
		Host: host,
		Port: uint16(port),
	})

	require.NoError(t, err)
	assert.True(t, r.IsConnected())
	assert.NotNil(t, r.Transport())

	// Disconnect closes the quit channel.
	require.NoError(t, r.Disconnect())

	select {
	case <-r.Quit():
	default:
		t.Error("quit channel should be closed after Disconnect")
	}

	// TODO: IsConnected still returns true after Disconnect because the
	// transport is not reset to nil.
	assert.True(t, r.IsConnected(), "transport is expected to remain non-nil after Disconnect")

	// Disconnecting again fails on the already-closed transport.
	err = r.Disconnect()
	require.Error(t, err)
	assert.True(t, apierr.IsCode(err, apierr.ErrNotConnected))
}

// TestRobot_ConnectFailure tests connecting to an unreachable port.
func TestRobot_ConnectFailure(t *testing.T) {
	listener, addr := startTestListener(t)
	host, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := net.LookupPort("tcp", portStr)
	require.NoError(t, err)

	// Free the port so nothing listens on it anymore.
	require.NoError(t, listener.Close())

	r := New(testRobotDriver())

	err = r.Connect(botfile.ConnectionConfig{
		Type: "tcp",
		Host: host,
		Port: uint16(port),
	})

	require.Error(t, err)
	assert.True(t, apierr.IsCode(err, apierr.ErrInvalidConnection))
	assert.False(t, r.IsConnected())
}

// TestRobot_Send_NotConnected tests sending without an opened connection.
func TestRobot_Send_NotConnected(t *testing.T) {
	r := New(testRobotDriver())

	msg, err := botfile.NewMessage(botfile.WireConfig{Format: "{Stop}"}, nil)
	require.NoError(t, err)

	err = r.Send(msg)

	require.Error(t, err)
	assert.True(t, apierr.IsCode(err, apierr.ErrNotConnected))
}

// TestRobot_Send_AfterContextExpiry tests sending after the robot context
// expired.
func TestRobot_Send_AfterContextExpiry(t *testing.T) {
	r := New(testRobotDriver())
	r.cancelCtx() // simulate expiry

	msg, err := botfile.NewMessage(botfile.WireConfig{Format: "{Stop}"}, nil)
	require.NoError(t, err)

	err = r.Send(msg)

	require.Error(t, err)
	assert.True(t, apierr.IsCode(err, apierr.ErrNotConnected))
}

// TestRobot_SendRoundTrip tests sending a message to a connected TCP peer.
func TestRobot_SendRoundTrip(t *testing.T) {
	listener, addr := startTestListener(t)
	host, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := net.LookupPort("tcp", portStr)
	require.NoError(t, err)

	r := New(testRobotDriver())
	defer func() {
		if r.IsConnected() {
			_ = r.Disconnect()
		}
	}()

	require.NoError(t, r.Connect(botfile.ConnectionConfig{
		Type: "tcp",
		Host: host,
		Port: uint16(port),
	}))

	serverConn := acceptTestConn(t, listener)
	defer serverConn.Close()

	msg, err := botfile.NewMessage(botfile.WireConfig{Format: "{Stop}"}, nil)
	require.NoError(t, err)

	require.NoError(t, r.Send(msg))

	// The peer receives the size-prefixed payload.
	expected, err := msg.MarshalBinary()
	require.NoError(t, err)

	got := readTestConn(t, serverConn, len(expected))
	assert.Equal(t, expected, got)
}

// TestRobot_SendHeartbeatOnConnect tests that a driver with a heartbeat
// command makes the receive routine send a heartbeat message immediately
// after connecting.
func TestRobot_SendHeartbeatOnConnect(t *testing.T) {
	listener, addr := startTestListener(t)
	host, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := net.LookupPort("tcp", portStr)
	require.NoError(t, err)

	r := New(testHeartbeatDriver())
	defer func() {
		if r.IsConnected() {
			_ = r.Disconnect()
		}
	}()

	require.NoError(t, r.Connect(botfile.ConnectionConfig{
		Type: "tcp",
		Host: host,
		Port: uint16(port),
	}))

	serverConn := acceptTestConn(t, listener)
	defer serverConn.Close()

	// The receive routine sends the heartbeat message on its first pass.
	got := readTestConn(t, serverConn, 4+len("{Heartbeat}"))

	n := binary.BigEndian.Uint32(got[:4])
	assert.Equal(t, uint32(len("{Heartbeat}")), n)
	assert.Equal(t, "{Heartbeat}", string(got[4:]))
}

// TestRobot_SleepOrQuit tests the interruptible sleep helper.
func TestRobot_SleepOrQuit(t *testing.T) {
	t.Run("duration_elapsed", func(t *testing.T) {
		r := New(testRobotDriver())

		assert.True(t, r.sleepOrQuit(context.Background(), 10*time.Millisecond))
	})

	t.Run("context_cancelled", func(t *testing.T) {
		r := New(testRobotDriver())
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		assert.False(t, r.sleepOrQuit(ctx, 10*time.Millisecond))
	})

	t.Run("quit_closed", func(t *testing.T) {
		r := New(testRobotDriver())
		close(r.quit)

		assert.False(t, r.sleepOrQuit(context.Background(), 10*time.Millisecond))
	})
}

// TestRobot_Constants tests the default constants sanity.
func TestRobot_Constants(t *testing.T) {
	assert.Equal(t, 3*time.Second, DefaultConnectionTimeout)
	assert.Equal(t, 10*time.Second, DefaultDisconnectSeconds)
	assert.Equal(t, 300*time.Millisecond, DefaultReceiveWaitPeriod)
	assert.Equal(t, 1000, DefaultHeartbeatDurationMs)
	assert.Equal(t, 1024, DefaultReadBufferSize)

	// Ensure that our implementation satisfies interface.
	assert.Implements(t, (*IRobot)(nil), New(testRobotDriver()))
	assert.Implements(t, (*apiconn.Transport)(nil), apiconn.NewTransport(botfile.ConnectionConfig{}))
}
