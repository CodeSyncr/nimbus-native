package ui

import (
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// A new host view hierarchy gets every view again, with the state the UI
// had: the open screen, the selected tab.
func TestReattachReplaysTheUI(t *testing.T) {
	h := start(t, twoTabs{})
	h.send(&render.Selected{Tag: h.tagOf("tabs"), Index: 1})
	h.send(&render.Press{Tag: h.tagOf("open-/orders/7")})
	detail := h.tagOf("detail")
	if detail == 0 {
		t.Fatal("setup: no detail screen")
	}

	n := len(h.commits)
	h.send(&render.Reattach{Reason: "test"})
	var creates, roots int
	created := map[uint32]bool{}
	sel := -1
	for _, c := range h.commits[n:] {
		if c.Root != 0 {
			roots++
		}
		for _, op := range c.Ops {
			switch o := op.(type) {
			case *render.Create:
				creates++
				created[o.Tag] = true
			case *render.TabSelect:
				sel = int(o.Index)
			}
		}
	}
	if !created[detail] || creates < 10 {
		t.Fatalf("replay created %d views (detail screen: %v)", creates, created[detail])
	}
	if roots != 1 || sel != 1 {
		t.Fatalf("replay: %d roots, tab %d", roots, sel)
	}
	if h.tagOf("detail") != detail {
		t.Fatal("tags changed: the UI was rebuilt, not replayed")
	}
}
