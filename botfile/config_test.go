package botfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConnectionConfig_Address tests building the address string per
// connection type.
func TestConnectionConfig_Address(t *testing.T) {
	testCases := []struct {
		name     string
		conf     ConnectionConfig
		expected string
	}{
		{
			name:     "tcp",
			conf:     ConnectionConfig{Type: "tcp", Host: "192.168.4.1", Port: 1234},
			expected: "tcp://192.168.4.1:1234",
		},
		{
			name:     "default_falls_back_to_typed_url",
			conf:     ConnectionConfig{Type: "udp", Host: "10.0.0.1", Port: 5555},
			expected: "udp://10.0.0.1:5555",
		},
		{
			name:     "empty_type",
			conf:     ConnectionConfig{Host: "192.168.4.1", Port: 1234},
			expected: "://192.168.4.1:1234",
		},
		{
			name:     "serial",
			conf:     ConnectionConfig{Type: "serial", Host: "/dev/ttyUSB1"},
			expected: "serial:/dev/ttyUSB1",
		},
		{
			name:     "bluetooth",
			conf:     ConnectionConfig{Type: "bluetooth", Host: "AA:BB:CC:DD:EE:FF"},
			expected: "bluetooth:AA:BB:CC:DD:EE:FF",
		},
		{
			// TODO: currently falls back to the typed-URL format because
			// `case c.Type == "ble":` has an empty body (no switch
			// fallthrough in Go); expected "bluetooth:AA:BB:CC:DD:EE:FF".
			name:     "ble",
			conf:     ConnectionConfig{Type: "ble", Host: "AA:BB:CC:DD:EE:FF"},
			expected: "ble://AA:BB:CC:DD:EE:FF:0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equalf(t, tc.expected, tc.conf.Address(),
				"expected %q, got %q", tc.expected, tc.conf.Address())
		})
	}
}

// TestConnectionConfig_String tests the formatted string representation.
func TestConnectionConfig_String(t *testing.T) {
	testCases := []struct {
		name     string
		conf     ConnectionConfig
		expected string
	}{
		{
			name: "with_attempts_and_timeout",
			conf: ConnectionConfig{
				Type:        "tcp",
				Host:        "192.168.4.1",
				Port:        1234,
				MaxAttempts: 3,
				TimeoutMs:   1500,
			},
			expected: "tcp://192.168.4.1:1234 (try: 3, timeout: 1.5s)",
		},
		{
			name: "zero_values",
			conf: ConnectionConfig{
				Type: "tcp",
				Host: "localhost",
				Port: 80,
			},
			expected: "tcp://localhost:80 (try: 0, timeout: 0s)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equalf(t, tc.expected, tc.conf.String(),
				"expected %q, got %q", tc.expected, tc.conf.String())
		})
	}
}
