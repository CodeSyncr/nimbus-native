package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/CodeSyncr/nimbus-native/layout"
	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// Layout vocabulary, re-exported so apps only import ui.
type (
	Value     = layout.Value
	Edges     = layout.Edges
	Direction = layout.Direction
	Justify   = layout.Justify
	Align     = layout.Align
)

const (
	Column = layout.Column
	Row    = layout.Row

	JustifyStart        = layout.JustifyStart
	JustifyCenter       = layout.JustifyCenter
	JustifyEnd          = layout.JustifyEnd
	JustifySpaceBetween = layout.JustifySpaceBetween
	JustifySpaceAround  = layout.JustifySpaceAround
	JustifySpaceEvenly  = layout.JustifySpaceEvenly

	AlignStretch = layout.AlignStretch
	AlignStart   = layout.AlignStart
	AlignCenter  = layout.AlignCenter
	AlignEnd     = layout.AlignEnd
)

// Pt is a length in points (dp on Android).
func Pt(v float32) Value { return layout.Pt(v) }

// Pct is a percentage of the parent's inner size.
func Pct(v float32) Value { return layout.Pct(v) }

// All is the same length on every side.
func All(v float32) Edges { return layout.All(v) }

// XY is horizontal and vertical lengths: XY(16, 8) pads 16 left and right,
// 8 top and bottom.
func XY(x, y float32) Edges { return Edges{Left: x, Top: y, Right: x, Bottom: y} }

// Color is 0xRRGGBBAA. The zero Color means "unset".
type Color uint32

// RGB is an opaque color.
func RGB(r, g, b uint8) Color { return Color(uint32(r)<<24 | uint32(g)<<16 | uint32(b)<<8 | 0xff) }

// Hex parses "#rgb", "#rrggbb" or "#rrggbbaa". It panics on bad input, so
// use it for constants.
func Hex(s string) Color {
	h := strings.TrimPrefix(s, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) == 6 {
		h += "ff"
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil || len(h) != 8 {
		panic(fmt.Sprintf("ui.Hex(%q): want #rgb, #rrggbb or #rrggbbaa", s))
	}
	return Color(v)
}

// Style is how an element looks and lays out. The zero Style is an
// auto-sized column that stretches its children.
type Style struct {
	// Layout
	Direction  Direction
	Justify    Justify
	AlignItems Align
	AlignSelf  Align

	Grow, Shrink float32
	Basis        Value

	Width, Height       Value
	MinWidth, MinHeight Value
	MaxWidth, MaxHeight Value

	Padding Edges
	Margin  Edges
	Gap     float32
	// Wrap moves children that don't fit onto new lines (rows of chips,
	// grids of tags); LineGap spaces the lines, Gap when zero.
	Wrap    bool
	LineGap float32

	// Transition animates this view's changes of position, size, opacity
	// and background over this many milliseconds, instead of jumping.
	Transition int

	Absolute                 bool
	Left, Top, Right, Bottom Value
	Hidden                   bool

	// Box
	Background  Color
	Radius      float32
	BorderWidth float32
	BorderColor Color
	Opacity     float32 // 0 means 1 (fully opaque); use Hidden to hide
	Clip        bool
	Corners     Corners   // which corners Radius rounds; 0 means all four
	Blur        Blur      // what is behind shows through, blurred; Background tints it
	Gradient    *Gradient // painted over Background
	Shadow      *Shadow   // drawn under the view

	// Text and inputs
	FontSize   float32 // 0 means 16
	FontWeight uint16  // 100..900; 0 means 400
	FontFamily string  // a font the app bundles (`nimbus native font add`); "" is the system font
	Color      Color   // text color; 0 means the platform default
	TextAlign  Align   // AlignStart, AlignCenter, AlignEnd
	Lines      uint32  // 0 = unlimited
	// LetterSpacing is extra space between letters, in points; negative
	// tightens a large headline.
	LetterSpacing float32
	// LineHeight is the distance from one line to the next, in points; 0 is
	// the font's own.
	LineHeight float32
	// TabularDigits gives every digit the same width, so figures line up in
	// columns and don't jiggle as they change.
	TabularDigits bool
	// MinScale lets a one-line text (Lines: 1) shrink to fit its width, down
	// to this share of its size (0.6 is 60%); 0 truncates with an ellipsis.
	MinScale float32
}

// Blur is how strongly what is behind a view is blurred (Style.Blur): the
// platform's materials on iOS, a backdrop blur on Android 12+ and the web.
// Older Android shows the Background tint alone.
type Blur = render.Blur

const (
	BlurNone      = render.BlurNone
	BlurUltraThin = render.BlurUltraThin
	BlurThin      = render.BlurThin
	BlurRegular   = render.BlurRegular
	BlurThick     = render.BlurThick
	BlurGlass     = render.BlurGlass // Liquid Glass on iOS 26+; elsewhere a thin blur, tinted
)

// Corners picks the corners Style.Radius rounds.
type Corners uint8

const (
	CornerTopLeft     Corners = render.CornerTopLeft
	CornerTopRight    Corners = render.CornerTopRight
	CornerBottomRight Corners = render.CornerBottomRight
	CornerBottomLeft  Corners = render.CornerBottomLeft

	CornersLeft   = CornerTopLeft | CornerBottomLeft
	CornersRight  = CornerTopRight | CornerBottomRight
	CornersTop    = CornerTopLeft | CornerTopRight
	CornersBottom = CornerBottomLeft | CornerBottomRight
)

// Point is a place in a view's unit square: (0, 0) its top left, (1, 1)
// its bottom right.
type Point struct{ X, Y float32 }

// Gradient is a fill that blends between colors (Style.Gradient).
type Gradient struct {
	Colors []Color
	Stops  []float32 // where each color sits, 0..1; empty spaces them evenly
	Radial bool      // from From outward to the distance to To; else along From → To
	From   Point
	To     Point
}

// LinearGradient blends colors along the line from → to:
//
//	ui.LinearGradient(ui.Point{0, 0}, ui.Point{0, 1}, top, bottom) // top to bottom
func LinearGradient(from, to Point, colors ...Color) *Gradient {
	return &Gradient{Colors: colors, From: from, To: to}
}

// RadialGradient blends colors outward from center, reaching the last at
// radius (a share of the view's width).
func RadialGradient(center Point, radius float32, colors ...Color) *Gradient {
	return &Gradient{Colors: colors, Radial: true, From: center, To: Point{center.X + radius, center.Y}}
}

// Shadow is a shadow under a view (Style.Shadow): its color (with alpha),
// how far it spreads, and its offset.
type Shadow struct {
	Color  Color
	Radius float32
	X, Y   float32
}

// Flex makes an element take its share of the free space: Flex(1) fills
// what is left, like grow 1, shrink 1, basis 0.
func Flex(n float32) Style { return Style{Grow: n, Shrink: 1, Basis: Pt(0)} }

// With returns s with the non-zero fields of o laid over it, so styles
// compose: base.With(ui.Style{Background: red}).
func (s Style) With(o Style) Style {
	pick := func(a, b float32) float32 {
		if b != 0 {
			return b
		}
		return a
	}
	pickV := func(a, b Value) Value {
		if b != (Value{}) {
			return b
		}
		return a
	}
	if o.Direction != 0 {
		s.Direction = o.Direction
	}
	if o.Justify != 0 {
		s.Justify = o.Justify
	}
	if o.AlignItems != 0 {
		s.AlignItems = o.AlignItems
	}
	if o.AlignSelf != 0 {
		s.AlignSelf = o.AlignSelf
	}
	s.Grow, s.Shrink = pick(s.Grow, o.Grow), pick(s.Shrink, o.Shrink)
	s.Basis = pickV(s.Basis, o.Basis)
	s.Width, s.Height = pickV(s.Width, o.Width), pickV(s.Height, o.Height)
	s.MinWidth, s.MinHeight = pickV(s.MinWidth, o.MinWidth), pickV(s.MinHeight, o.MinHeight)
	s.MaxWidth, s.MaxHeight = pickV(s.MaxWidth, o.MaxWidth), pickV(s.MaxHeight, o.MaxHeight)
	if o.Padding != (Edges{}) {
		s.Padding = o.Padding
	}
	if o.Margin != (Edges{}) {
		s.Margin = o.Margin
	}
	s.Gap = pick(s.Gap, o.Gap)
	s.LineGap = pick(s.LineGap, o.LineGap)
	s.Wrap = s.Wrap || o.Wrap
	if o.Transition != 0 {
		s.Transition = o.Transition
	}
	s.Absolute = s.Absolute || o.Absolute
	s.Left, s.Top = pickV(s.Left, o.Left), pickV(s.Top, o.Top)
	s.Right, s.Bottom = pickV(s.Right, o.Right), pickV(s.Bottom, o.Bottom)
	s.Hidden = s.Hidden || o.Hidden
	if o.Background != 0 {
		s.Background = o.Background
	}
	s.Radius, s.BorderWidth = pick(s.Radius, o.Radius), pick(s.BorderWidth, o.BorderWidth)
	if o.BorderColor != 0 {
		s.BorderColor = o.BorderColor
	}
	s.Opacity = pick(s.Opacity, o.Opacity)
	s.Clip = s.Clip || o.Clip
	s.FontSize = pick(s.FontSize, o.FontSize)
	if o.FontFamily != "" {
		s.FontFamily = o.FontFamily
	}
	if o.FontWeight != 0 {
		s.FontWeight = o.FontWeight
	}
	if o.Color != 0 {
		s.Color = o.Color
	}
	if o.TextAlign != 0 {
		s.TextAlign = o.TextAlign
	}
	if o.Lines != 0 {
		s.Lines = o.Lines
	}
	return s
}

func (s *Style) layout() layout.Style {
	ls := layout.Style{
		Direction: s.Direction, Justify: s.Justify, AlignItems: s.AlignItems, AlignSelf: s.AlignSelf,
		Grow: s.Grow, Shrink: s.Shrink, Basis: s.Basis,
		Width: s.Width, Height: s.Height, MinWidth: s.MinWidth, MinHeight: s.MinHeight,
		MaxWidth: s.MaxWidth, MaxHeight: s.MaxHeight,
		Padding: s.Padding, Margin: s.Margin, Gap: s.Gap, Wrap: s.Wrap, LineGap: s.LineGap,
		Left: s.Left, Top: s.Top, Right: s.Right, Bottom: s.Bottom, Hidden: s.Hidden,
	}
	if s.BorderWidth > 0 {
		ls.Border = All(s.BorderWidth)
	}
	if s.Absolute {
		ls.Position = layout.Absolute
	}
	return ls
}

func (s *Style) fontSize() float32 {
	if s.FontSize == 0 {
		return 16
	}
	return s.FontSize
}

func (s *Style) opacity() float32 {
	if s.Opacity == 0 {
		return 1
	}
	return s.Opacity
}
