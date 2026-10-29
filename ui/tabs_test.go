package ui

import (
	"fmt"
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

type tabsApp struct{}

func (tabsApp) Render(c *Ctx) *Element {
	return Tabs(Flex(1),
		Tab{Title: "Orders", Icon: "list.bullet", Screen: tabCounter{}},
		Tab{Title: "Account", Icon: "person", Screen: accountTab{}},
	).TestID("tabs")
}

type tabCounter struct{}

func (tabCounter) Render(c *Ctx) *Element {
	n, setN := State(c, 0)
	tabs := TabsOf(c)
	return View(Style{},
		Pressable(func() { setN(n + 1) }, Style{Height: Pt(44)}, Text(fmt.Sprint("count ", n), Style{})).TestID("inc"),
		Pressable(func() { tabs.Select(1) }, Style{Height: Pt(44)}).TestID("go-account"),
	)
}

type accountTab struct{}

func (accountTab) Render(c *Ctx) *Element { return Text("account", Style{}).TestID("account") }

func TestTabs(t *testing.T) {
	h := start(t, tabsApp{})
	items := h.all(func(o render.Op) bool { _, ok := o.(*render.TabItem); return ok })
	if len(items) != 2 || items[0].(*render.TabItem).Title != "Orders" || items[1].(*render.TabItem).Icon != "person" {
		t.Fatalf("tab items: %v", items)
	}
	sel := h.all(func(o render.Op) bool { _, ok := o.(*render.TabSelect); return ok })
	if len(sel) != 1 || sel[0].(*render.TabSelect).Index != 0 {
		t.Fatalf("initial selection: %v", sel)
	}
	if h.tagOf("account") == 0 {
		t.Fatal("every tab should be mounted")
	}

	h.send(&render.Press{Tag: h.tagOf("inc")})
	h.send(&render.Press{Tag: h.tagOf("go-account")})
	if s := h.find(h.last(), func(o render.Op) bool { x, ok := o.(*render.TabSelect); return ok && x.Index == 1 }); len(s) != 1 {
		t.Fatalf("Select(1) sent %v", h.last().Ops)
	}

	// The user taps the first tab: Go follows without echoing a TabSelect.
	n := len(h.commits)
	h.send(&render.Selected{Tag: h.tagOf("tabs"), Index: 0})
	for _, c := range h.commits[n:] {
		if s := h.find(c, func(o render.Op) bool { _, ok := o.(*render.TabSelect); return ok }); len(s) != 0 {
			t.Fatal("a user's tab switch was echoed back")
		}
	}
	// State survived the switches.
	if texts := h.all(func(o render.Op) bool { x, ok := o.(*render.Text); return ok && x.Text == "count 1" }); len(texts) != 1 {
		t.Fatal("tab state lost")
	}
}
