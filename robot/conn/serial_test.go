package conn

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/evias/gobots/botfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.bug.st/serial"
)

// MockSerialPort serves as a [serial.Port] compatibility bridge, and it is
// necessary because [net.Pipe] may substitue [net.Conn], but not [serial.Port].
type MockSerialPort struct {
	net.Conn
}

func (*MockSerialPort) Break(time.Duration) error                            { return nil }
func (*MockSerialPort) Drain() error                                         { return nil }
func (*MockSerialPort) GetModemStatusBits() (*serial.ModemStatusBits, error) { return nil, nil }
func (*MockSerialPort) ResetInputBuffer() error                              { return nil }
func (*MockSerialPort) ResetOutputBuffer() error                             { return nil }
func (*MockSerialPort) SetDTR(bool) error                                    { return nil }
func (*MockSerialPort) SetRTS(bool) error                                    { return nil }
func (*MockSerialPort) SetMode(*serial.Mode) error                           { return nil }
func (*MockSerialPort) SetReadTimeout(time.Duration) error                   { return nil }

// Ensure that our mock satisfies interface.
var _ serial.Port = (*MockSerialPort)(nil)

// TestNewSerialTransport tests creating a SerialTransport
func TestNewSerialTransport(t *testing.T) {
	st := NewSerialTransport(botfile.ConnectionConfig{
		Host: "/dev/ttyUSB1",
	})

	assert.NotNil(t, st.Type())
	assert.NotEmpty(t, st.String())

	expectedType := "serial"
	actualType := st.Type()
	assert.Equalf(t, expectedType, actualType,
		"transport type expected %s, got %s", expectedType, actualType)
}

// TestSerialTransport_String tests the string representation of a SerialTransport instance.
func TestSerialTransport_String(t *testing.T) {
	st := NewSerialTransport(botfile.ConnectionConfig{
		Host: "/dev/ttyUSB1",
	})

	expectedStr := "/dev/ttyUSB1"
	actualStr := st.String()
	assert.Equalf(t, expectedStr, actualStr,
		"transport URL expected %s, got %s", expectedStr, actualStr)
}

// TestSerialTransport_OpenClose injects a custom dialer to test connecting/disconnecting.
func TestSerialTransport_OpenClose(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()

	st := NewSerialTransport(botfile.ConnectionConfig{
		Host: "/dev/ttyUSB1",
	}, WithSerialDialer(func(ctx context.Context, network, addr string) (Socket, error) {
		if addr != "/dev/ttyUSB1" {
			t.Fatalf("unexpected addr %q", addr)
		}
		return &MockSerialPort{Conn: client}, nil
	}))

	openErr := st.Open()
	assert.NoError(t, openErr)
	assert.NotNil(t, st.(*SerialTransport).Socket(), "should set conn instance")

	closeErr := st.Close()
	assert.NoError(t, closeErr)
}

// TestSerialTransport_Write tests sending bytes through an opened SerialTransport.
func TestSerialTransport_Write(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	st := NewSerialTransport(botfile.ConnectionConfig{
		Host: "/dev/ttyUSB1",
	}, WithSerialDialer(func(ctx context.Context, network, addr string) (Socket, error) {
		assert.Equalf(t, "/dev/ttyUSB1", addr, "unexpected addr %q", addr)
		return &MockSerialPort{Conn: client}, nil
	}))

	err := st.Open()
	require.NoError(t, err)

	// Peer runs in a goroutine — net.Pipe is synchronous, so
	// Write blocks until the peer Reads.
	go func() {
		buf := make([]byte, 32)
		n, err := server.Read(buf)
		assert.NoError(t, err, "reading should not error")
		assert.Equalf(t, "ping", string(buf[:n]),
			"expected %q, got %q", "ping", buf[:n])
	}()

	if num, err := st.Write([]byte("ping")); err != nil {
		assert.NoError(t, err, "sending bytes should not error")
		assert.NotEmpty(t, num)
	}
}

// TestSerialTransport_Read tests reading bytes using an opened SerialTransport.
func TestSerialTransport_Read(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	st := NewSerialTransport(botfile.ConnectionConfig{
		Host: "/dev/ttyUSB1",
	}, WithSerialDialer(func(ctx context.Context, network, addr string) (Socket, error) {
		assert.Equalf(t, "/dev/ttyUSB1", addr, "unexpected addr %q", addr)
		return &MockSerialPort{Conn: client}, nil
	}))

	err := st.Open()
	require.NoError(t, err)

	go func() {
		server.Write([]byte("pong"))
		// keep the goroutine alive until the test reads, or close
	}()

	got := make([]byte, 4)
	num, err := st.Read(got)
	assert.NoError(t, err, "reading should not error")
	assert.NotEmpty(t, num)

	assert.Equalf(t, "pong", string(got),
		"expected %q, got %q", "pong", string(got))
}
