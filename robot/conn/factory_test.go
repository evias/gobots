package conn

import (
	"testing"

	"github.com/evias/gobots/botfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewTransport tests transport dispatch by connection type.
func TestNewTransport(t *testing.T) {
	testCases := []struct {
		name     string
		confType string
		expected Transport
	}{
		{name: "tcp", confType: "tcp", expected: (*TCPTransport)(nil)},
		{name: "serial", confType: "serial", expected: (*SerialTransport)(nil)},
		{name: "ble", confType: "ble", expected: (*BLETransport)(nil)},
		{name: "bluetooth", confType: "bluetooth", expected: (*BLETransport)(nil)},
		{name: "unknown_defaults_to_tcp", confType: "udp", expected: (*TCPTransport)(nil)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			transport := NewTransport(botfile.ConnectionConfig{
				Type: tc.confType,
				Host: "localhost",
				Port: 1234,
			})

			require.NotNil(t, transport)
			assert.IsType(t, tc.expected, transport)

			// Transport interface sanity: type and address are available
			// without opening a connection.
			assert.NotEmpty(t, transport.Type())
			assert.NotEmpty(t, transport.String())
		})
	}
}

// TestNewTransport_WithSerialConfig tests dispatch including serial options.
func TestNewTransport_WithSerialConfig(t *testing.T) {
	conf := botfile.ConnectionConfig{
		Type: "serial",
		Host: "/dev/ttyUSB1",
	}

	transport := NewTransport(conf, WithSerialConfig(botfile.SerialConfig{
		BaudRate: 9600,
	}))

	require.NotNil(t, transport)
	serialt, ok := transport.(*SerialTransport)
	require.True(t, ok, "expected *SerialTransport")
	assert.Equal(t, 9600, serialt.Mode().BaudRate)
}

// TestNewTransport_WithBluetoothChannelConfig tests dispatch including BLE options.
func TestNewTransport_WithBluetoothChannelConfig(t *testing.T) {
	conf := botfile.ConnectionConfig{
		Type: "bluetooth",
		Host: "AA:BB:CC:DD:EE:FF",
	}

	transport := NewTransport(conf, WithBluetoothChannelConfig(botfile.BluetoothChannelConfig{
		ServiceUUID:        "0000ffe0-0000-1000-8000-00805f9b34fb",
		CharacteristicUUID: "0000ffe1-0000-1000-8000-00805f9b34fb",
	}))

	require.NotNil(t, transport)
	assert.IsType(t, (*BLETransport)(nil), transport)
}
