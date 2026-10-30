package ui

import (
	"fmt"
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

type settingsScreen struct{ locked bool }

func (s settingsScreen) Render(c *Ctx) Component {
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

func (refreshFeed) Render(c *Ctx) Component {
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

func (keyboardForm) Render(c *Ctx) Component {
	return SafeArea(Style{Grow: 1}, View(Style{Height: Pt(20)}).TestID("body"))
}

func TestSafeAreaClearsKeyboard(t *testing.T) {
	h := start(t, keyboardForm{})
	h.send(&render.Viewport{W: 320, H: 480, Scale: 2, Platform: "ios", SafeBottom: 34, Keyboard: 300})
	root := hostRoot(hostRoot(h.rt.rootInst).children[0]) // under the overlays' root
	if p := root.node.Style.Padding.Bottom; p != 300 {
		t.Fatalf("bottom padding %v, want the keyboard's 300", p)
	}
	h.send(&render.Viewport{W: 320, H: 480, Scale: 2, Platform: "ios", SafeBottom: 34})
	if p := root.node.Style.Padding.Bottom; p != 34 {
		t.Fatalf("bottom padding %v after the keyboard hid, want 34", p)
	}
}

type volumeScreen struct{}

func (volumeScreen) Render(c *Ctx) Component {
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

type dragged struct{ pans *[]Pan }

func (d dragged) Render(c *Ctx) Component {
	dir, setDir := State(c, Swipe(0))
	scale, setScale := State(c, float32(0))
	return View(Style{},
		View(Style{Height: Pt(44)}).OnPanX(func(p Pan) { *d.pans = append(*d.pans, p) }).TestID("row"),
		View(Style{Height: Pt(44)}).OnSwipe(setDir).TestID("deck"),
		View(Style{Height: Pt(44)}).OnPinch(func(p Pinch) { setScale(p.Scale) }).TestID("photo"),
		Text(fmt.Sprintf("%d %.1f", dir, scale), Style{}))
}

func TestGestures(t *testing.T) {
	var pans []Pan
	h := start(t, dragged{&pans})
	row := h.tagOf("row")
	box := h.all(func(o render.Op) bool { b, ok := o.(*render.Box); return ok && b.Tag == row })
	if len(box) == 0 || box[0].(*render.Box).Pan != render.PanX {
		t.Fatalf("row's box: %+v", box)
	}
	h.send(&render.Panned{Tag: row, Phase: render.GestureBegan, X: 9})
	h.send(&render.Panned{Tag: row, Phase: render.GestureEnded, X: 120, VX: 300})
	if len(pans) != 2 || pans[1].Phase != Ended || pans[1].X != 120 {
		t.Fatalf("pans: %+v", pans)
	}
	h.send(&render.Panned{Tag: h.tagOf("deck"), Phase: render.GestureEnded, X: -20, VX: -900})
	h.send(&render.Pinched{Tag: h.tagOf("photo"), Phase: render.GestureChanged, Scale: 1.5})
	if got := h.all(func(o render.Op) bool { x, ok := o.(*render.Text); return ok && x.Text == "1 1.5" }); len(got) == 0 {
		t.Fatal("swipe or pinch did not arrive")
	}
}

func TestSwipeOf(t *testing.T) {
	cases := []struct {
		p    Pan
		want Swipe
	}{
		{Pan{X: 10, VX: 900}, SwipeRight},
		{Pan{X: -10, VX: -900}, SwipeLeft},
		{Pan{Y: 200}, SwipeDown},
		{Pan{Y: -10, VY: -700}, SwipeUp},
		{Pan{X: 30, VX: 100}, 0},
		{Pan{}, 0},
	}
	for _, c := range cases {
		if got := swipeOf(c.p); got != c.want {
			t.Errorf("%+v: got %d", c.p, got)
		}
	}
}

type overlayApp struct{}

func (overlayApp) Render(c *Ctx) Component {
	n, setN := State(c, 0)
	return View(Style{Grow: 1},
		Pressable(func() { setN(n + 1); Overlays(c).Toast("Saved", Good) }, Style{}, Text(fmt.Sprint("taps ", n), Style{})).TestID("save"))
}

func TestOverlayKeepsAppState(t *testing.T) {
	h := start(t, overlayApp{})
	h.send(&render.Press{Tag: h.tagOf("save")})
	if got := h.all(func(o render.Op) bool { x, ok := o.(*render.Text); return ok && x.Text == "Saved" }); len(got) == 0 {
		t.Fatal("no toast")
	}
	h.send(&render.Press{Tag: h.tagOf("save")})
	if got := h.all(func(o render.Op) bool { x, ok := o.(*render.Text); return ok && x.Text == "taps 2" }); len(got) == 0 {
		t.Fatal("the app lost its state under the overlay")
	}
}

type componentsApp struct{}

func (componentsApp) Render(c *Ctx) Component {
	on, setOn := State(c, false)
	at, setAt := State(c, 0)
	return View(Style{Grow: 1},
		C(Button{Label: "Save", Icon: "check", OnPress: func() {}, Loading: true}),
		C(Checkbox{Label: "Receipts", Checked: on, OnChange: setOn}),
		C(SegmentedControl{Options: []string{"Day", "Week"}, Selected: at, OnChange: setAt}),
		C(Avatar{Name: "Asha Rao"}),
		C(Badge{Label: "Paid", Tone: Good}),
		C(ListRow{Title: "Orders", Icon: "cart", OnPress: func() {}}),
		C(ProgressBar{Value: 0.4}),
		C(EmptyState{Title: "Nothing yet"}),
		C(Grid{Items: []Component{Text("a", Style{}), Text("b", Style{}), Text("c", Style{})}}),
		C(Accordion{Items: []AccordionItem{{"One", Text("inside", Style{})}, {"Two", Text("hidden", Style{})}}}),
		C(SwipeRow{Content: Text("row", Style{}), Actions: []SwipeAction{{Label: "Delete", Icon: "trash", Tone: Bad}}}),
		C(Carousel{Pages: []Component{Text("p1", Style{})}, Height: 100}),
		C(Banner{Title: "Offline", Tone: Caution}),
		C(AppBar{Title: "Orders", Large: true}))
}

func TestComponentsRender(t *testing.T) {
	h := start(t, componentsApp{})
	for _, want := range []string{"Save", "Receipts", "AR", "Paid", "Orders", "Nothing yet", "inside", "row", "p1", "Offline"} {
		if got := h.all(func(o render.Op) bool { x, ok := o.(*render.Text); return ok && x.Text == want }); len(got) == 0 {
			t.Errorf("%q did not render", want)
		}
	}
	if got := h.all(func(o render.Op) bool { x, ok := o.(*render.Text); return ok && x.Text == "hidden" }); len(got) != 0 {
		t.Error("a closed accordion item rendered")
	}
}

type label struct{ text string }

func (l label) Render(c *Ctx) Component { return Text(l.text, Style{}) }

type directChildren struct{}

func (directChildren) Render(c *Ctx) Component {
	var none *Element // a nil child, of either kind, draws nothing
	return View(Style{},
		Text("plain", Style{}),
		label{"a component"},
		none,
		nil,
		Key("row", label{"keyed"}),
		View(Style{}, label{"nested"}),
		C(C(label{"wrapped"})))
}

func TestChildrenTakeComponents(t *testing.T) {
	h := start(t, directChildren{})
	for _, want := range []string{"plain", "a component", "keyed", "nested", "wrapped"} {
		if got := h.all(func(o render.Op) bool { x, ok := o.(*render.Text); return ok && x.Text == want }); len(got) == 0 {
			t.Errorf("%q did not render", want)
		}
	}
}

func TestListTakesComponents(t *testing.T) {
	h := start(t, listOfComponents{})
	if got := h.all(func(o render.Op) bool { x, ok := o.(*render.Text); return ok && x.Text == "row 1" }); len(got) == 0 {
		t.Fatal("no list row")
	}
}

type listOfComponents struct{}

func (listOfComponents) Render(c *Ctx) Component {
	return List(3, func(i int) Component { return label{fmt.Sprint("row ", i)} }, Flex(1))
}

type returnsComponent struct{}

func (returnsComponent) Render(c *Ctx) Component { return label{"from a component"} }

type returnsNil struct{}

func (returnsNil) Render(c *Ctx) Component { return nil }

type card struct{}

func (card) Render(c *Ctx) Component { return View(Style{Height: Pt(40)}, Text("card", Style{})) }

type wrappedGestures struct {
	swipes *[]Swipe
	holds  *int
}

func (w wrappedGestures) Render(c *Ctx) Component {
	return View(Style{},
		returnsComponent{},
		returnsNil{},
		TestID("named", card{}),
		C(card{}).OnSwipe(func(s Swipe) { *w.swipes = append(*w.swipes, s) }).TestID("swipe"),
		Pressable(func() {}, Style{}, Text("p", Style{})),
		C(pressCard{}).OnLongPress(func() { *w.holds++ }).TestID("hold"))
}

type pressCard struct{}

func (pressCard) Render(c *Ctx) Component { return Pressable(func() {}, Style{Height: Pt(40)}) }

func TestWrapperReachesTheView(t *testing.T) {
	var swipes []Swipe
	holds := 0
	h := start(t, wrappedGestures{&swipes, &holds})
	if got := h.all(func(o render.Op) bool { x, ok := o.(*render.Text); return ok && x.Text == "from a component" }); len(got) == 0 {
		t.Fatal("a component returned from Render did not draw")
	}
	h.tagOf("named")
	tag := h.tagOf("swipe")
	box := h.all(func(o render.Op) bool { b, ok := o.(*render.Box); return ok && b.Tag == tag })
	if len(box) == 0 || box[0].(*render.Box).Pan == 0 {
		t.Fatalf("the wrapper's swipe never reached the view: %+v", box)
	}
	h.send(&render.Panned{Tag: tag, Phase: render.GestureEnded, X: -200, VX: -900})
	if len(swipes) != 1 || swipes[0] != SwipeLeft {
		t.Fatalf("swipes: %v", swipes)
	}
	h.send(&render.LongPressed{Tag: h.tagOf("hold")})
	if holds != 1 {
		t.Fatalf("long presses: %d", holds)
	}
}
