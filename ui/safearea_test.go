package ui

import (
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

type safeApp struct{ stacked bool }

func (a safeApp) Render(c *Ctx) Component {
	content := SafeArea(Style{Grow: 1, Padding: All(10)}, View(Style{Height: Pt(20)}).TestID("content"))
	if a.stacked {
		return Stack("Home", safeScreen{}, Flex(1))
	}
	return content
}

type safeScreen struct{}

func (safeScreen) Render(c *Ctx) Component {
	return SafeArea(Style{Grow: 1, Padding: All(10)}, View(Style{Height: Pt(20)}).TestID("content"))
}

func TestSafeAreaPadsInsets(t *testing.T) {
	h := start(t, safeApp{})
	h.send(&render.Viewport{W: 390, H: 844, SafeTop: 47, SafeBottom: 34, SafeLeft: 0, SafeRight: 0, Platform: "ios"})
	f := h.frameOf("content")
	if f == nil || f.Y != 47+10 || f.X != 10 || f.W != 390-20 {
		t.Fatalf("content at %+v, want x 10, y 57, w 370", f)
	}
}

func TestSafeAreaInStackSkipsTop(t *testing.T) {
	h := start(t, safeApp{stacked: true})
	h.send(&render.Viewport{W: 390, H: 844, SafeTop: 47, SafeBottom: 34, Platform: "ios"})
	if f := h.frameOf("content"); f == nil || f.Y != 10 {
		t.Fatalf("content at %+v, want y 10 (the bar covers the top inset)", f)
	}
}
