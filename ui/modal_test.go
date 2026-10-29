package ui

import (
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

type modalApp struct{}

func (modalApp) Render(c *Ctx) *Element {
	m := Modals(c)
	return View(Style{},
		Pressable(func() { m.Present("New order", newOrder{}) }, Style{Height: Pt(44)}).TestID("open"),
	)
}

type newOrder struct{}

func (newOrder) Render(c *Ctx) *Element {
	nav, m := Navigation(c), Modals(c)
	return View(Style{},
		Text("form", Style{}).TestID("form"),
		Pressable(func() { nav.Push("Step 2", step2{}) }, Style{Height: Pt(44)}).TestID("next"),
		Pressable(func() { m.Dismiss() }, Style{Height: Pt(44)}).TestID("done"),
	)
}

type step2 struct{}

func (step2) Render(c *Ctx) *Element { return Text("step 2", Style{}).TestID("step2") }

func (h *host) modalTag() uint32 {
	for tag, in := range h.rt.byTag {
		if in.el.kind == render.KindModal {
			return tag
		}
	}
	return 0
}

func TestModalPresentNavigateDismiss(t *testing.T) {
	h := start(t, modalApp{})
	h.send(&render.Press{Tag: h.tagOf("open")})
	tag := h.modalTag()
	if tag == 0 || h.tagOf("form") == 0 {
		t.Fatal("modal not rendered")
	}
	if p := h.find(h.last(), func(o render.Op) bool { x, ok := o.(*render.Present); return ok && x.Tag == tag }); len(p) != 1 {
		t.Fatalf("no Present op: %v", h.last().Ops)
	}
	if s := h.all(func(o render.Op) bool { x, ok := o.(*render.Screen); return ok && x.Title == "New order" }); len(s) != 1 {
		t.Fatal("the modal's stack has no title")
	}

	h.send(&render.Press{Tag: h.tagOf("next")}) // navigation inside the modal
	if h.tagOf("step2") == 0 {
		t.Fatal("Navigation inside a modal did not push")
	}

	h.send(&render.Resized{Tag: tag, W: 390, H: 790})
	if f := h.frameOf("form"); f == nil {
		t.Fatal("form lost after resize")
	}
	if f := h.find(h.last(), func(o render.Op) bool { x, ok := o.(*render.Frame); return ok && x.Tag == tag && x.H == 790 }); len(f) != 1 {
		t.Fatalf("modal not resized: %v", h.last().Ops)
	}

	h.send(&render.Press{Tag: h.tagOf("done")})
	if h.modalTag() != 0 {
		t.Fatal("Dismiss left the modal")
	}
	d := h.find(h.last(), func(o render.Op) bool { _, ok := o.(*render.Destroy); return ok })
	if len(d) != 1 || !containsTag(d[0].(*render.Destroy).Tags, tag) {
		t.Fatalf("dismiss did not destroy the modal: %v", d)
	}
}

func TestModalSwipedAway(t *testing.T) {
	h := start(t, modalApp{})
	h.send(&render.Press{Tag: h.tagOf("open")})
	tag := h.modalTag()
	h.send(&render.Dismissed{Tag: tag})
	if h.modalTag() != 0 || h.tagOf("form") != 0 {
		t.Fatal("Go kept a modal the user swiped away")
	}
	// Opening again presents a fresh one.
	h.send(&render.Press{Tag: h.tagOf("open")})
	if h.modalTag() == 0 || h.modalTag() == tag {
		t.Fatal("could not present again")
	}
}

func containsTag(tags []uint32, t uint32) bool {
	for _, x := range tags {
		if x == t {
			return true
		}
	}
	return false
}
