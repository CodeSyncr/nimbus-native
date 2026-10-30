package ui

import (
	"fmt"
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

type settingsScreen struct{ locked bool }

func (s settingsScreen) Render(c *Ctx) *Element {
	on, setOn := State(c, false)
	change := setOn
	if s.locked {
		change = func(bool) {} // refuses every change
	}
	return View(Style{}, Switch(on, change, Style{}).TestID("notify"), Text(map[bool]string{true: "on", false: "off"}[on], Style{}).TestID("state"))
}

func lastSwitch(h *host, tag uint32) *render.Switch {
	ops := h.all(func(o render.Op) bool { x, ok := o.(*render.Switch); return ok && x.Tag == tag })
	if len(ops) == 0 {
		return nil
	}
	return ops[len(ops)-1].(*render.Switch)
}

func TestSwitch(t *testing.T) {
	h := start(t, settingsScreen{})
	tag := h.tagOf("notify")
	if sw := lastSwitch(h, tag); sw == nil || sw.On || !sw.Enabled || sw.Tint == 0 {
		t.Fatalf("initial switch: %+v", sw)
	}
	n := len(h.commits)
	h.send(&render.Toggled{Tag: tag, On: true})
	texts := h.all(func(o render.Op) bool { x, ok := o.(*render.Text); return ok && x.Text == "on" })
	if len(texts) == 0 {
		t.Fatal("toggling did not update the state")
	}
	// The host already shows "on": no Switch op is needed for it.
	for _, c := range h.commits[n:] {
		if len(h.find(c, func(o render.Op) bool { _, ok := o.(*render.Switch); return ok })) > 0 {
			t.Fatal("echoed the host's own change back to it")
		}
	}

	// A handler that keeps the old value flips the switch back.
	h2 := start(t, settingsScreen{locked: true})
	tag2 := h2.tagOf("notify")
	h2.send(&render.Toggled{Tag: tag2, On: true})
	if sw := lastSwitch(h2, tag2); sw == nil || sw.On {
		t.Fatalf("refused toggle not undone: %+v", sw)
	}
}

type refreshFeed struct{}

func (refreshFeed) Render(c *Ctx) *Element {
	loading, setLoading := State(c, false)
	return Scroll(Flex(1), Text("item", Style{})).OnRefresh(loading, func() { setLoading(true) }).TestID("feed")
}

func lastScroll(h *host, tag uint32) *render.Scroll {
	ops := h.all(func(o render.Op) bool { x, ok := o.(*render.Scroll); return ok && x.Tag == tag })
	if len(ops) == 0 {
		return nil
	}
	return ops[len(ops)-1].(*render.Scroll)
}

func TestPullToRefresh(t *testing.T) {
	h := start(t, refreshFeed{})
	tag := h.tagOf("feed")
	if sc := lastScroll(h, tag); sc == nil || !sc.Refreshable || sc.Refreshing {
		t.Fatalf("initial scroll: %+v", sc)
	}
	h.send(&render.Refresh{Tag: tag})
	// Go now says refreshing, which the host already shows: nothing to send.
	if sc := lastScroll(h, tag); sc.Refreshing {
		t.Fatal("echoed the host's own spinner back")
	}
}

type keyboardForm struct{}

func (keyboardForm) Render(c *Ctx) *Element {
	return SafeArea(Style{Grow: 1}, View(Style{Height: Pt(20)}).TestID("body"))
}

func TestSafeAreaClearsKeyboard(t *testing.T) {
	h := start(t, keyboardForm{})
	h.send(&render.Viewport{W: 320, H: 480, Scale: 2, Platform: "ios", SafeBottom: 34, Keyboard: 300})
	root := hostRoot(h.rt.rootInst)
	if p := root.node.Style.Padding.Bottom; p != 300 {
		t.Fatalf("bottom padding %v, want the keyboard's 300", p)
	}
	h.send(&render.Viewport{W: 320, H: 480, Scale: 2, Platform: "ios", SafeBottom: 34})
	if p := root.node.Style.Padding.Bottom; p != 34 {
		t.Fatalf("bottom padding %v after the keyboard hid, want 34", p)
	}
}

type volumeScreen struct{}

func (volumeScreen) Render(c *Ctx) *Element {
	v, setV := State(c, float32(0.25))
	n, setN := State(c, 0)
	return View(Style{},
		Slider(v, 0, 1, setV, Style{}).TestID("volume"),
		Text(fmt.Sprintf("%.2f %d", v, n), Style{}).TestID("value"),
		Pressable(func() {}, Style{Height: Pt(44), Transition: 200}).OnLongPress(func() { setN(n + 1) }).TestID("hold"),
	)
}

func TestSliderLongPressTransition(t *testing.T) {
	h := start(t, volumeScreen{})
	sl := h.all(func(o render.Op) bool { _, ok := o.(*render.Slider); return ok })
	if len(sl) == 0 || sl[0].(*render.Slider).Max != 1 || !sl[0].(*render.Slider).Enabled {
		t.Fatalf("slider op: %v", sl)
	}
	box := h.all(func(o render.Op) bool { b, ok := o.(*render.Box); return ok && b.LongPress && b.Transition == 200 })
	if len(box) == 0 {
		t.Fatal("the pressable's box has no long press or transition")
	}
	h.send(&render.Slid{Tag: h.tagOf("volume"), Value: 0.75})
	h.send(&render.LongPressed{Tag: h.tagOf("hold")})
	if got := h.all(func(o render.Op) bool { x, ok := o.(*render.Text); return ok && x.Text == "0.75 1" }); len(got) == 0 {
		t.Fatal("slide and long press did not reach their handlers")
	}
}
