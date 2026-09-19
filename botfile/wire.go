package botfile

import (
	"fmt"
	"strings"
	tmpl "text/template"
)

// WireConfig describes a Wire message format that may contain data-driven
// templates, e.g. format should be `{"N":106,"D1":{{.Direction}}}`.
type WireConfig struct {
	Format string `yaml:"format"`
	// Not exported.
	values any
}

// Message defines a Wire message template around a [tmpl.Template].
type Message struct {
	tpl *tmpl.Template
}

// NewMessage create a [Message] from a [WireConfig] object.
func NewMessage(cfg WireConfig) Message {
	return Message{
		tpl: tmpl.Must(tmpl.New("wire").Parse(cfg.Format)),
	}
}

// ToBytes returns the byte-level representation of the Wire message
// with the template arguments filled from args.
func (m Message) ToBytes(args any) ([]byte, error) {
	var b strings.Builder
	err := m.tpl.Execute(&b, args)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to format wire message: %w", err)
	}

	return []byte(b.String()), nil
}
