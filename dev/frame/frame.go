// Package frame is the dev-mode wire between the device proxy, the relay in
// `nimbus native dev` and the app process on the laptop: typed,
// length-prefixed frames over TCP.
package frame

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Type says what a frame carries.
type Type uint8

const (
	Hello     Type = 1 // role byte + schema hash (u64 LE): who connected
	ToGo      Type = 2 // host → Go async message (device → app)
	FromGo    Type = 3 // Go → host async message (app → device)
	Call      Type = 4 // host → Go sync call: id u32 + request
	CallReply Type = 5 // id u32 + rc u8 + reply
	HostCall  Type = 6 // Go → host sync call: id u32 + request
	HostReply Type = 7 // id u32 + rc u8 + reply
	Reset     Type = 8 // relay → device: a new app process is taking over
)

// Roles in a Hello frame.
const (
	RoleDevice byte = 1
	RoleApp    byte = 2
)

// maxFrame bounds a frame so a corrupt length can't exhaust memory.
const maxFrame = 64 << 20

// Write sends one frame. Callers serialise writes to a connection.
func Write(w io.Writer, t Type, payload []byte) error {
	var h [5]byte
	h[0] = byte(t)
	binary.LittleEndian.PutUint32(h[1:], uint32(len(payload)))
	if _, err := w.Write(h[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

// Read receives one frame.
func Read(r io.Reader) (Type, []byte, error) {
	var h [5]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return 0, nil, err
	}
	n := binary.LittleEndian.Uint32(h[1:])
	if n > maxFrame {
		return 0, nil, fmt.Errorf("frame: %d bytes is too large", n)
	}
	p := make([]byte, n)
	if _, err := io.ReadFull(r, p); err != nil {
		return 0, nil, err
	}
	return Type(h[0]), p, nil
}

// HelloPayload is a Hello frame's payload.
func HelloPayload(role byte, schema uint64) []byte {
	return binary.LittleEndian.AppendUint64([]byte{role}, schema)
}

// ParseHello reads a Hello payload.
func ParseHello(p []byte) (role byte, schema uint64, ok bool) {
	if len(p) != 9 {
		return 0, 0, false
	}
	return p[0], binary.LittleEndian.Uint64(p[1:]), true
}

// WithID prefixes a call or host call with its id.
func WithID(id uint32, b []byte) []byte {
	return append(binary.LittleEndian.AppendUint32(nil, id), b...)
}

// Reply is a call or host-call reply: id, result code (nn.h), bytes.
func Reply(id uint32, rc byte, b []byte) []byte {
	return append(append(binary.LittleEndian.AppendUint32(nil, id), rc), b...)
}

// SplitID splits an id-prefixed payload.
func SplitID(p []byte) (uint32, []byte, bool) {
	if len(p) < 4 {
		return 0, nil, false
	}
	return binary.LittleEndian.Uint32(p), p[4:], true
}

// SplitReply splits a reply payload.
func SplitReply(p []byte) (id uint32, rc byte, b []byte, ok bool) {
	if len(p) < 5 {
		return 0, 0, nil, false
	}
	return binary.LittleEndian.Uint32(p), p[4], p[5:], true
}
