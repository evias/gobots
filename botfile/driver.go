package botfile

import (
	"fmt"
	"os"
	"reflect"

	"github.com/goccy/go-yaml"
)

// XXX
type Driver interface {
	Name() string
	Host() string
	Port() uint16
	Config() DriverConfig

	HasCommand(string) bool
	WireConfig(string, any) WireConfig
}

// XXX
type robotDriver struct {
	name string
	conf DriverConfig
}

// XXX
type DriverOption func(*robotDriver)

// Ensure that our implementation satisfies interface.
var _ Driver = (*robotDriver)(nil)

// XXX
func NewDriver(
	conf DriverConfig,
	options ...DriverOption,
) *robotDriver {
	drv := &robotDriver{
		name: conf.Name,
		conf: conf,
	}

	for _, option := range options {
		option(drv)
	}

	return drv
}

// XXX
func (drv *robotDriver) Name() string {
	return drv.name
}

// XXX
func (drv *robotDriver) Host() string {
	return drv.conf.Connection.Host
}

// XXX
func (drv *robotDriver) Port() uint16 {
	return drv.conf.Connection.Port
}

// XXX
func (drv *robotDriver) Config() DriverConfig {
	return drv.conf
}

// XXX
func (drv *robotDriver) HasCommand(command string) bool {
	_, ok := drv.conf.Commands[command]
	return ok
}

// XXX
func (drv *robotDriver) HasField(field string) bool {
	_, ok := drv.conf.Fields[field]
	return ok
}

// XXX
func (drv *robotDriver) FieldType(name string) string {
	defaultType := "string"
	if !drv.HasField(name) {
		return defaultType
	}

	field := drv.conf.Fields[name]
	return field.Type
}

// XXX
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
	}

	// Fallback to empty slice of bytes.
	return defaultValue
}

// XXX
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

// XXX
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

// XXX
func LoadFromConfig(
	cfgFile string,
) (*robotDriver, error) {
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
