package ui

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/CodeSyncr/nimbus-native/layout"
	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// Canvas draws shapes that paint describes, in a w×h space fitted to the
// element's frame (centred, keeping its aspect, like an SVG viewBox):
//
//	ui.Canvas(100, 100, func(p *ui.Painter) {
//		p.Circle(50, 50, 40)
//		p.Fill(ui.Hex("#8DC2CC"))
//	}, ui.Style{Width: ui.Pt(60), Height: ui.Pt(60)})
//
// paint runs during the render. To animate, give its numbers an animation
// (UseAnimation, UseTween) through Motion or Trim: the phone runs it, frame
// by frame. Without a Width and Height in s, the canvas is w×h points.
//
//nimbus:param w, h the drawing's own width and height
//nimbus:param paint draws it, during the render
//nimbus:param s its size and place; w×h points when it has none
func Canvas(w, h float32, paint func(p *Painter), s Style) *Element {
	p := &Painter{}
	if paint != nil {
		paint(p)
	}
	if s.Width == (Value{}) {
		s.Width = Pt(w)
	}
	if s.Height == (Value{}) {
		s.Height = Pt(h)
	}
	el := &Element{kind: render.KindCanvas, style: s}
	el.draw = &render.Draw{W: w, H: h, Cmds: p.cmds, Nums: p.nums, Colors: p.colors, Strs: p.strs, Anims: p.animNums()}
	for _, a := range p.anims {
		if a.kind == 2 {
			el.draw.Touch = true
		}
	}
	anims, tags := p.anims, p.tags
	el.onEvent = func(name, data string) {
		switch name {
		case "anim", "drag":
			id, rest, _ := strings.Cut(data, ":")
			for _, a := range anims {
				if strconv.FormatUint(uint64(a.id), 10) != id {
					continue
				}
				if name == "drag" {
					if v, err := strconv.ParseFloat(rest, 32); err == nil {
						a.dragged(float32(v))
					}
				} else if strconv.FormatUint(uint64(a.run), 10) == rest {
					a.ended()
				}
			}
		case "tap":
			if el.onCanvasTap == nil {
				return
			}
			f := strings.SplitN(data, ",", 3)
			if len(f) != 3 {
				return
			}
			n, _ := strconv.Atoi(f[0])
			x, _ := strconv.ParseFloat(f[1], 32)
			y, _ := strconv.ParseFloat(f[2], 32)
			t := Tap{X: float32(x), Y: float32(y)}
			if n > 0 && n <= len(tags) {
				t.ID = tags[n-1]
			}
			el.onCanvasTap(t)
		}
	}
	return el
}

// Tap is a tap on a canvas: the shape it hit (named by Painter.Tag; "" for
// none) and where, in the canvas's own units.
type Tap struct {
	ID string  // the shape's Tag; "" when it hit none
	X  float32 // where, across
	Y  float32 // where, down
}

// OnTap makes a canvas take taps: f gets the topmost tagged shape under
// the finger, as the phone drew it that moment, animations included.
//
//nimbus:param f called on each tap
func (e *Element) OnTap(f func(Tap)) *Element {
	e.onCanvasTap = f
	if e.draw != nil {
		e.draw.Touch = true
	}
	return e
}

// Painter collects a canvas's shapes: build a path (MoveTo, LineTo, …,
// Circle, Rect), then Fill or Stroke it, which starts the next one. Save,
// Translate, Clip, Shadow and the like change what's drawn after them, until
// the Restore that matches their Save.
type Painter struct {
	cmds   []uint8
	nums   []float32
	colors []uint32
	strs   []string
	anims  []*animRef
	tags   []string

	// The path so far, for Trim: its length, where it is and where its
	// subpath began.
	length       float32
	x, y, sx, sy float32
	// Its bounds, for shaders: min and max, once it has a point.
	bx0, by0, bx1, by1 float32
	bounded            bool
}

func (p *Painter) add(c uint8, nums ...float32) {
	p.cmds = append(p.cmds, c)
	p.nums = append(p.nums, nums...)
}

// painted starts a new path.
func (p *Painter) painted() { p.length, p.bounded = 0, false }

func (p *Painter) lineLen(x, y float32) {
	p.length += float32(math.Hypot(float64(x-p.x), float64(y-p.y)))
	p.x, p.y = x, y
	p.grow(x, y, x, y)
}

// grow takes a box into the path's bounds.
func (p *Painter) grow(x0, y0, x1, y1 float32) {
	if !p.bounded {
		p.bx0, p.by0, p.bx1, p.by1, p.bounded = x0, y0, x1, y1, true
		return
	}
	p.bx0, p.by0, p.bx1, p.by1 = min(p.bx0, x0), min(p.by0, y0), max(p.bx1, x1), max(p.by1, y1)
}

// bounds is the path's box, an empty one at the origin before it has any.
func (p *Painter) bounds() (x0, y0, x1, y1 float32) { return p.bx0, p.by0, p.bx1, p.by1 }

// MoveTo starts a new subpath at (x, y).
//
//nimbus:param x, y where the subpath starts
func (p *Painter) MoveTo(x, y float32) {
	p.add(render.DrawMoveTo, x, y)
	p.x, p.y, p.sx, p.sy = x, y, x, y
	p.grow(x, y, x, y)
}

// LineTo draws a straight line to (x, y).
//
//nimbus:param x, y where the line ends
func (p *Painter) LineTo(x, y float32) {
	p.add(render.DrawLineTo, x, y)
	p.lineLen(x, y)
}

// QuadTo draws a quadratic curve to (x, y) bending toward (cx, cy).
//
//nimbus:param cx, cy the point it bends toward
//nimbus:param x, y where the curve ends
func (p *Painter) QuadTo(cx, cy, x, y float32) {
	p.add(render.DrawQuadTo, cx, cy, x, y)
	x0, y0 := p.x, p.y
	p.curveLen(func(t float32) (float32, float32) {
		u := 1 - t
		return u*u*x0 + 2*u*t*cx + t*t*x, u*u*y0 + 2*u*t*cy + t*t*y
	})
}

// CubicTo draws a cubic curve to (x, y) with control points c1 and c2.
//
//nimbus:param c1x, c1y the first control point
//nimbus:param c2x, c2y the second control point
//nimbus:param x, y where the curve ends
func (p *Painter) CubicTo(c1x, c1y, c2x, c2y, x, y float32) {
	p.add(render.DrawCubicTo, c1x, c1y, c2x, c2y, x, y)
	x0, y0 := p.x, p.y
	p.curveLen(func(t float32) (float32, float32) {
		u := 1 - t
		a, b, c, d := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
		return a*x0 + b*c1x + c*c2x + d*x, a*y0 + b*c1y + c*c2y + d*y
	})
}

// curveLen adds a curve's length, as 32 straight pieces.
func (p *Painter) curveLen(at func(t float32) (float32, float32)) {
	for i := 1; i <= 32; i++ {
		p.lineLen(at(float32(i) / 32))
	}
}

// Close joins the subpath's end to its start.
func (p *Painter) Close() {
	p.add(render.DrawClose)
	p.lineLen(p.sx, p.sy)
}

// Circle adds a circle centred on (cx, cy).
//
//nimbus:param cx, cy its centre
//nimbus:param r its radius
func (p *Painter) Circle(cx, cy, r float32) {
	p.add(render.DrawCircle, cx, cy, r)
	p.length += 2 * math.Pi * r
	p.grow(cx-r, cy-r, cx+r, cy+r)
}

// Rect adds a rectangle, its corners rounded by radius.
//
//nimbus:param x, y its top left
//nimbus:param w, h its size
//nimbus:param radius how round its corners are; 0 for square
func (p *Painter) Rect(x, y, w, h, radius float32) {
	p.add(render.DrawRect, x, y, w, h, radius)
	r := max(min(radius, w/2, h/2), 0)
	p.length += 2*(w+h) - 8*r + 2*math.Pi*r
	p.grow(x, y, x+w, y+h)
}

// Arc adds part of a circle's edge, as a subpath of its own: from start
// degrees (clockwise from 3 o'clock) through sweep more (negative turns
// back). A ring that fills up is an Arc from -90, stroked.
//
//nimbus:param cx, cy the circle's centre
//nimbus:param r its radius
//nimbus:param start where it starts, in degrees
//nimbus:param sweep how far it goes, in degrees
func (p *Painter) Arc(cx, cy, r, start, sweep float32) {
	p.add(render.DrawArc, cx, cy, r, start, sweep)
	p.length += float32(math.Abs(float64(sweep))) * math.Pi / 180 * r
	p.grow(cx-r, cy-r, cx+r, cy+r) // the whole circle's: enough for a shader
}

// Oval adds an ellipse that fills the rectangle.
//
//nimbus:param x, y the rectangle's top left
//nimbus:param w, h its size
func (p *Painter) Oval(x, y, w, h float32) {
	p.add(render.DrawOval, x, y, w, h)
	a, b := float64(w/2), float64(h/2) // Ramanujan's
	p.length += float32(math.Pi * (3*(a+b) - math.Sqrt((3*a+b)*(a+3*b))))
	p.grow(x, y, x+w, y+h)
}

// EvenOdd fills the path by the even-odd rule, so a shape inside another
// cuts a hole in it (a ring from two circles).
func (p *Painter) EvenOdd() { p.add(render.DrawEvenOdd) }

// Fill paints the path's inside in c, then starts a new path.
//
//nimbus:param c the color
func (p *Painter) Fill(c Color) {
	p.cmds = append(p.cmds, render.DrawFill)
	p.colors = append(p.colors, uint32(c))
	p.painted()
}

// FillGradient paints the path's inside with g, then starts a new path. Its
// points are in the canvas's own space (not the unit square a Style's
// gradient uses); a radial one spreads from From out to the distance to To,
// and a sweep turns clockwise around From, starting toward To.
//
//nimbus:param g the gradient
func (p *Painter) FillGradient(g *Gradient) {
	if g == nil || len(g.Colors) == 0 {
		return
	}
	p.add(render.DrawFillGradient)
	p.gradient(g)
}

// StrokeGradient draws the path's outline width wide with g, then starts a
// new path.
//
//nimbus:param width how wide the line is
//nimbus:param g the gradient
func (p *Painter) StrokeGradient(width float32, g *Gradient) {
	if g == nil || len(g.Colors) == 0 {
		return
	}
	p.add(render.DrawStrokeGradient, width)
	p.gradient(g)
}

func (p *Painter) gradient(g *Gradient) {
	kind := float32(0)
	if g.Radial {
		kind = 1
	} else if g.Sweep {
		kind = 2
	}
	n := len(g.Colors)
	p.nums = append(p.nums, kind, g.From.X, g.From.Y, g.To.X, g.To.Y, float32(n))
	for i := 0; i < n; i++ {
		stop := float32(i) / float32(max(n-1, 1))
		if len(g.Stops) == n {
			stop = g.Stops[i]
		}
		p.nums = append(p.nums, stop)
		p.colors = append(p.colors, uint32(g.Colors[i]))
	}
	p.painted()
}

// Stroke draws the path's outline width wide in c, then starts a new path.
//
//nimbus:param width how wide the line is
//nimbus:param c the color
func (p *Painter) Stroke(width float32, c Color) {
	p.add(render.DrawStroke, width)
	p.colors = append(p.colors, uint32(c))
	p.painted()
}

// Clip keeps what's drawn after it inside the path, until Restore, and
// starts a new path.
func (p *Painter) Clip() {
	p.add(render.DrawClip)
	p.painted()
}

// Tag names the shape the next Fill, Stroke or gradient paints, for OnTap
// and for a Drag's Handle. Taps go to the topmost tagged shape: the last
// painted.
//
//nimbus:param id its name
func (p *Painter) Tag(id string) {
	n := slices.Index(p.tags, id)
	if n < 0 {
		p.tags = append(p.tags, id)
		n = len(p.tags) - 1
	}
	p.add(render.DrawTag, float32(n+1))
}

// Save remembers the transform, clip, blend, shadow, dashes and line style,
// for Restore to bring back.
func (p *Painter) Save() { p.add(render.DrawSave) }

// Restore goes back to the last Save (or Layer, or Motion).
func (p *Painter) Restore() { p.add(render.DrawRestore) }

// Layer is a Save whose drawing, up to its Restore, is blended in as one
// picture at opacity: overlapping shapes inside don't show through each
// other.
//
//nimbus:param opacity 0 (invisible) to 1
func (p *Painter) Layer(opacity float32) { p.add(render.DrawLayer, opacity) }

// Translate moves what's drawn after it by (x, y).
//
//nimbus:param x, y how far
func (p *Painter) Translate(x, y float32) { p.add(render.DrawTranslate, x, y) }

// Rotate turns what's drawn after it clockwise by degrees, around the
// origin: Translate to the pivot first.
//
//nimbus:param degrees clockwise
func (p *Painter) Rotate(degrees float32) { p.add(render.DrawRotate, degrees) }

// Scale stretches what's drawn after it, from the origin.
//
//nimbus:param sx, sy across and down; 1 leaves it
func (p *Painter) Scale(sx, sy float32) { p.add(render.DrawScale, sx, sy) }

// BlendMode is how a shape's colors combine with what's under it.
type BlendMode uint8

const (
	BlendNormal BlendMode = iota
	BlendMultiply
	BlendScreen
	BlendOverlay
	BlendDarken
	BlendLighten
	BlendColorDodge
	BlendColorBurn
	BlendHardLight
	BlendSoftLight
	BlendDifference
	BlendExclusion
	BlendHue
	BlendSaturation
	BlendColor
	BlendLuminosity
	BlendErase // erases what's under the shape, as much as it's opaque
	BlendAtop  // draws only over what's already drawn
	BlendPlus  // adds the colors
)

// Blend sets how what's drawn after it combines with what's under it. Inside
// a Layer, BlendErase and BlendAtop only touch the layer's own drawing: a
// Layer, a picture, then a gradient drawn with BlendErase fades the picture
// out. Android 9 and older draw BlendColorDodge to BlendLuminosity as
// BlendNormal.
//
//nimbus:param mode BlendNormal, BlendMultiply, …
func (p *Painter) Blend(mode BlendMode) { p.add(render.DrawBlend, float32(mode)) }

// Shadow gives what's drawn after it a shadow, (dx, dy) away, blurred by
// blur points (as CSS's box-shadow), in c; 0 for c removes it. Android 8
// draws none.
//
//nimbus:param dx, dy how far the shadow falls
//nimbus:param blur how soft it is
//nimbus:param c its color; 0 for none
func (p *Painter) Shadow(dx, dy, blur float32, c Color) {
	p.add(render.DrawShadow, dx, dy, blur)
	p.colors = append(p.colors, uint32(c))
}

// Dash makes strokes after it dashed: lengths on, off, on, …; none makes them
// solid again.
//
//nimbus:param lengths on, off, on, … in the canvas's units
func (p *Painter) Dash(lengths ...float32) {
	p.add(render.DrawDash, float32(len(lengths)))
	p.nums = append(p.nums, lengths...)
}

// Cap is how a stroke's open ends look.
type Cap uint8

const (
	CapRound  Cap = iota // a half circle past the end
	CapButt              // square, at the end
	CapSquare            // square, half the width past the end
)

// Join is how a stroke turns a corner.
type Join uint8

const (
	JoinRound Join = iota
	JoinMiter
	JoinBevel
)

// LineStyle sets the ends and corners of strokes after it. Strokes start
// round.
//
//nimbus:param cap CapRound, CapButt or CapSquare
//nimbus:param join JoinRound, JoinMiter or JoinBevel
func (p *Painter) LineStyle(cap Cap, join Join) {
	p.add(render.DrawLineStyle, float32(cap), float32(join))
}

// Trim makes the next stroke show only from start to end of the path's
// length, as fractions (0 to 1): a ring that fills up, a line that draws
// itself with an Anim's Range. Call it after the path, before Stroke. It
// replaces Dash for that stroke, and works on a path in one piece.
//
//nimbus:param start where the shown part starts, 0 to 1
//nimbus:param end where it ends, 0 to 1
func (p *Painter) Trim(start, end Num) {
	p.bind(1, start)
	p.bind(2, end)
	p.add(render.DrawTrim, p.length, start.from, end.from)
}

// Image draws the picture at url (as ui.Image takes) in the rectangle,
// fitted by fit (FitCover, FitContain, FitStretch); nothing until it has
// loaded.
//
//nimbus:param url where the picture is
//nimbus:param x, y the rectangle's top left
//nimbus:param w, h its size
//nimbus:param fit FitCover, FitContain or FitStretch
func (p *Painter) Image(url string, x, y, w, h float32, fit render.Fit) {
	p.add(render.DrawImage, x, y, w, h, float32(fit))
	p.strs = append(p.strs, url)
}

// Text writes s with its top left at (x, y), in s's FontSize, FontWeight,
// FontFamily, Color, TextAlign and Lines. With a Width in points it wraps
// there, and TextAlign aligns within it. It isn't part of the path.
//
//nimbus:param text what it says
//nimbus:param x, y its top left
//nimbus:param s its font, color, alignment, Lines and wrapping Width
func (p *Painter) Text(text string, x, y float32, s Style) {
	width := float32(0)
	if s.Width.U == layout.Point {
		width = s.Width.V
	}
	size := s.FontSize
	if size == 0 {
		size = 16
	}
	weight := float32(s.FontWeight)
	if weight == 0 {
		weight = 400
	}
	p.add(render.DrawText, x, y, width, size, weight, float32(textAlign(s.TextAlign)), float32(s.Lines))
	p.strs = append(p.strs, text, s.FontFamily)
	p.colors = append(p.colors, uint32(s.Color))
}
