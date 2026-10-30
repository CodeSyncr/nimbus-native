package ui

import "github.com/CodeSyncr/nimbus-native/ui/render"

// Canvas draws shapes that paint describes, in a w×h space fitted to the
// element's frame (centred, keeping its aspect, like an SVG viewBox):
//
//	ui.Canvas(100, 100, func(p *ui.Painter) {
//		p.Circle(50, 50, 40)
//		p.Fill(ui.Hex("#8DC2CC"))
//	}, ui.Style{Width: ui.Pt(60), Height: ui.Pt(60)})
//
// paint runs during the render, so to animate, render again (from a timer's
// State) with new shapes. Without a Width and Height in s, the canvas is
// w×h points.
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
	return &Element{kind: render.KindCanvas, style: s, draw: &render.Draw{W: w, H: h, Cmds: p.cmds, Nums: p.nums, Colors: p.colors}}
}

// Painter collects a canvas's shapes: build a path (MoveTo, LineTo, …,
// Circle, Rect), then Fill or Stroke it, which starts the next one.
type Painter struct {
	cmds   []uint8
	nums   []float32
	colors []uint32
}

func (p *Painter) add(c uint8, nums ...float32) {
	p.cmds = append(p.cmds, c)
	p.nums = append(p.nums, nums...)
}

// MoveTo starts a new subpath at (x, y).
func (p *Painter) MoveTo(x, y float32) { p.add(render.DrawMoveTo, x, y) }

// LineTo draws a straight line to (x, y).
func (p *Painter) LineTo(x, y float32) { p.add(render.DrawLineTo, x, y) }

// QuadTo draws a quadratic curve to (x, y) bending toward (cx, cy).
func (p *Painter) QuadTo(cx, cy, x, y float32) { p.add(render.DrawQuadTo, cx, cy, x, y) }

// CubicTo draws a cubic curve to (x, y) with control points c1 and c2.
func (p *Painter) CubicTo(c1x, c1y, c2x, c2y, x, y float32) {
	p.add(render.DrawCubicTo, c1x, c1y, c2x, c2y, x, y)
}

// Close joins the subpath's end to its start.
func (p *Painter) Close() { p.add(render.DrawClose) }

// Circle adds a circle centred on (cx, cy).
func (p *Painter) Circle(cx, cy, r float32) { p.add(render.DrawCircle, cx, cy, r) }

// Rect adds a rectangle, its corners rounded by radius.
func (p *Painter) Rect(x, y, w, h, radius float32) { p.add(render.DrawRect, x, y, w, h, radius) }

// Fill paints the path's inside in c, then starts a new path.
func (p *Painter) Fill(c Color) {
	p.cmds = append(p.cmds, render.DrawFill)
	p.colors = append(p.colors, uint32(c))
}

// Stroke draws the path's outline width wide in c, then starts a new path.
func (p *Painter) Stroke(width float32, c Color) {
	p.add(render.DrawStroke, width)
	p.colors = append(p.colors, uint32(c))
}
