package ui

import (
	"math"
	"sync/atomic"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// Ease is how an animation speeds up and slows down.
type Ease uint8

const (
	EaseInOut Ease = iota // starts and ends gently
	Linear                // the same speed throughout
	EaseIn                // starts gently
	EaseOut               // ends gently
	Spring                // like a spring, overshooting by Animation.Bounce
)

// Forever repeats an animation until its canvas leaves the screen.
const Forever = -1

// Animation is how a canvas animates a number: the phone runs it, frame by
// frame, without a render from Go.
type Animation struct {
	Ms      int     // how long one run takes; 0 means 300
	Delay   int     // milliseconds before the first run
	Ease    Ease    // EaseInOut when unset
	Bounce  float32 // Spring only: 0 settles without overshooting, 0.5 bounces well
	Repeat  int     // more runs after the first; Forever
	Reverse bool    // every other run goes backward
	OnDone  func()  // after the last run (never for Forever)
}

// Num is a number a canvas animates, for Painter.Motion and Painter.Trim.
// The zero Num is unset; Fixed is a number that doesn't move.
type Num struct {
	anim     *animRef
	from, to float32
}

// Fixed is a Num that stays v.
//
//nimbus:param v the number
func Fixed(v float32) Num { return Num{from: v, to: v} }

func (n Num) set() bool { return n.anim != nil || n.from != 0 }

// Anim is an animation of a canvas: Range turns it into the numbers it
// moves through.
type Anim struct{ ref *animRef }

type animRef struct {
	id, run uint32
	kind    float32 // 0: a clock, 1: follows to
	spec    Animation
	to      float32
	done    uint32 // run+1 of the run OnDone was called for
	play    func()
}

// ended is a host saying the run is over: OnDone, once per run, however
// many canvases use it.
func (r *animRef) ended() {
	if r.kind == 0 && r.done != r.run+1 {
		r.done = r.run + 1
		if r.spec.OnDone != nil {
			r.spec.OnDone()
		}
	}
}

var animIDs atomic.Uint32

// UseAnimation is an animation that starts when a canvas using it first
// shows, and runs as a says:
//
//	spin := ui.UseAnimation(c, ui.Animation{Ms: 1000, Ease: ui.Linear, Repeat: ui.Forever})
//	ui.Canvas(40, 40, func(p *ui.Painter) {
//		p.Motion(ui.Motion{X: ui.Fixed(20), Y: ui.Fixed(20), Rotate: spin.Range(0, 360)})
//		p.Arc(0, 0, 16, 0, 270)
//		p.Stroke(4, th.Primary)
//		p.Restore()
//	}, ui.Style{})
//
//nimbus:param a how it runs
func UseAnimation(c *Ctx, a Animation) Anim {
	r := Ref(c, animRef{})
	if r.id == 0 {
		r.id = animIDs.Add(1)
		rt, inst := c.rt, c.inst
		r.play = func() {
			rt.do(func() {
				if inst.unmounted {
					return
				}
				r.run++
				inst.dirty, rt.dirty = true, true
			})
		}
	}
	r.spec, r.to = a, 1
	return Anim{r}
}

// Range is the animation moving from from to to (and back, if it reverses).
//
//nimbus:param from where it starts
//nimbus:param to where it ends
func (a Anim) Range(from, to float32) Num { return Num{a.ref, from, to} }

// Play runs the animation again from the start.
func (a Anim) Play() { a.ref.play() }

// UseTween is a number that moves smoothly to target whenever target
// changes, from wherever it is on screen, as a says (Repeat and Reverse
// aside). It starts at target.
//
//nimbus:param target where the number goes
//nimbus:param a how it gets there
func UseTween(c *Ctx, target float32, a Animation) Num {
	r := Ref(c, animRef{})
	if r.id == 0 {
		r.id = animIDs.Add(1)
	}
	r.kind, r.spec, r.to = 1, a, target
	r.spec.Repeat, r.spec.Reverse = 0, false
	return Num{r, 0, 1}
}

// Motion moves, turns, scales and fades what's drawn after it, until its
// Restore, with numbers that can animate. It turns and scales about the
// point it moves to. Unset fields leave it be. Like Save, it needs a
// Restore.
type Motion struct {
	X       Num // moves it across
	Y       Num // moves it down
	Rotate  Num // turns it, in degrees, clockwise
	Scale   Num // scales it; 1 is its own size
	Opacity Num // fades it as one picture, 0 to 1
}

// Motion starts a group moved as m says; Restore ends it.
//
//nimbus:param m how it moves
func (p *Painter) Motion(m Motion) {
	if m.Opacity.set() {
		p.bind(0, m.Opacity)
		p.add(render.DrawLayer, m.Opacity.from)
	} else {
		p.add(render.DrawSave)
	}
	if m.X.set() || m.Y.set() {
		p.bind(0, m.X)
		p.bind(1, m.Y)
		p.add(render.DrawTranslate, m.X.from, m.Y.from)
	}
	if m.Rotate.set() {
		p.bind(0, m.Rotate)
		p.add(render.DrawRotate, m.Rotate.from)
	}
	if m.Scale.set() {
		p.bind(0, m.Scale)
		p.bind(1, m.Scale)
		p.add(render.DrawScale, m.Scale.from, m.Scale.from)
	}
}

// bind makes the next command's arg-th number follow n's animation.
func (p *Painter) bind(arg int, n Num) {
	if n.anim == nil {
		return
	}
	i := -1
	for j, a := range p.anims {
		if a == n.anim {
			i = j
		}
	}
	if i < 0 {
		i = len(p.anims)
		p.anims = append(p.anims, n.anim)
	}
	p.add(render.DrawBind, float32(arg), float32(i), n.from, n.to)
}

func (p *Painter) animNums() []float32 {
	var out []float32
	for _, a := range p.anims {
		s := a.spec
		ms := s.Ms
		if ms <= 0 {
			ms = 300
		}
		reverse := float32(0)
		if s.Reverse {
			reverse = 1
		}
		out = append(out, float32(a.id), float32(a.run), a.kind, float32(ms), float32(max(s.Delay, 0)),
			float32(s.Ease), s.Bounce, float32(s.Repeat), reverse, a.to)
	}
	return out
}

// eased is where an animation of ms milliseconds is at, t milliseconds into
// a run, and how long a run takes: ms, or longer for a spring to settle.
// The hosts do the same sums.
func eased(e Ease, bounce, t, ms float64) (v, length float64) {
	if e == Spring {
		// A spring of period ms, run until it's within 0.001 of the end.
		zeta := math.Min(math.Max(1-bounce, 0.05), 1)
		w := 2 * math.Pi
		if zeta == 1 {
			length = ms * 9.24 / w // (1 + wτ)e^(-wτ) = 0.001
			tau := math.Min(t, length) / ms
			return 1 - math.Exp(-w*tau)*(1+w*tau), length
		}
		wd := w * math.Sqrt(1-zeta*zeta)
		length = ms * math.Max(1, math.Log(1000*(1+zeta*w/wd))/(zeta*w))
		tau := math.Min(t, length) / ms
		return 1 - math.Exp(-zeta*w*tau)*(math.Cos(wd*tau)+zeta*w/wd*math.Sin(wd*tau)), length
	}
	x := math.Min(math.Max(t/ms, 0), 1)
	switch e {
	case Linear:
		return x, ms
	case EaseIn:
		return x * x * x, ms
	case EaseOut:
		return 1 - math.Pow(1-x, 3), ms
	}
	if x < 0.5 {
		return 4 * x * x * x, ms
	}
	return 1 - 4*math.Pow(1-x, 3), ms
}
