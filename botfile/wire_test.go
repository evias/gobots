package botfile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewMessage tests rendering a Wire message template with arguments.
func TestNewMessage(t *testing.T) {
	args := struct {
		Speed     int
		Direction string
	}{Speed: 10, Direction: "forward"}

	msg, err := NewMessage(WireConfig{
		Format: `{"N":102,"D1":{{.Speed}},"D2":"{{.Direction}}"}`,
	}, args)

	require.NoError(t, err)
	require.NotNil(t, msg)

	expected := `{"N":102,"D1":10,"D2":"forward"}`
	assert.Equalf(t, expected, msg.String(),
		"expected %q, got %q", expected, msg.String())
}

// TestNewMessage_WithoutArgs tests rendering a template without arguments.
func TestNewMessage_WithoutArgs(t *testing.T) {
	msg, err := NewMessage(WireConfig{Format: "{Heartbeat}"}, nil)

	require.NoError(t, err)
	assert.Equal(t, "{Heartbeat}", msg.String())
}

// TestNewMessage_RenderError tests that template execution errors are wrapped.
func TestNewMessage_RenderError(t *testing.T) {
	// .Missing does not exist on the args struct, Execute should fail.
	_, err := NewMessage(WireConfig{Format: `{{.Missing}}`}, struct{}{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "msg: render:")
}

// TestMessage_MarshalBinary tests the size-prefixed wire format layout:
// [uint32 big-endian length][payload].
func TestMessage_MarshalBinary(t *testing.T) {
	msg := &Message{Payload: []byte("{Stop}")}

	b, err := msg.MarshalBinary()

	require.NoError(t, err)
	require.Len(t, b, 4+len(msg.Payload))

	n := binary.BigEndian.Uint32(b[:4])
	assert.Equalf(t, uint32(len(msg.Payload)), n,
		"expected size prefix %d, got %d", len(msg.Payload), n)
	assert.Equal(t, "{Stop}", string(b[4:]))
}

// TestMessage_MarshalBinary_EmptyPayload tests marshaling a nil/empty payload.
func TestMessage_MarshalBinary_EmptyPayload(t *testing.T) {
	msg := &Message{}

	b, err := msg.MarshalBinary()

	require.NoError(t, err)
	assert.Equal(t, []byte{0, 0, 0, 0}, b)
}

// TestMessage_MarshalBinary_TooLarge tests the MaxPayload sanity cap.
func TestMessage_MarshalBinary_TooLarge(t *testing.T) {
	msg := &Message{Payload: make([]byte, MaxPayload+1)}

	_, err := msg.MarshalBinary()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "msg: payload too large")
}

// TestMessage_MarshalBinary_MaxPayloadBoundary tests that a payload of exactly
// MaxPayload bytes is accepted.
func TestMessage_MarshalBinary_MaxPayloadBoundary(t *testing.T) {
	msg := &Message{Payload: make([]byte, MaxPayload)}

	b, err := msg.MarshalBinary()

	require.NoError(t, err)
	assert.Len(t, b, 4+MaxPayload)
}

// TestMessage_UnmarshalBinary tests decoding a size-prefixed payload.
func TestMessage_UnmarshalBinary(t *testing.T) {
	var m Message

	err := m.UnmarshalBinary([]byte{0, 0, 0, 6, '{', 'S', 't', 'o', 'p', '}'})

	require.NoError(t, err)
	assert.Equal(t, "{Stop}", string(m.Payload))
}

// TestMessage_UnmarshalBinary_TooShort tests that buffers shorter than the
// 4-byte size prefix yield [ErrShort].
func TestMessage_UnmarshalBinary_TooShort(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{name: "empty", data: []byte{}},
		{name: "three_bytes", data: []byte{0, 0, 0}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var m Message

			err := m.UnmarshalBinary(tc.data)

			assert.ErrorIs(t, err, ErrShort)
		})
	}
}

// TestMessage_UnmarshalBinary_TruncatedPayload tests that a declared length
// exceeding the available bytes yields [ErrShort].
func TestMessage_UnmarshalBinary_TruncatedPayload(t *testing.T) {
	var m Message

	// Declares 10 bytes but only provides 4.
	err := m.UnmarshalBinary([]byte{0, 0, 0, 10, '{', 'S', 't', 'o'})

	assert.ErrorIs(t, err, ErrShort)
}

// TestMessage_UnmarshalBinary_DeclaredTooLarge tests the MaxPayload enforcement
// on the declared length.
func TestMessage_UnmarshalBinary_DeclaredTooLarge(t *testing.T) {
	var m Message

	// Declares MaxPayload+1 bytes; header only, must error without allocating.
	n := uint32(MaxPayload + 1)
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, n)

	err := m.UnmarshalBinary(data)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "msg: declared length")
}

// TestMessage_UnmarshalBinary_Copy tests that the payload is copied and does
// not alias the caller's buffer.
func TestMessage_UnmarshalBinary_Copy(t *testing.T) {
	data := []byte{0, 0, 0, 2, 'a', 'b'}
	var m Message

	require.NoError(t, m.UnmarshalBinary(data))

	data[4] = 'x' // mutate the source buffer
	assert.Equal(t, []byte("ab"), m.Payload)
}

// TestMessage_RoundTrip tests MarshalBinary followed by UnmarshalBinary.
func TestMessage_RoundTrip(t *testing.T) {
	original := &Message{Payload: []byte(`{"N":102,"D1":10}`)}

	b, err := original.MarshalBinary()
	require.NoError(t, err)

	decoded, err := LoadFromBytes(b)
	require.NoError(t, err)

	assert.Equal(t, original.Payload, decoded.Payload)
	assert.Equal(t, original.String(), decoded.String())
}

// TestLoadFromBytes tests the [LoadFromBytes] constructor.
func TestLoadFromBytes(t *testing.T) {
	msg, err := LoadFromBytes([]byte{0, 0, 0, 1, 'x'})

	require.NoError(t, err)
	assert.Equal(t, []byte("x"), msg.Payload)

	_, err = LoadFromBytes([]byte{0, 0})
	assert.ErrorIs(t, err, ErrShort)
}

// TestMessage_ToBytes tests that ToBytes matches MarshalBinary.
func TestMessage_ToBytes(t *testing.T) {
	msg := &Message{Payload: []byte("{Stop}")}

	expected, err := msg.MarshalBinary()
	require.NoError(t, err)

	b, err := msg.ToBytes()

	require.NoError(t, err)
	assert.Equal(t, expected, b)
}

// TestMessage_WriteTo tests writing the size-prefixed message to a writer.
func TestMessage_WriteTo(t *testing.T) {
	msg := &Message{Payload: []byte("ping")}
	var buf bytes.Buffer

	n, err := msg.WriteTo(&buf)

	require.NoError(t, err)
	assert.Equal(t, int64(len(buf.Bytes())), n)
	assert.Equal(t, append([]byte{0, 0, 0, 4}, []byte("ping")...), buf.Bytes())
}

// TestMessage_WriteTo_TooLarge tests that WriteTo propagates the MaxPayload
// error without writing anything.
func TestMessage_WriteTo_TooLarge(t *testing.T) {
	msg := &Message{Payload: make([]byte, MaxPayload+1)}
	var buf bytes.Buffer

	n, err := msg.WriteTo(&buf)

	assert.Error(t, err)
	assert.Equal(t, int64(0), n)
	assert.Empty(t, buf.Bytes())
}

// TestMessage_WriteTo_WriteError tests that writer errors are propagated.
func TestMessage_WriteTo_WriteError(t *testing.T) {
	msg := &Message{Payload: []byte("ping")}

	_, err := msg.WriteTo(errWriter{})

	assert.Error(t, err)
}

// errWriter is a stub [io.Writer] which always fails.
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

// TestMessage_ReadFrom tests reading a size-prefixed message from a reader.
func TestMessage_ReadFrom(t *testing.T) {
	r := bytes.NewReader([]byte{0, 0, 0, 4, 'p', 'o', 'n', 'g'})
	var m Message

	n, err := m.ReadFrom(r)

	require.NoError(t, err)
	assert.Equal(t, int64(8), n)
	assert.Equal(t, "pong", string(m.Payload))
}

// TestMessage_ReadFrom_ShortHeader tests that a reader ending before the
// 4-byte header returns an error.
func TestMessage_ReadFrom_ShortHeader(t *testing.T) {
	r := bytes.NewReader([]byte{0, 0})
	var m Message

	n, err := m.ReadFrom(r)

	assert.Error(t, err)
	assert.Equal(t, int64(0), n)
}

// TestMessage_ReadFrom_ShortPayload tests that a reader ending before the
// declared payload returns an error.
func TestMessage_ReadFrom_ShortPayload(t *testing.T) {
	// Declares 10 bytes but only provides 4.
	r := bytes.NewReader([]byte{0, 0, 0, 10, 'p', 'o', 'n', 'g'})
	var m Message

	n, err := m.ReadFrom(r)

	assert.Error(t, err)
	assert.Equal(t, int64(8), n) // 4 header + 4 read payload bytes
}

// TestMessage_ReadFrom_DeclaredTooLarge tests the MaxPayload enforcement while
// reading from a reader.
func TestMessage_ReadFrom_DeclaredTooLarge(t *testing.T) {
	data := make([]byte, 4)
	binary.BigEndian.PutUint32(data, uint32(MaxPayload+1))
	r := bytes.NewReader(data)
	var m Message

	n, err := m.ReadFrom(r)

	assert.Error(t, err)
	assert.Equal(t, int64(4), n) // header was consumed
}

// TestMessage_ReadFrom_StreamingChunks tests that ReadFrom tolerates payload
// bytes received in multiple reads (io.ReadFull semantics).
func TestMessage_ReadFrom_StreamingChunks(t *testing.T) {
	r := io.MultiReader(
		bytes.NewReader([]byte{0, 0, 0, 3}),
		bytes.NewReader([]byte{'a'}),
		bytes.NewReader([]byte{'b', 'c'}),
	)
	var m Message

	n, err := m.ReadFrom(r)

	require.NoError(t, err)
	assert.Equal(t, int64(7), n)
	assert.Equal(t, "abc", string(m.Payload))
}

// TestMaxPayload tests the sanity cap constant value.
func TestMaxPayload(t *testing.T) {
	assert.Equal(t, 1<<20, MaxPayload)
	assert.True(t, strings.Contains(ErrShort.Error(), "buffer too short"))
}
