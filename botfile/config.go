package botfile

import "time"

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
// TODO(evias): Some fields require "min", "max" fields, others TBI?
type FieldConfig struct {
	Type    string `yaml:"type"`
	Default any    `yaml:"default"`
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
	Fields []string              `yaml:"fields"`
	Params ParamsConfig          `yaml:"params"`
	Wire   map[string]WireConfig `yaml:"wire"`
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
