package conn

import (
	"context"
	"net"
	"testing"

	"github.com/evias/gobots/botfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tinygo.org/x/bluetooth"
)

// TODO(evias): Add unit tests for BLEConn#SetCurrentChannel, BLEConn#Discover

type MockBluetoothChannel struct{}

func (c MockBluetoothChannel) Write(p []byte) (int, error)    { return len(p), nil }
func (c MockBluetoothChannel) Read(bytes []byte) (int, error) { return len(bytes), nil }

var _ BLEChannel = (*MockBluetoothChannel)(nil)

func mockBLEConn() *BLEConn {
	return &BLEConn{
		device:         nil,
		currentChannel: &MockBluetoothChannel{},
		servicesByUUID: map[bluetooth.UUID]bluetooth.DeviceService{},
		channelsByUUID: map[bluetooth.UUID]BLEChannel{},
	}
}

// TestNewBLETransport tests creating a BLETransport
func TestNewBLETransport(t *testing.T) {
	blet := NewBLETransport(botfile.ConnectionConfig{
		Host: "AA:BB:CC:DD:EE:FF",
	})

	assert.NotNil(t, blet.Type())
	assert.NotEmpty(t, blet.String())

	expectedType := "bluetooth"
	actualType := blet.Type()
	assert.Equalf(t, expectedType, actualType,
		"transport type expected %s, got %s", expectedType, actualType)
}

// TestBLETransport_String tests the string representation of a BLETransport instance.
func TestBLETransport_String(t *testing.T) {
	blet := NewBLETransport(botfile.ConnectionConfig{
		Host: "AA:BB:CC:DD:EE:FF",
	})

	expectedStr := "AA:BB:CC:DD:EE:FF"
	actualStr := blet.String()
	assert.Equalf(t, expectedStr, actualStr,
		"transport URL expected %s, got %s", expectedStr, actualStr)
}

// TestBLETransport_OpenClose injects a custom dialer to test connecting/disconnecting.
func TestBLETransport_OpenClose(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	blet := NewBLETransport(botfile.ConnectionConfig{
		Host: "AA:BB:CC:DD:EE:FF",
	}, WithBLEDialer(func(ctx context.Context, network, addr string) (Socket, error) {
		if addr != "AA:BB:CC:DD:EE:FF" {
			t.Fatalf("unexpected addr %q", addr)
		}
		return mockBLEConn(), nil
	}))

	openErr := blet.Open()
	assert.NoError(t, openErr)
	assert.NotNil(t, blet.(*BLETransport).Socket(), "should set conn instance")

	closeErr := blet.Close()
	assert.NoError(t, closeErr)
}

// TestBLETransport_Write tests sending bytes through an opened BLETransport.
func TestBLETransport_Write(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	blet := NewBLETransport(botfile.ConnectionConfig{
		Host: "AA:BB:CC:DD:EE:FF",
	}, WithBLEDialer(func(ctx context.Context, network, addr string) (Socket, error) {
		assert.Equalf(t, "AA:BB:CC:DD:EE:FF", addr, "unexpected addr %q", addr)
		return mockBLEConn(), nil
	}))

	err := blet.Open()
	require.NoError(t, err)
	defer blet.Close()

	// Peer runs in a goroutine — net.Pipe is synchronous, so
	// Write blocks until the peer Reads.
	go func() {
		buf := make([]byte, 32)
		n, err := server.Read(buf)
		assert.NoError(t, err, "reading should not error")
		assert.Equalf(t, "ping", string(buf[:n]),
			"expected %q, got %q", "ping", buf[:n])
	}()

	if num, err := blet.Write([]byte("ping")); err != nil {
		assert.NoError(t, err, "sending bytes should not error")
		assert.NotEmpty(t, num)
	}
}

// TestBLETransport_Read tests reading bytes using an opened BLETransport.
func TestBLETransport_Read(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	blet := NewBLETransport(botfile.ConnectionConfig{
		Host: "AA:BB:CC:DD:EE:FF",
	}, WithBLEDialer(func(ctx context.Context, network, addr string) (Socket, error) {
		assert.Equalf(t, "AA:BB:CC:DD:EE:FF", addr, "unexpected addr %q", addr)
		return mockBLEConn(), nil
	}))

	err := blet.Open()
	require.NoError(t, err)
	defer blet.Close()

	go func() {
		server.Write([]byte("pong"))
		// keep the goroutine alive until the test reads, or close
	}()

	got := make([]byte, 4)
	num, err := blet.Read(got)
	assert.NoError(t, err, "reading should not error")
	assert.NotEmpty(t, num)
}
