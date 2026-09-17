package conn

import (
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

	expectedType := "tcp"
	actualType := tcpt.Type()
	assert.Equalf(t, expectedType, actualType,
		"transport type expected %s, got %s", expectedType, actualType)

	expectedStr := "tcp://localhost:1234"
	actualStr := tcpt.String()
	assert.Equalf(t, expectedStr, actualStr,
		"transport URL expected %s, got %s", expectedStr, actualStr)
}
