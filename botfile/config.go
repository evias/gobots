package botfile

import "time"

// XXX
type DriverConfig struct {
	Name       string                   `yaml:name`
	Repository string                   `yaml:repository`
	Fields     map[string]FieldConfig   `yaml:fields`
	Setup      []string                 `yaml:setup`
	Shutdown   []string                 `yaml:shutdown`
	Connection ConnectionConfig         `yaml:connection`
	Commands   map[string]CommandConfig `yaml:commands`
}

// XXX
type FieldConfig struct {
	Type    string `yaml:type`
	Default any    `yaml:default`
}

// XXX
type ConnectionConfig struct {
	Type      string           `yaml:type`
	Host      string           `yaml:host`
	Port      uint16           `yaml:port`
	Heartbeat CommandFrequency `yaml:heartbeat`
}

// XXX
type CommandFrequency struct {
	Command   string        `yaml:command`
	Frequency time.Duration `yaml:frequency`
}

// XXX
type CommandConfig struct {
	Fields []string              `yaml:fields`
	Params ParamsConfig          `yaml:params`
	Wire   map[string]WireConfig `yaml:wire`
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

// XXX
type WireConfig struct {
	Format string `yaml:format`
	// Not exported.
	values any
}
