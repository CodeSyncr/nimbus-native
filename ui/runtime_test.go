package ui

import (
	"context"
	"fmt"
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// host is a fake native host: it records commits and measures text at 8 pt
// per character, 20 pt per line.
type host struct {
	rt       *runtime
	commits  []*render.Commit
	measures int
}

func start(t *testing.T, root Component) *host {
	t.Helper()
	return startWith(t, root, nil)
}

// startWith is start with a chance to set up the runtime before it renders.
func startWith(t *testing.T, root Component, setup func(*runtime)) *host {
	t.Helper()
	h := &host{rt: newRuntime(root)}
	if setup != nil {
		setup(h.rt)
	}
	h.rt.send = func(b []byte) {
		m, err := render.Decode(b)
		if err != nil {
			t.Fatalf("undecodable commit: %v", err)
		}
		h.commits = append(h.commits, m.(*render.Commit))
	}
	h.rt.measure = func(m *render.MeasureText) render.TextSize {
		h.measures++
		w := float32(len(m.Text) * 8)
		if m.MaxWidth >= 0 && w > m.MaxWidth {
			lines := int(w/m.MaxWidth) + 1
			return render.TextSize{W: m.MaxWidth, H: float32(20 * lines)}
		}
		return render.TextSize{W: w, H: 20}
	}
	h.send(&render.Viewport{W: 320, H: 480, Scale: 2, Platform: "ios"})
	return h
}

func (h *host) send(m render.Message) {
	h.rt.receive(render.Encode(m))
	h.rt.drain()
}

func (h *host) last() *render.Commit { return h.commits[len(h.commits)-1] }

// rerender renders every component again, as after data changed outside
// any component's state.
func (h *host) rerender() {
	h.rt.do(func() { h.rt.dirty, h.rt.forceAll = true, true })
	h.rt.drain()
}

func (h *host) find(c *render.Commit, pred func(render.Op) bool) []render.Op {
	var out []render.Op
	for _, o := range c.Ops {
		if pred(o) {
			out = append(out, o)
		}
	}
	return out
}

func (h *host) tagOf(testID string) uint32 {
	for tag, in := range h.rt.byTag {
		if in.el.testID == testID {
			return tag
		}
	}
	return 0
}

type counter struct{}

func (counter) Render(c *Ctx) *Element {
	n, setN := State(c, 0)
	return View(Style{Padding: All(10)},
		Text(fmt.Sprintf("Tapped %d times", n), Style{}).TestID("label"),
		Pressable(func() { setN(n + 1) }, Style{Height: Pt(44)}).TestID("button"),
	)
}

func TestFirstRenderAndTap(t *testing.T) {
	h := start(t, counter{})
	c := h.last()
	if c.Root == 0 {
		t.Fatal("first commit names no root")
	}
	creates := h.find(c, func(o render.Op) bool { _, ok := o.(*render.Create); return ok })
	if len(creates) != 4 { // with the overlays' root
		t.Fatalf("%d creates, want 4", len(creates))
	}

	h.send(&render.Press{Tag: h.tagOf("button")})
	c = h.last()
	texts := h.find(c, func(o render.Op) bool { _, ok := o.(*render.Text); return ok })
	if len(texts) != 1 || texts[0].(*render.Text).Text != "Tapped 1 times" {
		t.Fatalf("after a tap: %v", c.Ops)
	}
	if len(h.find(c, func(o render.Op) bool { _, ok := o.(*render.Create); return ok })) != 0 {
		t.Fatal("a tap re-created views")
	}
	// Same text width ("0" → "1"): only the text op, no frame changes.
	if len(c.Ops) != 1 {
		t.Fatalf("tap commit has %d ops, want 1: %v", len(c.Ops), c.Ops)
	}
}

type list struct{ items *[]string }

func (l list) Render(c *Ctx) *Element {
	var rows []Component
	for _, it := range *l.items {
		rows = append(rows, C(row{it}).Key(it))
	}
	return Scroll(Flex(1), rows...)
}

type row struct{ name string }

func (r row) Render(c *Ctx) *Element {
	mounted := Ref(c, 0)
	Effect(c, func(ctx context.Context) func() {
		*mounted++
		return func() { cleanups[r.name]++ }
	}, Once)
	return View(Style{Height: Pt(40)}, Text(r.name, Style{})).TestID(r.name)
}

var cleanups = map[string]int{}

func TestKeyedReorderKeepsViews(t *testing.T) {
	items := []string{"a", "b", "c"}
	h := start(t, list{&items})
	tags := map[string]uint32{"a": h.tagOf("a"), "b": h.tagOf("b"), "c": h.tagOf("c")}

	items = []string{"c", "a"} // reorder and drop b
	h.rerender()
	c := h.last()

	if len(h.find(c, func(o render.Op) bool { _, ok := o.(*render.Create); return ok })) != 0 {
		t.Fatal("reorder created views")
	}
	ch := h.find(c, func(o render.Op) bool { _, ok := o.(*render.Children); return ok })
	if len(ch) != 1 || fmt.Sprint(ch[0].(*render.Children).Children) != fmt.Sprint([]uint32{tags["c"], tags["a"]}) {
		t.Fatalf("children op %v, want [c a] = %v", ch, []uint32{tags["c"], tags["a"]})
	}
	d := h.find(c, func(o render.Op) bool { _, ok := o.(*render.Destroy); return ok })
	if len(d) != 1 || len(d[0].(*render.Destroy).Tags) != 2 { // b's row and its text
		t.Fatalf("destroy %v", d)
	}
	if cleanups["b"] != 1 || cleanups["a"] != 0 {
		t.Fatalf("effect cleanups %v: want b cleaned up once, a untouched", cleanups)
	}
	// Moved rows get new frames: c moved up.
	frames := h.find(c, func(o render.Op) bool { f, ok := o.(*render.Frame); return ok && f.Tag == tags["c"] })
	if len(frames) != 1 || frames[0].(*render.Frame).Y != 0 {
		t.Fatalf("c's frame: %v", frames)
	}
}

type wrapped struct{ text *string }

func (w wrapped) Render(c *Ctx) *Element {
	return View(Style{Width: Pt(100), AlignItems: AlignStart}, Text(*w.text, Style{}))
}

func TestTextMeasuredOncePerChange(t *testing.T) {
	text := "a long line of text that wraps"
	h := start(t, wrapped{&text})
	first := h.measures
	if first == 0 {
		t.Fatal("text never measured")
	}
	h.rerender()
	if h.measures != first {
		t.Fatalf("re-render re-measured unchanged text (%d → %d calls)", first, h.measures)
	}
	text = "short"
	h.rerender()
	if h.measures == first {
		t.Fatal("changed text not measured")
	}
	f := h.find(h.last(), func(o render.Op) bool { f, ok := o.(*render.Frame); return ok && f.W == 40 })
	if len(f) != 1 {
		t.Fatalf("short text frame not updated to 40 wide: %v", h.last().Ops)
	}
}

type panicky struct{}

func (panicky) Render(c *Ctx) *Element { panic("boom") }

func TestPanickingComponentRendersNothing(t *testing.T) {
	logf = func(string, ...any) {}
	h := start(t, panicky{})
	creates := h.find(h.last(), func(o render.Op) bool { _, ok := o.(*render.Create); return ok })
	if len(h.commits) != 1 || len(creates) != 1 { // only the overlays' root
		t.Fatalf("commits %v, %d creates", h.commits, len(creates))
	}
}

var rowRenders int

type countedRow struct{ id int }

func (r countedRow) Render(c *Ctx) *Element {
	rowRenders++
	return View(Style{Height: Pt(40)}, Text(fmt.Sprint(r.id), Style{}))
}

type page struct{}

func (page) Render(c *Ctx) *Element {
	n, setN := State(c, 0)
	rows := []Component{Pressable(func() { setN(n + 1) }, Style{}, Text(fmt.Sprint(n), Style{})).TestID("button")}
	for i := 0; i < 100; i++ {
		rows = append(rows, C(countedRow{i}).Key(fmt.Sprint(i)))
	}
	return View(Style{}, rows...)
}

func TestUnchangedComponentsSkipRender(t *testing.T) {
	rowRenders = 0
	h := start(t, page{})
	if rowRenders != 100 {
		t.Fatalf("first render rendered %d rows", rowRenders)
	}
	rowRenders = 0
	h.send(&render.Press{Tag: h.tagOf("button")})
	if rowRenders != 0 {
		t.Fatalf("a tap re-rendered %d unchanged rows", rowRenders)
	}
	if got := h.last().Ops; len(got) != 1 {
		t.Fatalf("tap commit: %v", got)
	}
	h.send(&render.Viewport{W: 400, H: 480, Scale: 2, Platform: "ios"})
	if rowRenders != 100 {
		t.Fatalf("a viewport change re-rendered %d rows, want all 100", rowRenders)
	}
}
