package tui

import (
	"testing"

	"github.com/evias/gobots/botfile"
	"github.com/stretchr/testify/assert"
)

// TestUpperFirst tests unicode-aware first-rune capitalization.
func TestUpperFirst(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "lowercase", input: "speed", expected: "Speed"},
		{name: "already_upper", input: "Speed", expected: "Speed"},
		{name: "empty", input: "", expected: ""},
		{name: "single_char", input: "a", expected: "A"},
		{name: "multibyte_rune", input: "ünïcode", expected: "Ünïcode"},
		{name: "digit_prefix", input: "1abc", expected: "1abc"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equalf(t, tc.expected, upperFirst(tc.input),
				"expected %q, got %q", tc.expected, upperFirst(tc.input))
		})
	}
}

// TestSliceToMap tests bash-style argument parsing.
func TestSliceToMap(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		expected map[string]string
	}{
		{
			name:     "space_separated",
			args:     []string{"--speed", "10"},
			expected: map[string]string{"--speed": "10"},
		},
		{
			// TODO: the `--key=value` split only triggers when a
			// subsequent token starts with `--`, so a trailing
			// `--speed=10` keeps the full token as key with an
			// empty value.
			name:     "equals_sign_trailing",
			args:     []string{"--speed=10"},
			expected: map[string]string{"--speed=10": ""},
		},
		{
			name:     "flag_without_value",
			args:     []string{"--debug"},
			expected: map[string]string{"--debug": ""},
		},
		{
			name:     "separator_is_skipped",
			args:     []string{"--", "--speed", "10"},
			expected: map[string]string{"--speed": "10"},
		},
		{
			name:     "multiple_flags",
			args:     []string{"--speed", "10", "--direction", "forward"},
			expected: map[string]string{"--speed": "10", "--direction": "forward"},
		},
		{
			name:     "consecutive_flags",
			args:     []string{"--a", "--b", "5"},
			expected: map[string]string{"--a": "", "--b": "5"},
		},
		{
			name:     "equals_sign_before_flag_value",
			args:     []string{"--a=1", "--b"},
			expected: map[string]string{"--a": "1", "--b": ""},
		},
		{
			name:     "empty",
			args:     []string{},
			expected: map[string]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equalf(t, tc.expected, sliceToMap(tc.args),
				"expected %v, got %v", tc.expected, sliceToMap(tc.args))
		})
	}
}

// testParserDriver provides a [botfile.Driver] fixture with fields and params.
func testParserDriver() botfile.Driver {
	return botfile.NewDriver(botfile.DriverConfig{
		Name: "Parser Device",
		Commands: map[string]botfile.CommandConfig{
			"move": {
				Fields: []string{"Speed", "Direction"},
				Params: botfile.ParamsConfig{
					"direction": botfile.ParamConfig{
						"forward":  uint64(1),
						"backward": int(2),
					},
					"mode": botfile.ParamConfig{
						"turbo": "TURBO",
					},
				},
			},
			"empty": {},
		},
	})
}

// TestParseDataArgs tests data argument parsing for command fields and params.
func TestParseDataArgs(t *testing.T) {
	drv := testParserDriver()

	data := ParseDataArgs(drv, "move", []string{
		"--speed", "10", "--direction", "forward", "--mode", "turbo",
	})

	assert.Equal(t, map[string]string{
		"Speed":     "10",
		"Direction": "1", // "forward" resolves to uint64(1)
		"Mode":      "TURBO",
	}, data)
}

// TestParseDataArgs_ParamIntValue tests int-typed param value resolution.
func TestParseDataArgs_ParamIntValue(t *testing.T) {
	drv := testParserDriver()

	data := ParseDataArgs(drv, "move", []string{"--direction", "backward"})

	assert.Equal(t, "2", data["Direction"])
}

// TestParseDataArgs_MissingFields tests that required fields are present with
// an empty value even when not provided in args.
func TestParseDataArgs_MissingFields(t *testing.T) {
	drv := testParserDriver()

	data := ParseDataArgs(drv, "move", []string{})

	assert.Equal(t, map[string]string{
		"Speed":     "",
		"Direction": "",
	}, data)
}

// TestParseDataArgs_UnknownParams tests that absent or unknown params are
// ignored (params are optional).
func TestParseDataArgs_UnknownParams(t *testing.T) {
	drv := testParserDriver()

	// Unknown param value identifier: the raw value passes through.
	data := ParseDataArgs(drv, "move", []string{"--direction", "unknown"})
	assert.Equal(t, "unknown", data["Direction"])

	// Unknown param name: ignored.
	data = ParseDataArgs(drv, "move", []string{"--unknown", "value"})
	assert.NotContains(t, data, "Unknown")
}

// TestParseDataArgs_UnknownCommand tests parsing against a command which does
// not exist in the driver (no fields/params to resolve).
func TestParseDataArgs_UnknownCommand(t *testing.T) {
	drv := testParserDriver()

	data := ParseDataArgs(drv, "missing", []string{"--speed", "10"})

	assert.Empty(t, data)
}

// TestParseDataArgs_EmptyCommand tests parsing against a command without
// fields or params.
func TestParseDataArgs_EmptyCommand(t *testing.T) {
	drv := testParserDriver()

	data := ParseDataArgs(drv, "empty", []string{"--speed", "10"})

	assert.Empty(t, data)
}
