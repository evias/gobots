package botfile

import (
	"fmt"
	"os"
	"reflect"

	"github.com/goccy/go-yaml"
)

// Driver defines the contract for device drivers.
//
// A device driver typically defines connection details and executable commands
// for a particular device, e.g. a smartcar robot provides commands "move"/"turn".
//
// Communication with devices is ruled by a pre-configured message layer named
// the Wire protocol. Wire defines JSON-formatted data-driven templates such that
// parameters may be injected at the time of sending messages.
type Driver interface {
	// Name should return the device driver's name.
	Name() string

	// Host should return a string-representation of the device hostname.
	Host() string
	// Port should return a port number, used for connection to the device.
	Port() uint16

	// Config should return a YAML-mapped [DriverConfig] instance.
	Config() DriverConfig

	// HasCommand should return true given an existing command name.
	HasCommand(command string) bool
	// HasField should return true given an existing field name.
	HasField(field string) bool
	// FieldType should return the type of a field by name,
	// e.g. "string", "number" or "duration".
	FieldType(name string) string
	// FieldDefault should return the field's default value as a [reflect.Value].
	FieldDefault(name string) reflect.Value

	// CommandConfig should return a [CommandConfig] for a command name.
	CommandConfig(command string) CommandConfig
	// WireConfig should return a [WireConfig] for command, with values args.
	WireConfig(command string, args any) WireConfig
}

// DriverOption defines the contract for [Driver] option helpers.
type DriverOption func(Driver)

// ----------------------------------------------------------------------------

// robotDriver is a private implementation of the [Driver] interface.
type robotDriver struct {
	name string
	conf DriverConfig
}

// Ensure that our implementation satisfies interface.
var _ Driver = (*robotDriver)(nil)

// NewDriver creates a [Driver] instance around conf and options.
func NewDriver(
	conf DriverConfig,
	options ...DriverOption,
) Driver {
	drv := &robotDriver{
		name: conf.Name,
		conf: conf,
	}

	for _, option := range options {
		option(drv)
	}

	return drv
}

// WithHostAndPort implements an option helper to inject a custom host and port.
func WithHostAndPort(host string, port uint16) DriverOption {
	return func(d Driver) {
		rd := d.(*robotDriver)
		rd.conf.Connection.Host = host
		rd.conf.Connection.Port = port
	}
}

// WithMaxAttempts implements an option helper to inject a max number of attempts to connect.
func WithMaxAttempts(max uint16) DriverOption {
	return func(d Driver) {
		rd := d.(*robotDriver)
		rd.conf.Connection.MaxAttempts = max
	}
}

// Name returns the name a read from [DriverConfig] upon creation.
func (drv *robotDriver) Name() string {
	return drv.name
}

// Host returns the driver's connection hostname, e.g. "192.168.4.1"
func (drv *robotDriver) Host() string {
	return drv.conf.Connection.Host
}

// Port returns the driver's connection port number.
func (drv *robotDriver) Port() uint16 {
	return drv.conf.Connection.Port
}

// Config returns the injected [DriverConfig].
func (drv *robotDriver) Config() DriverConfig {
	return drv.conf
}

// HasCommands reads the [DriverConfig] to find a [CommandConfig] by name.
func (drv *robotDriver) HasCommand(command string) bool {
	_, ok := drv.conf.Commands[command]
	return ok
}

// HasField reads the [DriverConfig] to find a [FieldConfig] by name.
func (drv *robotDriver) HasField(field string) bool {
	_, ok := drv.conf.Fields[field]
	return ok
}

// FieldType returns a string-representation of the type of a field,
// importantly this defines the reflection process for default values.
func (drv *robotDriver) FieldType(name string) string {
	defaultType := "string"
	if !drv.HasField(name) {
		return defaultType
	}

	field := drv.conf.Fields[name]
	return field.Type
}

// FieldDefault returns the field's default value as a [reflect.Value].
func (drv *robotDriver) FieldDefault(name string) reflect.Value {
	defaultValue := reflect.New(reflect.TypeOf([]byte{}))
	defaultValue.SetBytes([]byte{})
	if !drv.HasField(name) {
		return defaultValue
	}

	f := drv.conf.Fields[name]
	ft := drv.FieldType(name)

	switch {
	case ft == "binary":
		v := reflect.New(reflect.TypeOf([]byte{}))
		bz, ok := f.Default.([]byte)
		if !ok {
			v.SetBytes([]byte{})
			return v
		}
		v.SetBytes(bz)
		return v
	case ft == "string":
		v := reflect.New(reflect.TypeOf(string("")))
		str, ok := f.Default.(string)
		if !ok {
			v.SetString("")
			return v
		}
		v.SetString(str)
		return v
	case ft == "number":
		v := reflect.New(reflect.TypeOf(float64(0)))
		i, ok := f.Default.(float64)
		if !ok {
			v.SetFloat(0.0)
			return v
		}
		v.SetFloat(i)
		return v

		// XXX case ft == "duration" should recognize e.g. "5s".
	}

	// Fallback to empty slice of bytes.
	return defaultValue
}

// CommandConfig returns a [CommandConfig] object by name.
func (drv *robotDriver) CommandConfig(command string) CommandConfig {
	defaultCmd := CommandConfig{
		Fields: []string{},
		Params: ParamsConfig{},
		Wire: map[string]WireConfig{
			"string": WireConfig{Format: "{{{.Value}}}"},
		},
	}
	if !drv.HasCommand(command) {
		return defaultCmd
	}

	cmd := drv.conf.Commands[command]
	return cmd
}

// WireConfig returns a [WireConfig] object for command, with fields
// and parameters filled from args.
func (drv *robotDriver) WireConfig(command string, args any) WireConfig {
	defaultWire := WireConfig{Format: "{{{.Value}}}", values: args}
	if !drv.HasCommand(command) {
		return defaultWire
	}

	cmd := drv.conf.Commands[command]
	v := reflect.ValueOf(args)

	// Make sure we have all fields (required).
	for _, field := range cmd.Fields {
		a := v.FieldByName(field)
		t := reflect.TypeOf(a)

		// If we are missing a field, fill with default.
		if a == reflect.Zero(t) {
			a.Set(drv.FieldDefault(field))
		}
	}

	// Encode the content of params, i.e. "forward" becomes 1.
	for param, paramValues := range cmd.Params {
		if len(paramValues) == 0 {
			continue
		}

		a := v.FieldByName(string(param))

		for pvn, pv := range paramValues {
			if a.String() == string(pvn) {
				a.Set(reflect.ValueOf(pv))
			}
		}
	}

	wireTypes := []string{
		"binary",
		"string",
		"number",
	}

	for _, wt := range wireTypes {
		if wire, isType := cmd.Wire[wt]; isType {
			return wire
		}
	}

	return defaultWire
}

// ----------------------------------------------------------------------------

// LoadFromConfig loads a [Driver] instance from a YAML configuration file.
// An error is returned if the file does not exist, or if it cannot be read,
// or if the format does not satisfy the unmarshalling to [DriverConfig].
//
// TODO(evias): Should accept options DriverOption if any available.
func LoadFromConfig(
	cfgFile string,
) (Driver, error) {
	if _, err := os.Stat(cfgFile); err != nil {
		return nil, fmt.Errorf("gobot driver file not found: %w", err)
	}

	bz, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read gobot driver: %w", err)
	}

	conf := DriverConfig{}
	if err := yaml.Unmarshal(bz, &conf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal driver config: %w", err)
	}

	return NewDriver(conf), nil
}
