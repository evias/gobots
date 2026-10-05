package tui

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/evias/gobots/botfile"
)

// -----------------------------------------------------------------------------
// Parser — parses custom data arguments passed to gobots driver commands.
// -----------------------------------------------------------------------------

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	r, i := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[i:]
}

// sliceToMap converts a slice of bash-style arguments, e.g. "--speed", into a
// key-value map with string keys and values.
func sliceToMap(args []string) map[string]string {
	m := make(map[string]string)
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			// detects --, but nothing to do
			continue
		}
		if strings.HasPrefix(args[i], "--") {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				m[args[i]] = args[i+1]
				i++ // consume the value
			} else if i+1 < len(args) && strings.Contains(args[i], "=") {
				nv := strings.SplitN(args[i], "=", 2)
				m[nv[0]] = nv[1]
			} else {
				m[args[i]] = "" // flag with no value (or boolean-style flag)
			}
		}
	}
	return m
}

// ParseDataArgs parses data arguments, typically passed through os.Args in a
// format similar to e.g. `--speed 10` or `--direction=forward`.
//
// Returns a map[string]string with keys from [botfile.CommandConfig#Fields]
// and [botfile.CommandConfig#Params].
// Returns a map with keys upper-first, e.g. "Direction", or "Speed".
func ParseDataArgs(
	driver botfile.Driver,
	command string,
	args []string,
) (data map[string]string) {
	wireCommand := driver.Config().Commands[command]

	// v contains "--sleep", "--direction" keys.
	v := sliceToMap(args)

	// data contains "sleep", "direction" keys.
	data = make(map[string]string, len(args))

	// Make sure we have all fields (required).
	for _, field := range wireCommand.Fields {
		f := "--" + strings.ToLower(field)
		a, ok := v[f]

		// Golang text/template expects uppercase-first keys.
		key := upperFirst(field)

		data[key] = ""
		if ok {
			data[key] = a
		}
	}

	// Encode the content of params, i.e. "forward" becomes 1.
	// Empty/Non-present parameters are ignored (optional).
	for param, paramValues := range wireCommand.Params {
		if len(paramValues) == 0 {
			continue
		}

		p := "--" + strings.ToLower(string(param))
		a, ok := v[string(p)]
		if !ok {
			continue
		}

		// Golang text/template expects uppercase-first keys.
		key := upperFirst(string(param))

		for pvn, pv := range paramValues {
			if a != string(pvn) {
				continue
			}

			if v, ok := pv.(uint64); ok {
				data[key] = strconv.FormatUint(v, 10)
				break
			} else if v, ok := pv.(int); ok {
				data[key] = strconv.Itoa(v)
				break
			} else if v, ok := pv.(string); ok {
				data[key] = v
				break
			}
		}
	}

	return /* data */
}
