package ui

import (
	"fmt"
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

var appRenders int

type feed struct{ varied bool } // comparable props, so scrolling can skip it

func (f feed) Render(c *Ctx) *Element {
	appRenders++
	return View(Style{},
		List(1000, func(i int) *Element {
			h := fixed(i)
			if f.varied {
				h = varied(i)
			}
			return View(Style{Height: Pt(h)}, Text(fmt.Sprint("row ", i), Style{})).TestID(fmt.Sprint("row", i))
		}, Flex(1)).TestID("list"),
	)
}

func fixed(i int) float32 { return 40 }

func (h *host) creates(c *render.Commit) int {
	return len(h.find(c, func(o render.Op) bool { _, ok := o.(*render.Create); return ok }))
}

func (h *host) frameOf(testID string) *render.Frame {
	tag := h.tagOf(testID)
	for i := len(h.commits) - 1; i >= 0; i-- {
		for _, o := range h.commits[i].Ops {
			if f, ok := o.(*render.Frame); ok && f.Tag == tag {
				return f
			}
		}
	}
	return nil
}

func TestListRendersOnlyTheWindow(t *testing.T) {
	h := start(t, feed{}) // viewport 320×480
	views := 0
	for _, c := range h.commits {
		views += h.creates(c)
	}
	// 480 pt visible + 480 below → 24 rows of 40 pt, two views each, plus
	// the root, the list and two spacers.
	if views > 2*24+4+2 {
		t.Fatalf("%d native views for a 1,000-row list", views)
	}
	if h.tagOf("row0") == 0 || h.tagOf("row23") == 0 || h.tagOf("row30") != 0 {
		t.Fatal("wrong rows rendered at the top")
	}
	sc := h.find(h.last(), func(o render.Op) bool { _, ok := o.(*render.Scroll); return ok })
	if len(sc) == 0 {
		for _, c := range h.commits {
			sc = append(sc, h.find(c, func(o render.Op) bool { _, ok := o.(*render.Scroll); return ok })...)
		}
	}
	if s := sc[len(sc)-1].(*render.Scroll); s.H != 1000*40 || !s.Report {
		t.Fatalf("content %v tall (want 40000), report %v", s.H, s.Report)
	}
}

func TestListScrollMovesTheWindow(t *testing.T) {
	h := start(t, feed{})
	appRenders = 0
	h.send(&render.Scrolled{Tag: h.tagOf("list"), Y: 20000, W: 320, H: 480})
	if appRenders != 0 {
		t.Fatalf("scrolling rendered the app component %d times", appRenders)
	}
	if h.tagOf("row0") != 0 || h.tagOf("row500") == 0 {
		t.Fatal("window did not move to row 500")
	}
	if f := h.frameOf("row500"); f == nil || f.Y != 500*40 {
		t.Fatalf("row 500 at %v, want y = 20000", f)
	}
	// A small scroll inside the rendered window renders nothing.
	n := len(h.commits)
	h.send(&render.Scrolled{Tag: h.tagOf("list"), Y: 20010, W: 320, H: 480})
	if len(h.commits) != n {
		t.Fatal("a 10 pt scroll caused a commit")
	}
}

func varied(i int) float32 { return float32(30 + (i%3)*20) } // 30, 50, 70, …

func TestListMeasuresVariedHeights(t *testing.T) {
	h := start(t, feed{varied: true})
	// Scroll down the way a finger does, so every row passes the window and
	// gets measured; then positions are exact.
	for y := float32(200); y <= 3000; y += 200 {
		h.send(&render.Scrolled{Tag: h.tagOf("list"), Y: y, W: 320, H: 480})
	}
	want := float32(0)
	for i := 0; i < 70; i++ {
		want += varied(i)
	}
	if f := h.frameOf("row70"); f == nil || f.Y != want || f.H != varied(70) {
		t.Fatalf("row 70 at %+v, want y = %v, h = %v", f, want, varied(70))
	}
}
