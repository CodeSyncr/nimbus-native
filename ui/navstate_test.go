package ui

import (
	"path/filepath"
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

type pathOpener struct{ path string }

func (o pathOpener) Render(c *Ctx) Component {
	nav := Navigation(c)
	return Pressable(func() { nav.PushPath("Order", o.path, detailScreen{id: 7}) }, Style{Height: Pt(44)}).TestID("open-" + o.path)
}

type twoTabs struct{}

func (twoTabs) Render(c *Ctx) Component {
	return Tabs(Flex(1),
		Tab{Title: "Home", Screen: stackOf{"Home", homeScreen{}}},
		Tab{Title: "Orders", Screen: stackOf{"Orders", pathOpener{"/orders/7"}}},
	).TestID("tabs")
}

type stackOf struct {
	title string
	root  Component
}

func (s stackOf) Render(c *Ctx) Component { return Stack(s.title, s.root, Flex(1)) }

// A dev reload reopens the selected tab and the screens pushed by path.
func TestNavStateSurvivesReload(t *testing.T) {
	file := filepath.Join(t.TempDir(), "navstate")
	restored := []string{}
	RestoreRoute = func(path string) (string, Component, bool) {
		restored = append(restored, path)
		return "Order", detailScreen{id: 7}, path == "/orders/7"
	}
	defer func() { RestoreRoute = nil }()

	h := startWith(t, twoTabs{}, func(rt *runtime) { rt.loadNav(file) })
	h.send(&render.Selected{Tag: h.tagOf("tabs"), Index: 1})
	h.send(&render.Press{Tag: h.tagOf("open-/orders/7")})
	if h.tagOf("detail") == 0 {
		t.Fatal("the pushed screen is not showing")
	}

	// The next process: same app, reading what the first saved.
	h2 := startWith(t, twoTabs{}, func(rt *runtime) { rt.loadNav(file) })
	sel := h2.all(func(o render.Op) bool { s, ok := o.(*render.TabSelect); return ok && s.Index == 1 })
	if len(sel) == 0 {
		t.Error("the Orders tab was not selected again")
	}
	if h2.tagOf("detail") == 0 || len(restored) != 1 {
		t.Errorf("the order screen was not reopened (restored %v)", restored)
	}

	// Popping back is saved too: the next process opens the root.
	h2.send(&render.Popped{Tag: h2.rt.rootTagOf(t, "Orders"), Depth: 1})
	h3 := startWith(t, twoTabs{}, func(rt *runtime) { rt.loadNav(file) })
	if h3.tagOf("detail") != 0 {
		t.Error("a popped screen came back")
	}
}

// rootTagOf finds the Stack titled title.
func (rt *runtime) rootTagOf(t *testing.T, title string) uint32 {
	t.Helper()
	for tag, in := range rt.byTag {
		if in.stack != nil && in.el.stack.title == title {
			return tag
		}
	}
	t.Fatalf("no stack %q", title)
	return 0
}

// Without NIMBUS_DEV_STATE nothing is read or written.
func TestNavStateOffByDefault(t *testing.T) {
	h := start(t, twoTabs{})
	h.send(&render.Selected{Tag: h.tagOf("tabs"), Index: 1})
	if h.rt.navLast != "" {
		t.Fatal("navigation state kept outside dev mode")
	}
}
