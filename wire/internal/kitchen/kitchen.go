// Package kitchen is a wire schema that uses every feature the generators
// support. The conformance test encodes values in Go and checks that the
// generated Swift and Java decode and re-encode them byte for byte.
package kitchen

// Kind is a small enum; values outside the list must survive a round trip.
//
//wire:enum
type Kind uint8

const (
	KindText Kind = iota
	KindImage
	KindBigText
)

// Level is a signed enum.
//
//wire:enum
type Level int16

const (
	LevelLow  Level = -1
	LevelHigh Level = 1
)

// Scalars covers every scalar type at the edges of its range.
//
//wire:message
type Scalars struct {
	B   bool
	U8  uint8
	U16 uint16
	U32 uint32
	U64 uint64
	I8  int8
	I16 int16
	I32 int32
	I64 int64
	I   int
	U   uint
	F32 float32
	F64 float64
	S   string
	Raw []byte
}

// Collections covers lists, maps and optionals, nested.
//
//wire:message
type Collections struct {
	Names    []string
	Matrix   [][]int32
	Scores   map[string]float64
	ByID     map[uint64]string
	Signed   map[int32]bool
	Nested   map[string][]uint16
	MaybeU32 *uint32
	MaybeS   *string
	MaybeF   *float64
	Kinds    []Kind
	MaybeK   *Kind
	Level    Level
}

// Node is a recursive tree, the shape a UI tree will have.
//
//wire:message
type Node struct {
	ID       uint32
	Kind     Kind
	Label    string
	Props    map[string]string
	Children []*Node
	Meta     *Meta
	Default  string // a Swift and Java keyword as a field name
}

// Meta is a plain nested struct.
type Meta struct {
	Created int64
	Tags    []string
}

// Op is a union: one of several structs.
//
//wire:union
type Op interface{ isOp() }

// Create adds a node.
//
//wire:variant Op
type Create struct {
	Parent uint32
	Node   Node
}

// Remove deletes a node.
//
//wire:variant Op
type Remove struct {
	ID uint32
}

// Batch is a list of ops — the shape of a UI commit.
//
//wire:message
type Batch struct {
	Seq     uint64
	Ops     []Op
	Current Op
}
