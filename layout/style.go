// Package layout is Nimbus Native's flexbox engine, in pure Go so the same
// layout runs on iOS, Android and (later) the web.
//
// It follows the CSS Flexbox algorithm with Nimbus Native's defaults: column
// direction, flex-shrink 0, no automatic minimum size, every node a flex
// container. Results are in points (iOS) / dp (Android); hosts round to
// pixels. conformance_test.go checks it against Chromium's own layout.
package layout

import "math"

// Unit says how a Value is measured.
type Unit uint8

const (
	Auto    Unit = iota // the zero Value: size from content or flex
	Point               // absolute, in points / dp
	Percent             // of the parent's inner size on that axis
)

// Value is a length: Auto, points or a percentage.
type Value struct {
	V float32
	U Unit
}

// Pt is a length in points.
func Pt(v float32) Value { return Value{v, Point} }

// Pct is a percentage of the parent's inner size.
func Pct(v float32) Value { return Value{v, Percent} }

// resolve turns v into points against owner (the parent's inner size on the
// same axis), or undefined.
func (v Value) resolve(owner float32) float32 {
	switch v.U {
	case Point:
		return v.V
	case Percent:
		if defined(owner) {
			return v.V * owner / 100
		}
	}
	return undefined
}

type Direction uint8

const (
	Column Direction = iota
	Row
)

type Justify uint8

const (
	JustifyStart Justify = iota
	JustifyCenter
	JustifyEnd
	JustifySpaceBetween
	JustifySpaceAround
	JustifySpaceEvenly
)

// Align positions children on the cross axis. On AlignSelf, AlignAuto
// means "use the parent's AlignItems"; on AlignItems it means Stretch.
type Align uint8

const (
	AlignAuto Align = iota
	AlignStretch
	AlignStart
	AlignCenter
	AlignEnd
)

type Position uint8

const (
	Relative Position = iota
	Absolute
)

// Edges are per-side lengths in points.
type Edges struct{ Left, Top, Right, Bottom float32 }

// All is the same length on every side.
func All(v float32) Edges { return Edges{v, v, v, v} }

// Style is everything that affects a node's layout. The zero Style is an
// auto-sized column that stretches its children.
type Style struct {
	Direction  Direction
	Justify    Justify
	AlignItems Align
	AlignSelf  Align

	Grow   float32
	Shrink float32
	Basis  Value

	Width, Height       Value
	MinWidth, MinHeight Value
	MaxWidth, MaxHeight Value

	Padding Edges
	Margin  Edges
	Border  Edges
	Gap     float32 // between children on the main axis

	// Wrap starts a new line when the children don't fit along the main
	// axis (flex-wrap: wrap, lines packed at the start); LineGap is the
	// space between lines, Gap when zero.
	Wrap    bool
	LineGap float32

	Position                 Position
	Left, Top, Right, Bottom Value // insets; Auto means unset

	// Hidden removes the node from layout (display: none).
	Hidden bool
	// Scroll lays children out unconstrained along the main axis, as the
	// content of a scroll view. The node's own size is unaffected.
	Scroll bool
}

var undefined = float32(math.NaN())

func defined(v float32) bool { return v == v }

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// clamp bounds v by min and max, either of which may be undefined. Min wins
// over max, as in CSS.
func clamp(v, lo, hi float32) float32 {
	if defined(hi) && v > hi {
		v = hi
	}
	if defined(lo) && v < lo {
		v = lo
	}
	return v
}
