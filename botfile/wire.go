package botfile

import (
	"fmt"
	"strings"
	tmpl "text/template"
)

// e.g. Format = '{"N":102,"D1":{{.Speed}},"D2":{{.Direction}}}\n'

// XXX
type Message struct {
	tpl *tmpl.Template
}

// XXX
func NewMessage(cfg WireConfig) Message {
	return Message{
		tpl: tmpl.Must(tmpl.New("wire").Parse(cfg.Format)),
	}
}

// XXX
func (m Message) ToBytes(args any) ([]byte, error) {
	var b strings.Builder
	err := m.tpl.Execute(&b, args)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to format wire message: %w", err)
	}

	return []byte(b.String()), nil
}
