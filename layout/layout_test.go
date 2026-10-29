package layout

import (
	"math"
	"testing"
)

var nan = float32(math.NaN())

// fakeText measures like wrapping text: 8 pt per character, 16 pt per line.
func fakeText(chars int, calls *int) MeasureFunc {
	return func(w float32, wm Mode, h float32, hm Mode) (float32, float32) {
		if calls != nil {
			*calls++
		}
		full := float32(chars * 8)
		if wm == Undefined || full <= w {
			return full, 16
		}
		perLine := int(w / 8)
		if perLine < 1 {
			perLine = 1
		}
		lines := (chars + perLine - 1) / perLine
		return float32(perLine * 8), float32(lines * 16)
	}
}

func TestMeasuredTextWraps(t *testing.T) {
	text := &Node{Measure: fakeText(30, nil), Style: Style{Padding: All(4)}} // 240 pt on one line
	root := &Node{Style: Style{Width: Pt(100), Height: Pt(300), AlignItems: AlignStart}, Children: []*Node{text}}
	root.Calculate(nan, nan)
	// 92 pt of room → 11 chars a line → 3 lines.
	if text.W != 88+8 || text.H != 48+8 {
		t.Fatalf("text %v×%v, want 96×56", text.W, text.H)
	}
}

func TestMeasuredTextInRow(t *testing.T) {
	icon := &Node{Style: Style{Width: Pt(24), Height: Pt(24)}}
	label := &Node{Measure: fakeText(40, nil), Style: Style{Grow: 1, Shrink: 1}}
	row := &Node{Style: Style{Direction: Row, Gap: 8}, Children: []*Node{icon, label}}
	root := &Node{Style: Style{Width: Pt(200), Height: Pt(400)}, Children: []*Node{row}}
	root.Calculate(nan, nan)
	// The label shrinks to 168 pt (200 - 24 - 8) and wraps to 2 lines.
	if label.X != 32 || label.W != 168 || label.H != 32 || row.H != 32 {
		t.Fatalf("label at %v %v×%v, row height %v", label.X, label.W, label.H, row.H)
	}
}

func TestScrollContent(t *testing.T) {
	var rows []*Node
	for i := 0; i < 50; i++ {
		rows = append(rows, &Node{Style: Style{Height: Pt(40)}})
	}
	scroll := &Node{Style: Style{Grow: 1, Shrink: 1, Basis: Pt(0), Scroll: true, Padding: Edges{Top: 10, Bottom: 10}}, Children: rows}
	root := &Node{Style: Style{Width: Pt(320), Height: Pt(480)}, Children: []*Node{scroll}}
	root.Calculate(nan, nan)
	if scroll.H != 480 || scroll.ContentH != 50*40+20 || rows[49].Y != 10+49*40 || rows[0].W != 320 {
		t.Fatalf("scroll %v, content %v, last row y %v, row width %v", scroll.H, scroll.ContentH, rows[49].Y, rows[0].W)
	}
}

func TestHiddenTakesNoSpace(t *testing.T) {
	a := &Node{Style: Style{Height: Pt(10)}}
	b := &Node{Style: Style{Height: Pt(10), Hidden: true}}
	c := &Node{Style: Style{Height: Pt(10)}}
	root := &Node{Style: Style{Width: Pt(100), Height: Pt(100), Gap: 5}, Children: []*Node{a, b, c}}
	root.Calculate(nan, nan)
	if c.Y != 15 || b.W != 0 {
		t.Fatalf("c.y %v, hidden width %v", c.Y, b.W)
	}
}

// screen builds a 1,000-row list like the demo app: each row is a row
// container with an image, a two-line text column and a badge.
func screen(calls *int) *Node {
	var rows []*Node
	for i := 0; i < 1000; i++ {
		title := &Node{Measure: fakeText(12+i%9, calls)}
		sub := &Node{Measure: fakeText(30, calls)}
		texts := &Node{Style: Style{Grow: 1, Shrink: 1, Gap: 2}, Children: []*Node{title, sub}}
		rows = append(rows, &Node{
			Style: Style{Direction: Row, AlignItems: AlignCenter, Padding: Edges{12, 8, 12, 8}, Gap: 12},
			Children: []*Node{
				{Style: Style{Width: Pt(40), Height: Pt(40)}},
				texts,
				{Style: Style{Padding: All(4)}, Children: []*Node{{Measure: fakeText(3, calls)}}},
			},
		})
	}
	return &Node{Style: Style{Width: Pt(390), Height: Pt(844)}, Children: []*Node{
		{Style: Style{Height: Pt(56)}},
		{Style: Style{Grow: 1, Shrink: 1, Basis: Pt(0), Scroll: true}, Children: rows},
	}}
}

// Measurements are cached per pass: each text is measured a bounded number
// of times however deep the tree, not once per ancestor.
func TestMeasureCalls(t *testing.T) {
	calls := 0
	screen(&calls).Calculate(nan, nan)
	if per := float64(calls) / 3000; per > 4 {
		t.Fatalf("%d measure calls for 3,000 texts (%.1f each)", calls, per)
	}
	t.Logf("%d measure calls for 3,000 texts", calls)
}

func BenchmarkScreen1000Rows(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		screen(nil).Calculate(nan, nan) // a fresh tree: full layout
	}
}

func rects(n *Node, out *[][4]float32) {
	*out = append(*out, [4]float32{n.X, n.Y, n.W, n.H})
	for _, c := range n.Children {
		rects(c, out)
	}
}

func TestIncremental(t *testing.T) {
	calls := 0
	root := screen(&calls)
	root.Calculate(nan, nan)
	calls = 0
	root.Calculate(nan, nan)
	if calls != 0 {
		t.Fatalf("an unchanged tree was measured again (%d calls)", calls)
	}

	// Change one title deep in the list, and the header's height.
	list := root.Children[1]
	title := list.Children[500].Children[1].Children[0]
	title.Measure = fakeText(60, &calls) // now wraps
	title.MarkDirty()
	header := root.Children[0]
	header.Style.Height = Pt(80)
	header.MarkDirty()
	root.Calculate(nan, nan)
	if calls == 0 || calls > 12 {
		t.Fatalf("%d measure calls after changing one text; want a handful", calls)
	}

	// The incremental result must equal a from-scratch layout of the same tree.
	var got, want [][4]float32
	rects(root, &got)
	fresh := screen(nil)
	fresh.Children[1].Children[500].Children[1].Children[0].Measure = fakeText(60, nil)
	fresh.Children[0].Style.Height = Pt(80)
	fresh.Calculate(nan, nan)
	rects(fresh, &want)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("node %d: incremental %v, fresh %v", i, got[i], want[i])
		}
	}

	// The root has a fixed size, so a new viewport changes nothing; a root
	// that fills the viewport follows it.
	root.Calculate(320, 600)
	if root.W != 390 || list.H != 844-80 {
		t.Fatalf("fixed root: %v wide, list %v tall", root.W, list.H)
	}
	root.Style.Width, root.Style.Height = Value{}, Value{}
	root.MarkDirty()
	root.Calculate(320, 600)
	if root.W != 320 || list.H != 600-80 || list.Children[0].W != 320 {
		t.Fatalf("filling root: %v wide, list %v tall, row %v wide", root.W, list.H, list.Children[0].W)
	}
}

func BenchmarkScreenOneChange(b *testing.B) {
	root := screen(nil)
	root.Calculate(nan, nan)
	header := root.Children[0]
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		header.Style.Height = Pt(float32(50 + i%10))
		header.MarkDirty()
		root.Calculate(nan, nan)
	}
}
