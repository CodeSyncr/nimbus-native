// Package wire is the runtime for the Nimbus Native wire format: the binary
// encoding every message between Go and a host uses. Generated code (see
// wire/gen) calls these functions; apps rarely touch them directly.
//
// Encoding, identical in Go, Swift and Java:
//
//	bool                1 byte, 0 or 1
//	uint8…uint64        unsigned LEB128 varint
//	int8…int64          zigzag, then varint
//	float32, float64    IEEE 754, little-endian, 4 / 8 bytes
//	string, []byte      varint length, then the bytes (strings are UTF-8)
//	[]T                 varint count, then each element
//	map[K]V             varint count, then key, value pairs sorted by key
//	*T (optional)       1 byte present flag, then the value if present
//	struct              its fields in declaration order, no tags
//	union (interface)   varint variant index (1-based), then the variant
//
// Structs carry no field tags: both sides are generated from the same
// schema, and the schema hash checked at nn_start guarantees they match.
package wire

import (
	"encoding/binary"
	"errors"
	"math"
)

// Writer helpers append to b and return the extended slice.

func AppendBool(b []byte, v bool) []byte {
	if v {
		return append(b, 1)
	}
	return append(b, 0)
}

func AppendUint(b []byte, v uint64) []byte { return binary.AppendUvarint(b, v) }

func AppendInt(b []byte, v int64) []byte { return binary.AppendUvarint(b, uint64(v<<1)^uint64(v>>63)) }

func AppendFloat32(b []byte, v float32) []byte {
	return binary.LittleEndian.AppendUint32(b, math.Float32bits(v))
}

func AppendFloat64(b []byte, v float64) []byte {
	return binary.LittleEndian.AppendUint64(b, math.Float64bits(v))
}

func AppendString(b []byte, v string) []byte {
	b = binary.AppendUvarint(b, uint64(len(v)))
	return append(b, v...)
}

func AppendBytes(b []byte, v []byte) []byte {
	b = binary.AppendUvarint(b, uint64(len(v)))
	return append(b, v...)
}

// Errors a Reader reports. Decoding stops at the first one.
var (
	ErrShort    = errors.New("wire: message ends early")
	ErrOverflow = errors.New("wire: varint overflows 64 bits")
	ErrRange    = errors.New("wire: value out of range for its type")
	ErrVariant  = errors.New("wire: unknown union variant or message id")
	ErrTrailing = errors.New("wire: bytes left over after the message")
)

// maxPrealloc caps how many elements a decoder allocates up front from a
// count it read off the wire, so a corrupt count can't exhaust memory.
const maxPrealloc = 4096

// Reader decodes from a byte slice. The first error sticks: later reads
// return zero values, and Err reports what went wrong. Generated code reads
// a whole message and checks Err once.
type Reader struct {
	b   []byte
	err error
}

func NewReader(b []byte) *Reader { return &Reader{b: b} }

// Err is the first error encountered, or nil.
func (r *Reader) Err() error { return r.err }

// Fail records err unless an earlier error is already recorded.
func (r *Reader) Fail(err error) {
	if r.err == nil {
		r.err = err
	}
}

// Done reports ErrTrailing if bytes remain, and returns the reader's error.
func (r *Reader) Done() error {
	if r.err == nil && len(r.b) > 0 {
		r.err = ErrTrailing
	}
	return r.err
}

func (r *Reader) Bool() bool {
	if r.err != nil || len(r.b) < 1 {
		r.Fail(ErrShort)
		return false
	}
	v := r.b[0]
	r.b = r.b[1:]
	if v > 1 {
		r.Fail(ErrRange)
	}
	return v == 1
}

func (r *Reader) Uint() uint64 {
	if r.err != nil {
		return 0
	}
	v, n := binary.Uvarint(r.b)
	switch {
	case n == 0:
		r.Fail(ErrShort)
		return 0
	case n < 0:
		r.Fail(ErrOverflow)
		return 0
	}
	r.b = r.b[n:]
	return v
}

func (r *Reader) Int() int64 {
	u := r.Uint()
	return int64(u>>1) ^ -int64(u&1)
}

// UintMax reads an unsigned varint and fails with ErrRange above max — for
// uint8/16/32 fields.
func (r *Reader) UintMax(max uint64) uint64 {
	v := r.Uint()
	if v > max {
		r.Fail(ErrRange)
		return 0
	}
	return v
}

// IntRange reads a signed varint and fails with ErrRange outside [min, max].
func (r *Reader) IntRange(min, max int64) int64 {
	v := r.Int()
	if v < min || v > max {
		r.Fail(ErrRange)
		return 0
	}
	return v
}

func (r *Reader) Float32() float32 {
	if r.err != nil || len(r.b) < 4 {
		r.Fail(ErrShort)
		return 0
	}
	v := math.Float32frombits(binary.LittleEndian.Uint32(r.b))
	r.b = r.b[4:]
	return v
}

func (r *Reader) Float64() float64 {
	if r.err != nil || len(r.b) < 8 {
		r.Fail(ErrShort)
		return 0
	}
	v := math.Float64frombits(binary.LittleEndian.Uint64(r.b))
	r.b = r.b[8:]
	return v
}

func (r *Reader) String() string { return string(r.raw()) }

func (r *Reader) Bytes() []byte {
	raw := r.raw()
	if raw == nil {
		return nil
	}
	return append([]byte{}, raw...)
}

// Count reads a list or map length and checks it is plausible: every
// element takes at least one byte, so a count beyond the remaining bytes is
// corrupt.
func (r *Reader) Count() int {
	n := r.Uint()
	if r.err == nil && n > uint64(len(r.b)) {
		r.Fail(ErrShort)
		return 0
	}
	return int(n)
}

// Cap is the capacity to preallocate for a list of n elements.
func Cap(n int) int {
	if n > maxPrealloc {
		return maxPrealloc
	}
	return n
}

func (r *Reader) raw() []byte {
	n := r.Uint()
	if r.err != nil {
		return nil
	}
	if n > uint64(len(r.b)) {
		r.Fail(ErrShort)
		return nil
	}
	v := r.b[:n]
	r.b = r.b[n:]
	return v
}
