package ui

import (
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

func near(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

// Every command sends exactly the numbers, colors and strings its comment
// in render says, in order.
func TestPainterCommands(t *testing.T) {
	p := &Painter{}
	p.Save()
	p.Translate(10, 20)
	p.Rotate(45)
	p.Scale(2, 3)
	p.Layer(0.5)
	p.Blend(BlendMultiply)
	p.Shadow(1, 2, 8, Hex("#000000"))
	p.Dash(4, 2)
	p.LineStyle(CapButt, JoinBevel)
	p.Arc(5, 5, 4, -90, 180)
	p.Oval(0, 0, 10, 6)
	p.EvenOdd()
	p.Clip()
	p.Image("https://x/y.png", 1, 2, 3, 4, FitContain)
	p.Text("Hi", 5, 6, Style{Width: Pt(80), FontSize: 18, FontWeight: 700, TextAlign: AlignCenter, Lines: 2, FontFamily: "Inter", Color: Hex("#ff0000")})
	p.Restore()
	p.Restore()
	want := []uint8{render.DrawSave, render.DrawTranslate, render.DrawRotate, render.DrawScale, render.DrawLayer, render.DrawBlend,
		render.DrawShadow, render.DrawDash, render.DrawLineStyle, render.DrawArc, render.DrawOval, render.DrawEvenOdd, render.DrawClip,
		render.DrawImage, render.DrawText, render.DrawRestore, render.DrawRestore}
	if !slices.Equal(p.cmds, want) {
		t.Fatalf("cmds %v", p.cmds)
	}
	nums := []float32{10, 20, 45, 2, 3, 0.5, float32(BlendMultiply), 1, 2, 8, 2, 4, 2, float32(CapButt), float32(JoinBevel),
		5, 5, 4, -90, 180, 0, 0, 10, 6, 1, 2, 3, 4, float32(FitContain), 5, 6, 80, 18, 700, float32(render.AlignCenter), 2}
	if !slices.Equal(p.nums, nums) {
		t.Fatalf("nums %v", p.nums)
	}
	if !slices.Equal(p.strs, []string{"https://x/y.png", "Hi", "Inter"}) || len(p.colors) != 2 {
		t.Fatalf("strs %q colors %v", p.strs, p.colors)
	}
}

func TestGradientKinds(t *testing.T) {
	p := &Painter{}
	p.FillGradient(&Gradient{Colors: []Color{1, 2}, Sweep: true, From: Point{5, 5}, To: Point{10, 5}})
	p.StrokeGradient(3, &Gradient{Colors: []Color{1, 2, 3}, Radial: true})
	if !slices.Equal(p.cmds, []uint8{render.DrawFillGradient, render.DrawStrokeGradient}) {
		t.Fatalf("cmds %v", p.cmds)
	}
	want := []float32{2, 5, 5, 10, 5, 2, 0, 1, 3, 1, 0, 0, 0, 0, 3, 0, 0.5, 1}
	if !slices.Equal(p.nums, want) {
		t.Fatalf("nums %v", p.nums)
	}
}

// Trim sends the length of the path it trims.
func TestTrimLength(t *testing.T) {
	for _, c := range []struct {
		name string
		path func(p *Painter)
		want float64
	}{
		{"circle", func(p *Painter) { p.Circle(0, 0, 10) }, 2 * math.Pi * 10},
		{"square", func(p *Painter) { p.Rect(0, 0, 10, 10, 0) }, 40},
		{"rounded", func(p *Painter) { p.Rect(0, 0, 10, 10, 5) }, 2 * math.Pi * 5},
		{"half arc", func(p *Painter) { p.Arc(0, 0, 10, -90, -180) }, math.Pi * 10},
		{"lines", func(p *Painter) { p.MoveTo(0, 0); p.LineTo(3, 4); p.LineTo(3, 0); p.Close() }, 12},
		{"curve", func(p *Painter) { p.MoveTo(0, 0); p.CubicTo(0, 0, 10, 0, 10, 0) }, 10},
		{"circle oval", func(p *Painter) { p.Oval(0, 0, 20, 20) }, 2 * math.Pi * 10},
	} {
		p := &Painter{}
		p.Fill(1) // an earlier path doesn't count
		c.path(p)
		p.Trim(Fixed(0), Fixed(0.5))
		got := p.nums[len(p.nums)-3]
		if !near(float64(got), c.want, 0.05) {
			t.Errorf("%s: length %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSVGPath(t *testing.T) {
	p := &Painter{}
	// Absolute and relative, implicit linetos, H and V, S, T, and Z.
	p.Path("M10,10 l5 0 5 5 H30V0 h-5 v5 C30 10 35 10 40 5 s5-5 10 0 Q55 0 60 5 T70 5 z m2 2 L-1e1 .5")
	want := []uint8{render.DrawMoveTo, render.DrawLineTo, render.DrawLineTo, render.DrawLineTo, render.DrawLineTo, render.DrawLineTo,
		render.DrawLineTo, render.DrawCubicTo, render.DrawCubicTo, render.DrawQuadTo, render.DrawQuadTo, render.DrawClose,
		render.DrawMoveTo, render.DrawLineTo}
	if !slices.Equal(p.cmds, want) {
		t.Fatalf("cmds %v", p.cmds)
	}
	nums := []float32{10, 10, 15, 10, 20, 15, 30, 15, 30, 0, 25, 0, 25, 5, 30, 10, 35, 10, 40, 5,
		45, 0, 45, 0, 50, 5, 55, 0, 60, 5, 65, 10, 70, 5, 12, 12, -10, 0.5}
	if !slices.Equal(p.nums, nums) {
		t.Fatalf("nums %v", p.nums)
	}
	// It stops at nonsense, keeping what came before.
	q := &Painter{}
	q.Path("M0 0 L5 5 L x")
	if len(q.cmds) != 2 {
		t.Fatalf("after nonsense: %v", q.cmds)
	}
}

// An SVG arc ends where it says, and a half circle's curves stay on the
// circle.
func TestSVGArc(t *testing.T) {
	p := &Painter{}
	p.Path("M0 10 A10 10 0 0 1 20 10")
	n := len(p.nums)
	if p.nums[n-2] != 20 || p.nums[n-1] != 10 {
		t.Fatalf("ends at %v", p.nums[n-2:])
	}
	if !near(float64(p.length), math.Pi*10, 0.05) {
		t.Fatalf("length %v, want a half circle's", p.length)
	}
	// From the top left, sweeping clockwise, the half circle goes over the
	// top: its first control point is above.
	if p.nums[3] >= 10 {
		t.Fatalf("went under: %v", p.nums)
	}
	// Flags run into the next number.
	q := &Painter{}
	q.Path("M0 10a10 10 0 0120 0")
	if !slices.Equal(q.nums[len(q.nums)-2:], []float32{20, 10}) {
		t.Fatalf("packed flags: %v", q.nums)
	}
}

func TestEased(t *testing.T) {
	for _, e := range []Ease{EaseInOut, Linear, EaseIn, EaseOut} {
		if v, _ := eased(e, 0, 0, 300); v != 0 {
			t.Errorf("ease %d starts at %v", e, v)
		}
		if v, l := eased(e, 0, 300, 300); v != 1 || l != 300 {
			t.Errorf("ease %d ends at %v after %v", e, v, l)
		}
	}
	if v, _ := eased(EaseInOut, 0, 150, 300); v != 0.5 {
		t.Errorf("ease in-out is %v halfway", v)
	}
	// A spring without bounce never overshoots; with bounce it does, and
	// both settle by the end of their run.
	peak := func(b float64) (float64, float64) {
		top := 0.0
		_, l := eased(Spring, b, 0, 300)
		for t := 0.0; t <= l; t += 1 {
			v, _ := eased(Spring, b, t, 300)
			top = math.Max(top, v)
		}
		end, _ := eased(Spring, b, l, 300)
		return top, end
	}
	if top, end := peak(0); top > 1.0001 || !near(end, 1, 0.002) {
		t.Errorf("no bounce: peak %v, end %v", top, end)
	}
	if top, end := peak(0.5); top < 1.1 || !near(end, 1, 0.002) {
		t.Errorf("bounce: peak %v, end %v", top, end)
	}
}

type spinner struct {
	done *int
	same bool // two canvases share the animation
}

func (s spinner) Render(c *Ctx) Component {
	spin := UseAnimation(c, Animation{Ms: 500, Ease: Linear, OnDone: func() { *s.done++ }})
	ref := Ref(c, spin)
	*ref = spin
	draw := func(p *Painter) {
		p.Motion(Motion{X: Fixed(20), Y: Fixed(20), Rotate: spin.Range(0, 360)})
		p.Arc(0, 0, 10, 0, 270)
		p.Trim(Fixed(0), spin.Range(0, 1))
		p.Stroke(2, 1)
		p.Restore()
	}
	kids := []Component{Canvas(40, 40, draw, Style{}).TestID("a"),
		Pressable(func() { spin.Play() }, Style{}).TestID("play")}
	if s.same {
		kids = append(kids, Canvas(40, 40, draw, Style{}).TestID("b"))
	}
	return View(Style{}, kids...)
}

// A Motion and a Trim send binds to one animation, which the host runs:
// OnDone comes once per run however many canvases report it, and Play
// starts a new run.
func TestCanvasAnimation(t *testing.T) {
	done := 0
	h := start(t, spinner{done: &done, same: true})
	draws := h.find(h.last(), func(o render.Op) bool { _, ok := o.(*render.Draw); return ok })
	if len(draws) != 2 {
		t.Fatalf("%d draws", len(draws))
	}
	d := draws[0].(*render.Draw)
	if len(d.Anims) != render.AnimSize || d.Anims[1] != 0 || d.Anims[3] != 500 || d.Anims[5] != float32(Linear) {
		t.Fatalf("anims %v", d.Anims)
	}
	id := d.Anims[0]
	want := []uint8{render.DrawSave, render.DrawTranslate, render.DrawBind, render.DrawRotate, render.DrawArc,
		render.DrawBind, render.DrawTrim, render.DrawStroke, render.DrawRestore}
	if !slices.Equal(d.Cmds, want) {
		t.Fatalf("cmds %v", d.Cmds)
	}
	ended := func(tag uint32, run int) {
		h.send(&render.NativeEvent{Tag: tag, Name: "anim", Data: fmt.Sprintf("%d:%d", int(id), run)})
	}
	ended(h.tagOf("a"), 0)
	ended(h.tagOf("b"), 0)
	if done != 1 {
		t.Fatalf("OnDone ran %d times", done)
	}
	h.send(&render.Press{Tag: h.tagOf("play")})
	draws = h.find(h.last(), func(o render.Op) bool { _, ok := o.(*render.Draw); return ok })
	if len(draws) != 2 || draws[0].(*render.Draw).Anims[1] != 1 {
		t.Fatalf("Play sent %v", draws)
	}
	ended(h.tagOf("a"), 0) // the old run: late, ignored
	ended(h.tagOf("a"), 1)
	if done != 2 {
		t.Fatalf("OnDone ran %d times after Play", done)
	}
	// Nothing changed: nothing sent.
	h.rerender()
	if len(h.find(h.last(), func(o render.Op) bool { _, ok := o.(*render.Draw); return ok })) != 0 {
		t.Fatal("an unchanged canvas was sent again")
	}
}

type follower struct{ to *float32 }

func (f follower) Render(c *Ctx) Component {
	x := UseTween(c, *f.to, Animation{Ease: Spring})
	return Canvas(10, 10, func(p *Painter) {
		p.Motion(Motion{X: x})
		p.Circle(0, 5, 2)
		p.Fill(1)
		p.Restore()
	}, Style{})
}

// A tween sends its new target, not a new animation.
func TestTween(t *testing.T) {
	to := float32(3)
	h := start(t, follower{&to})
	d := h.find(h.last(), func(o render.Op) bool { _, ok := o.(*render.Draw); return ok })[0].(*render.Draw)
	if d.Anims[2] != 1 || d.Anims[9] != 3 {
		t.Fatalf("anims %v", d.Anims)
	}
	id := d.Anims[0]
	to = 8
	h.rerender()
	d = h.find(h.last(), func(o render.Op) bool { _, ok := o.(*render.Draw); return ok })[0].(*render.Draw)
	if d.Anims[0] != id || d.Anims[9] != 8 {
		t.Fatalf("after a new target: %v", d.Anims)
	}
}
