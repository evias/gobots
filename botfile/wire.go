package botfile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	tmpl "text/template"
)

const (
	MaxPayload = 1 << 20 // 1Mb sanity cap
)

// Wireable defines the contract for structure that may be transported or
// transmitted in the form of size-prefixed bytes payloads.
type Wireable interface {
	// ToBytes should encode the message to a bytes slice.
	ToBytes() ([]byte, error)

	// WriteTo should write the message to a [io.Writer].
	WriteTo(w io.Writer) (int64, error)
	// ReadFrom should write a message from a [io.Reader].
	ReadFrom(r io.Reader) (int64, error)
}

// WireConfig describes a Wire message format that may contain data-driven
// templates, e.g. format should be `{"N":106,"D1":{{.Direction}}}`.
type WireConfig struct {
	Format string `yaml:"format"`
	// Not exported.
	values any
}

// Message defines a Wire message template around a [tmpl.Template] which
// contains a **size-prefixed bytes slice** to represent the message.
type Message struct {
	Payload []byte
}

// Ensure that our implementation satisfies interface [Wireable].
var _ Wireable = (*Message)(nil)

// NewMessage creates a [Message] from a [WireConfig] object and args,
// and executes the [WireConfig#Format] template to replace arguments.
func NewMessage(cfg WireConfig, args any) (*Message, error) {
	var buf bytes.Buffer
	tpl := tmpl.Must(tmpl.New("wire").Parse(cfg.Format))
	if err := tpl.Execute(&buf, args); err != nil {
		return nil, fmt.Errorf("msg: render: %w", err)
	}

	return &Message{
		Payload: buf.Bytes(),
	}, nil
}

// ToBytes encodes the [Message#Payload] to a bytes slice.
// ToBytes implements Wireable
func (m *Message) ToBytes() ([]byte, error) { return m.MarshalBinary() }

// ----------------------------------------------------------------------------

var ErrShort = errors.New("msg: buffer too short")

// MarshalBinary encodes the message to a size-prefixed bytes slice or returns
// an error. It prepends the payload size to the payload bytes as a Uint32.
// Wire format: [uint32 big-endian length][payload]
// MarshalBinary implements encoding.BinaryMarshaler
func (m *Message) MarshalBinary() ([]byte, error) {
	if len(m.Payload) > MaxPayload {
		return nil, fmt.Errorf("msg: payload too large (%d)", len(m.Payload))
	}
	out := make([]byte, 4+len(m.Payload))
	binary.BigEndian.PutUint32(out, uint32(len(m.Payload)))
	copy(out[4:], m.Payload)
	return out, nil
}

// UnmarshalBinary decodes a size-prefixed bytes payload to a [Message#Payload]
// or returns an error. It expects at least 4 bytes (the size prefix as uint32),
// and it enforces a MaxPayload size of 1Mb (`1 << 20`).
// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (m *Message) UnmarshalBinary(b []byte) error {
	if len(b) < 4 {
		return ErrShort
	}
	n := binary.BigEndian.Uint32(b)
	if n > MaxPayload {
		return fmt.Errorf("msg: declared length %d exceeds max", n)
	}
	if uint32(len(b)-4) < n {
		return ErrShort
	}
	m.Payload = append([]byte(nil), b[4:4+n]...) // copy: don't alias caller's buffer
	return nil
}

// ----------------------------------------------------------------------------

// WriteTo may be used to write directly to socket connections or other transports.
// WriteTo implements io.WriterTo
func (m *Message) WriteTo(w io.Writer) (int64, error) {
	b, err := m.MarshalBinary()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(b)
	return int64(n), err
}

// ReadFrom may be used to read directly from socket connections or other transports.
// ReadFrom implements io.ReaderFrom
func (m *Message) ReadFrom(r io.Reader) (int64, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n > MaxPayload {
		return 4, fmt.Errorf("msg: declared length %d exceeds max", n)
	}
	m.Payload = make([]byte, n)
	read, err := io.ReadFull(r, m.Payload)
	return 4 + int64(read), err
}

// ----------------------------------------------------------------------------

// LoadFromBytes loads a [Message] instance from a bytes slice payload.
func LoadFromBytes(b []byte) (*Message, error) {
	var m Message
	if err := m.UnmarshalBinary(b); err != nil {
		return nil, err
	}
	return &m, nil
}
