package botfile

import (
	"fmt"
	"time"
)

// DriverConfig describes a device communication driver. A driver defines the
// configuration fields, the setup commands, the shutdown process, connection
// details and executable commands available for a particular device.
//
// YAML configuration files must contain: name, brand, connection details.
// YAML configuration files may also contain a repository, required fields,
// setup commands, a shutdown process and executable commands.
//
// TODO(evias): Connection field may need to allow multiple values.
type DriverConfig struct {
	Name       string                   `yaml:"name"`
	Brand      string                   `yaml:"brand"`
	Repository string                   `yaml:"repository"`
	Fields     map[string]FieldConfig   `yaml:"fields"`
	Setup      []string                 `yaml:"setup"`
	Shutdown   []string                 `yaml:"shutdown"`
	Connection ConnectionConfig         `yaml:"connection"`
	Commands   map[string]CommandConfig `yaml:"commands"`
}

// FieldConfig describes configuration fields that may be required for the
// execution of commands, e.g. a "speed" field for the "move" command.
//
// Min/Max fields use a pointer to permit distinguishing between 0-value
// and nil (not present), notably during marshaling/unmarshaling.
type FieldConfig struct {
	Type    string   `yaml:"type"`
	Default any      `yaml:"default"`
	Min     *float64 `yaml:"min,omitempty"`
	Max     *float64 `yaml:"max,omitempty"`
}

// ConnectionConfig describes connection options for a particular device.
// Required fields include: Type, Host and Port.
// The Heartbeat command is optional and when set, will be repeated as
// specified with [CommandFrequency].
type ConnectionConfig struct {
	Type        string           `yaml:"type"`
	Host        string           `yaml:"host"`
	Port        uint16           `yaml:"port"`
	Heartbeat   CommandFrequency `yaml:"heartbeat"`
	MaxAttempts uint16           `yaml:"attempts"`
	TimeoutMs   uint32           `yaml:"timeout"`
}

// TODO(evias): Max attempt is 0 when unspecified, extract DefaultMaxAttempt from robot.conn.
func (c ConnectionConfig) String() string {
	timeoutMsStr := (time.Duration(c.TimeoutMs) * time.Millisecond).String()
	return fmt.Sprintf("%s (try: %d, timeout: %s)",
		c.Address(), c.MaxAttempts, timeoutMsStr)
}

// Address returns a formatted address from the connection configuration.
func (c ConnectionConfig) Address() string {
	switch {
	case c.Type == "serial":
		return fmt.Sprintf("serial:%s", c.Host)

	case c.Type == "ble":
	case c.Type == "bluetooth":
		return fmt.Sprintf("bluetooth:%s", c.Host)

	default:
	}

	// tcp, udp, http, (https, ftp?)
	return fmt.Sprintf("%s://%s:%d", c.Type, c.Host, c.Port)
}

// SerialConfig describes connection options for a serial port connection.
type SerialConfig struct {
	BaudRate uint32 `yaml:"baud_rate"`
	Parity   uint8  `yaml:"parity"`
	DataBits uint8  `yaml:"data_bits"`
	StopBits uint8  `yaml:"stop_bits"`
}

// BluetoothChannelConfig describes a service/characteristic pair to represent
// a bluetooth messaging channel.
type BluetoothChannelConfig struct {
	ServiceUUID        string `yaml:"service"`
	CharacteristicUUID string `yaml:"characteristic"`
}

// CommandFrequency describe the interval between each Command execution.
// The frequency field contains a [time.Duration]
type CommandFrequency struct {
	Command   string        `yaml:"command"`
	Frequency time.Duration `yaml:"frequency"`
}

// CommandConfig describes an executable command around required fields,
// a parameters map and a [WireConfig] paired to a string-representation
// of the message type, i.e. "string" or "binary".
type CommandConfig struct {
	Fields   []string              `yaml:"fields"`
	Params   ParamsConfig          `yaml:"params"`
	Wire     map[string]WireConfig `yaml:"wire"`
	Shutdown []string              `yaml:"shutdown"`
}

// ParamName describes a parameter name, e.g. "direction".
type ParamName string

// ParamValueName describes a parameter value identifier, e.g. "forward".
type ParamValueName string

// ParamValue describes an individual value of a parameter, e.g. 1 or "forward".
type ParamValue any

// ParamConfig maps value identifiers to actual values, e.g. {"forward": 1}.
type ParamConfig map[ParamValueName]ParamValue

// ParamsConfig maps parameter names with their respective values.
type ParamsConfig map[ParamName]ParamConfig
