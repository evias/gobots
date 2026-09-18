package conn

import (
	"context"
	"net"
	"testing"

	"github.com/evias/gobots/botfile"
	"github.com/stretchr/testify/assert"
)

// TestNewTCPTransport tests creating a TCPTransport
func TestNewTCPTransport(t *testing.T) {
	tcpt := NewTCPTransport(botfile.ConnectionConfig{
		Host: "localhost",
		Port: 1234,
	})

	assert.NotNil(t, tcpt.Type())
	assert.NotEmpty(t, tcpt.String())

	expectedType := "tcp"
	actualType := tcpt.Type()
	assert.Equalf(t, expectedType, actualType,
		"transport type expected %s, got %s", expectedType, actualType)
}

// TestTCPTransport_String tests the string representation of a TCPTransport instance.
func TestTCPTransport_String(t *testing.T) {
	tcpt := NewTCPTransport(botfile.ConnectionConfig{
		Host: "localhost",
		Port: 1234,
	})

	expectedStr := "localhost:1234"
	actualStr := tcpt.String()
	assert.Equalf(t, expectedStr, actualStr,
		"transport URL expected %s, got %s", expectedStr, actualStr)
}

// TestTCPTransport_OpenClose injects a custom dialer to test connecting/disconnecting.
func TestTCPTransport_OpenClose(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()

	tcpt := NewTCPTransport(botfile.ConnectionConfig{
		Host: "localhost",
		Port: 1234,
	}, WithDialer(func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr != "localhost:1234" {
			t.Fatalf("unexpected addr %q", addr)
		}
		return client, nil
	}))

	openErr := tcpt.Open()
	assert.NoError(t, openErr)
	assert.NotNil(t, tcpt.Conn(), "should set conn instance")

	closeErr := tcpt.Close()
	assert.NoError(t, closeErr)
}

func TestTCPTransport_Write(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	tcpt := NewTCPTransport(botfile.ConnectionConfig{
		Host: "localhost",
		Port: 1234,
	}, WithDialer(func(ctx context.Context, network, addr string) (net.Conn, error) {
		assert.Equalf(t, "localhost:1234", addr, "unexpected addr %q", addr)
		return client, nil
	}))

	// Peer runs in a goroutine — net.Pipe is synchronous, so
	// Write blocks until the peer Reads.
	go func() {
		buf := make([]byte, 32)
		n, err := server.Read(buf)
		assert.NoError(t, err, "reading should not error")
		assert.Equalf(t, "ping", string(buf[:n]),
			"got %q, want %q", buf[:n], "ping")
	}()

	if num, err := tcpt.Write([]byte("ping")); err != nil {
		assert.NoError(t, err, "sending bytes should not error")
		assert.NotEmpty(t, num)
	}
}
