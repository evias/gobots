package botfile

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testDriverConfig provides a [DriverConfig] fixture for tests.
func testDriverConfig() DriverConfig {
	return DriverConfig{
		Name:  "Test Device",
		Brand: "Test Brand",
		Connection: ConnectionConfig{
			Type:        "tcp",
			Host:        "192.168.4.1",
			Port:        1234,
			MaxAttempts: 2,
			TimeoutMs:   3000,
		},
		Fields: map[string]FieldConfig{
			"speed":     {Type: "number", Default: float64(50)},
			"direction": {Type: "string", Default: "forward"},
			"payload":   {Type: "binary"},
			"duration":  {Type: "duration"},
			"broken":    {Type: "string", Default: 42},
		},
		Commands: map[string]CommandConfig{
			"move": {
				Fields: []string{"Speed", "Direction"},
				Wire: map[string]WireConfig{
					"string": {Format: `{"N":102,"D1":{{.Speed}},"D2":"{{.Direction}}"}`},
				},
			},
			"heartbeat": {
				Wire: map[string]WireConfig{
					"string": {Format: "{Heartbeat}"},
				},
			},
			"binary_cmd": {
				Wire: map[string]WireConfig{
					"binary": {Format: `\x01\x02{{.Speed}}`},
					"string": {Format: `{"Speed":{{.Speed}}}`},
				},
			},
		},
	}
}

// TestNewDriver tests construction and getters.
func TestNewDriver(t *testing.T) {
	conf := testDriverConfig()

	drv := NewDriver(conf)

	require.NotNil(t, drv)
	assert.Equal(t, "Test Device", drv.Name())
	assert.Equal(t, "192.168.4.1", drv.Host())
	assert.Equal(t, uint16(1234), drv.Port())
	assert.Equal(t, conf, drv.Config())
}

// TestDriverOptions tests the [DriverOption] helpers.
func TestDriverOptions(t *testing.T) {
	drv := NewDriver(testDriverConfig(),
		WithHostAndPort("localhost", 9999),
		WithMaxAttempts(7),
		WithTimeoutMs(5000),
	)

	assert.Equal(t, "localhost", drv.Host())
	assert.Equal(t, uint16(9999), drv.Port())
	assert.Equal(t, uint16(7), drv.Config().Connection.MaxAttempts)
	assert.Equal(t, uint32(5000), drv.Config().Connection.TimeoutMs)

	// Unrelated config fields should remain untouched.
	assert.Equal(t, "Test Device", drv.Name())
	assert.Equal(t, "tcp", drv.Config().Connection.Type)
}

// TestDriver_HasCommand tests command lookups.
func TestDriver_HasCommand(t *testing.T) {
	drv := NewDriver(testDriverConfig())

	assert.True(t, drv.HasCommand("move"))
	assert.True(t, drv.HasCommand("heartbeat"))
	assert.False(t, drv.HasCommand("missing"))
	assert.False(t, drv.HasCommand(""))
}

// TestDriver_HasField tests field lookups.
func TestDriver_HasField(t *testing.T) {
	drv := NewDriver(testDriverConfig())

	assert.True(t, drv.HasField("speed"))
	assert.True(t, drv.HasField("payload"))
	assert.False(t, drv.HasField("missing"))
	assert.False(t, drv.HasField(""))
}

// TestDriver_FieldType tests field type lookups with the "string" default.
func TestDriver_FieldType(t *testing.T) {
	drv := NewDriver(testDriverConfig())

	assert.Equal(t, "number", drv.FieldType("speed"))
	assert.Equal(t, "string", drv.FieldType("direction"))
	assert.Equal(t, "binary", drv.FieldType("payload"))

	// Unknown fields default to "string".
	assert.Equal(t, "string", drv.FieldType("missing"))
}

// TestDriver_FieldDefault tests native-type defaults per field type.
func TestDriver_FieldDefault(t *testing.T) {
	testCases := []struct {
		name     string
		field    string
		expected any
	}{
		{name: "number_with_default", field: "speed", expected: float64(50)},
		{name: "string_with_default", field: "direction", expected: "forward"},
		{name: "binary", field: "payload", expected: []byte{}},
		{name: "duration_unimplemented", field: "duration", expected: []byte{}},
		{name: "string_type_mismatch", field: "broken", expected: ""},
		{name: "unknown_field", field: "missing", expected: []byte{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			drv := NewDriver(testDriverConfig())

			assert.Equalf(t, tc.expected, drv.FieldDefault(tc.field),
				"field %q: expected %v, got %v", tc.field, tc.expected, drv.FieldDefault(tc.field))
		})
	}
}

// TestDriver_FieldDefault_NumberMismatch tests the zero-value fallback when a
// "number" field holds a non-numeric default.
func TestDriver_FieldDefault_NumberMismatch(t *testing.T) {
	conf := testDriverConfig()
	conf.Fields["wrong"] = FieldConfig{Type: "number", Default: "not-a-number"}

	drv := NewDriver(conf)

	assert.Equal(t, float64(0), drv.FieldDefault("wrong"))
}

// TestDriver_FieldDefault_IntegerYAMLDefault documents that integer defaults
// loaded from YAML (unmarshalled as uint64 by go-yaml) do not satisfy the
// float64 type assertion and fall back to zero.
// TODO: "number" fields with integer YAML defaults silently resolve to 0.
func TestDriver_FieldDefault_IntegerYAMLDefault(t *testing.T) {
	conf := DriverConfig{
		Name: "Int Default Device",
		Fields: map[string]FieldConfig{
			"speed": {Type: "number", Default: 50},
		},
	}

	drv := NewDriver(conf)

	assert.Equal(t, "number", drv.FieldType("speed"))
	assert.Equalf(t, float64(0), drv.FieldDefault("speed"),
		"integer YAML default is expected to fall back to 0 (uint64 != float64)")
}

// TestDriver_FieldDefault_MissingNumber tests the zero-value fallback when a
// "number" field has no default.
func TestDriver_FieldDefault_MissingNumber(t *testing.T) {
	conf := testDriverConfig()
	conf.Fields["empty"] = FieldConfig{Type: "number"}

	drv := NewDriver(conf)

	assert.Equal(t, float64(0), drv.FieldDefault("empty"))
}

// TestDriver_CommandConfig tests config lookups and the fallback default.
func TestDriver_CommandConfig(t *testing.T) {
	drv := NewDriver(testDriverConfig())

	cmd := drv.CommandConfig("move")
	require.Contains(t, cmd.Wire, "string")
	assert.Equal(t, []string{"Speed", "Direction"}, cmd.Fields)

	// Unknown commands return a default wire format.
	fallback := drv.CommandConfig("missing")
	assert.Equal(t, "{{{.Value}}}", fallback.Wire["string"].Format)
	assert.NotNil(t, fallback.Fields)
	assert.NotNil(t, fallback.Params)
}

// TestDriver_WireConfig tests wire type resolution priority.
func TestDriver_WireConfig(t *testing.T) {
	drv := NewDriver(testDriverConfig())

	// Prefers "binary" over "string" when both are available.
	wire := drv.WireConfig("binary_cmd", map[string]any{"Speed": 10})
	assert.Equal(t, `\x01\x02{{.Speed}}`, wire.Format)

	// Returns the only available type otherwise.
	wire = drv.WireConfig("heartbeat", nil)
	assert.Equal(t, "{Heartbeat}", wire.Format)

	// Unknown commands fall back to the default format with values.
	args := map[string]any{"Value": "hello"}
	wire = drv.WireConfig("missing", args)
	assert.Equal(t, "{{{.Value}}}", wire.Format)
	assert.Equal(t, args, wire.values)
}

// TestLoadFromConfig tests loading a driver from a YAML file.
func TestLoadFromConfig(t *testing.T) {
	drv, err := LoadFromConfig(filepath.Join("..", "drivers", "robot.yaml"))

	require.NoError(t, err)
	assert.Equal(t, "Generic Edge Device", drv.Name())
	assert.Equal(t, "192.168.4.1", drv.Host())
	assert.Equal(t, uint16(1234), drv.Port())
	assert.True(t, drv.HasCommand("heartbeat"))
	assert.True(t, drv.HasCommand("stop"))
}

// TestLoadFromConfig_ComplexDriver tests unmarshalling of the full schema.
func TestLoadFromConfig_ComplexDriver(t *testing.T) {
	yamlDoc := `name: "Full Device"
brand: Full Brand
repository: "https://github.com/evias/gobots"
fields:
  speed:
    type: number
    default: 50.5
    min: 0
    max: 100
setup:
  - stop
shutdown:
  - stop
connection:
  type: serial
  host: "/dev/ttyUSB1"
  attempts: 3
  timeout: 2500
  heartbeat:
    command: heartbeat
    frequency: 1000ms
commands:
  move:
    fields:
      - Speed
      - Direction
    wire:
      string:
        format: '{"N":102,"D1":{{.Speed}},"D2":"{{.Direction}}"}'
`
	path := filepath.Join(t.TempDir(), "driver.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yamlDoc), 0o600))

	drv, err := LoadFromConfig(path)

	require.NoError(t, err)
	assert.Equal(t, "Full Device", drv.Name())
	assert.Equal(t, "/dev/ttyUSB1", drv.Host())
	assert.Equal(t, "serial", drv.Config().Connection.Type)
	assert.Equal(t, uint16(3), drv.Config().Connection.MaxAttempts)
	assert.Equal(t, uint32(2500), drv.Config().Connection.TimeoutMs)
	assert.Equal(t, time.Second, drv.Config().Connection.Heartbeat.Frequency)
	assert.Equal(t, []string{"stop"}, drv.Config().Setup)
	assert.Equal(t, []string{"stop"}, drv.Config().Shutdown)
	assert.Equal(t, float64(50.5), drv.FieldDefault("speed"))
	assert.Equal(t, "number", drv.FieldType("speed"))
	assert.True(t, drv.HasCommand("move"))
	assert.Equal(t, []string{"Speed", "Direction"}, drv.CommandConfig("move").Fields)
}

// TestLoadFromConfig_MissingFile tests the not-found error.
func TestLoadFromConfig_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")

	_, err := LoadFromConfig(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "gobot driver file not found")
}

// TestLoadFromConfig_MalformedYAML tests the unmarshalling error.
func TestLoadFromConfig_MalformedYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "driver.yaml")
	require.NoError(t, os.WriteFile(path, []byte("name: [unclosed"), 0o600))

	_, err := LoadFromConfig(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal driver config")
}

// TestLoadFromConfig_UnreadableFile tests the read error (skipped when
// running as root, which bypasses file permissions).
func TestLoadFromConfig_UnreadableFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, file permissions are not enforced")
	}

	path := filepath.Join(t.TempDir(), "driver.yaml")
	require.NoError(t, os.WriteFile(path, []byte("name: test"), 0o000))

	_, err := LoadFromConfig(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read gobot driver")
}
