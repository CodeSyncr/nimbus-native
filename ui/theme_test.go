package ui

import (
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

type plainText struct{}

func (plainText) Render(c *Ctx) *Element {
	th := UseTheme(c)
	return View(Style{Background: th.Surface}, Text("hello", Style{}).TestID("t")).TestID("card")
}

func (h *host) lastText(tag uint32) *render.Text {
	for i := len(h.commits) - 1; i >= 0; i-- {
		for _, o := range h.commits[i].Ops {
			if x, ok := o.(*render.Text); ok && x.Tag == tag {
				return x
			}
		}
	}
	return nil
}

func TestThemeFollowsDarkMode(t *testing.T) {
	h := start(t, plainText{})
	ch := h.all(func(o render.Op) bool { _, ok := o.(*render.Chrome); return ok })
	if len(ch) != 1 || ch[0].(*render.Chrome).Dark || ch[0].(*render.Chrome).Bar != uint32(LightTheme.Bar) {
		t.Fatalf("light chrome: %v", ch)
	}
	if x := h.lastText(h.tagOf("t")); x == nil || x.Color != uint32(LightTheme.Text) {
		t.Fatalf("unset text color should be the theme's: %+v", x)
	}

	h.send(&render.Viewport{W: 320, H: 480, Platform: "ios", Dark: true})
	c := h.last()
	if x := h.find(c, func(o render.Op) bool { x, ok := o.(*render.Chrome); return ok && x.Dark }); len(x) != 1 {
		t.Fatalf("no dark chrome: %v", c.Ops)
	}
	if x := h.lastText(h.tagOf("t")); x.Color != uint32(DarkTheme.Text) {
		t.Fatalf("text did not switch to dark: %#x", x.Color)
	}
	boxes := h.find(c, func(o render.Op) bool { x, ok := o.(*render.Box); return ok && x.Tag == h.tagOf("card") })
	if len(boxes) != 1 || boxes[0].(*render.Box).Background != uint32(DarkTheme.Surface) {
		t.Fatalf("UseTheme component did not re-render dark: %v", boxes)
	}
}

func TestSetTheme(t *testing.T) {
	defer SetTheme(LightTheme, DarkTheme)
	brand := LightTheme
	brand.Text = Hex("#123456")
	SetTheme(brand, DarkTheme)
	h := start(t, plainText{})
	if x := h.lastText(h.tagOf("t")); x.Color != uint32(Hex("#123456")) {
		t.Fatalf("custom theme not used: %#x", x.Color)
	}
}
