package ui

import (
	"fmt"
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

type homeScreen struct{}

func (homeScreen) Render(c *Ctx) *Element {
	return View(Style{}, C(openButton{}))
}

// openButton is nested below the screen, to check Navigation finds the
// stack through any depth of components and views.
type openButton struct{}

func (openButton) Render(c *Ctx) *Element {
	nav := Navigation(c)
	return Pressable(func() { nav.Push("Detail", detailScreen{id: 7}) }, Style{Height: Pt(44)}).TestID("open")
}

type detailScreen struct{ id int }

func (d detailScreen) Render(c *Ctx) *Element {
	nav := Navigation(c)
	return View(Style{},
		Text("detail", Style{}).TestID("detail"),
		Pressable(func() { nav.Pop() }, Style{Height: Pt(44)}).TestID("back"),
	)
}

type navApp struct{}

func (navApp) Render(c *Ctx) *Element {
	return Stack("Home", homeScreen{}, Flex(1)).TestID("stack")
}

func (h *host) all(pred func(render.Op) bool) []render.Op {
	var out []render.Op
	for _, c := range h.commits {
		out = append(out, h.find(c, pred)...)
	}
	return out
}

func TestStackPushAndPop(t *testing.T) {
	h := start(t, navApp{})
	stack := h.tagOf("stack")
	titles := h.all(func(o render.Op) bool { _, ok := o.(*render.Screen); return ok })
	if len(titles) != 1 || titles[0].(*render.Screen).Title != "Home" {
		t.Fatalf("titles after start: %v", titles)
	}

	h.send(&render.Press{Tag: h.tagOf("open")})
	c := h.last()
	ch := h.find(c, func(o render.Op) bool { x, ok := o.(*render.Children); return ok && x.Tag == stack })
	if len(ch) != 1 || len(ch[0].(*render.Children).Children) != 2 {
		t.Fatalf("stack children after push: %v", ch)
	}
	if s := h.find(c, func(o render.Op) bool { x, ok := o.(*render.Screen); return ok && x.Title == "Detail" }); len(s) != 1 {
		t.Fatalf("no Detail title in %v", c.Ops)
	}
	if h.tagOf("detail") == 0 {
		t.Fatal("detail screen not rendered")
	}

	h.send(&render.Press{Tag: h.tagOf("back")})
	if h.tagOf("detail") != 0 {
		t.Fatal("Pop left the detail screen")
	}
	if d := h.find(h.last(), func(o render.Op) bool { _, ok := o.(*render.Destroy); return ok }); len(d) != 1 {
		t.Fatalf("pop destroyed nothing: %v", h.last().Ops)
	}
}

func TestStackFollowsUserBack(t *testing.T) {
	h := start(t, navApp{})
	h.send(&render.Press{Tag: h.tagOf("open")})
	h.send(&render.Popped{Tag: h.tagOf("stack"), Depth: 1}) // the user swiped back
	if h.tagOf("detail") != 0 {
		t.Fatal("Go still has the screen the user left")
	}
	// The Children op now matches what the host already shows.
	ch := h.find(h.last(), func(o render.Op) bool { _, ok := o.(*render.Children); return ok })
	if len(ch) != 1 || len(ch[0].(*render.Children).Children) != 1 {
		t.Fatalf("children after back: %v", ch)
	}
}

func TestStackScreensTakeReportedSize(t *testing.T) {
	h := start(t, navApp{})
	h.send(&render.Resized{Tag: h.tagOf("stack"), W: 320, H: 380})
	f := h.find(h.last(), func(o render.Op) bool { x, ok := o.(*render.Frame); return ok && x.W == 320 && x.H == 380 })
	if len(f) != 1 || f[0].(*render.Frame).X != 0 || f[0].(*render.Frame).Y != 0 {
		t.Fatalf("screen frame after resize: %v", h.last().Ops)
	}
}

type plain struct{ got **Nav }

func (p plain) Render(c *Ctx) *Element {
	*p.got = Navigation(c)
	return View(Style{})
}

func TestNavigationOutsideAStack(t *testing.T) {
	var nav *Nav = &Nav{}
	start(t, plain{&nav})
	if nav != nil {
		t.Fatal("Navigation outside a Stack should be nil")
	}
	nav.Push("x", plain{&nav}) // must not panic
	nav.Pop()
}

// State below a skipped component must still render: here the counter lives
// in the stack's screen, below the root component whose props never change.
type countingScreen struct{}

func (countingScreen) Render(c *Ctx) *Element {
	n, setN := State(c, 0)
	return Pressable(func() { setN(n + 1) }, Style{Height: Pt(44)},
		Text(fmt.Sprint("taps ", n), Style{}).TestID("taps")).TestID("tap")
}

type deepApp struct{}

func (deepApp) Render(c *Ctx) *Element { return Stack("Home", countingScreen{}, Flex(1)) }

func TestStateBelowSkippedComponent(t *testing.T) {
	h := start(t, deepApp{})
	h.send(&render.Press{Tag: h.tagOf("tap")})
	h.send(&render.Press{Tag: h.tagOf("tap")})
	texts := h.all(func(o render.Op) bool { x, ok := o.(*render.Text); return ok && x.Text == "taps 2" })
	if len(texts) != 1 {
		t.Fatal("a state change below an unchanged component was not rendered")
	}
}
