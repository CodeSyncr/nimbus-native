package ui

import (
	"strconv"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// List shows count items in a vertical scroll view, but only creates native
// views for the items on screen and about one screen above and below. Items
// may differ in height: each is measured once it has been shown.
//
//	ui.List(len(orders), func(i int) *ui.Element {
//		return ui.C(OrderRow{orders[i]})
//	}, ui.Flex(1)).ItemKey(func(i int) string { return orders[i].ID })
//
// Scrolling renders no components but the items coming into view.
func List(count int, item func(i int) *Element, s Style) *Element {
	return &Element{kind: render.KindScroll, style: scrollFill(s), list: &listSpec{count: count, item: item}}
}

// ItemKey names a List's items, so an item keeps its native views and
// measured height when the data changes order. Without it, items are keyed
// by index.
func (e *Element) ItemKey(key func(i int) string) *Element {
	if e.list != nil {
		e.list.key = key
	}
	return e
}

// EstimatedItemHeight is the height a List assumes for items it has not
// shown yet (default: the average of the items it has shown).
func (e *Element) EstimatedItemHeight(h float32) *Element {
	if e.list != nil {
		e.list.estimate = h
	}
	return e
}

type listSpec struct {
	count    int
	item     func(i int) *Element
	key      func(i int) string
	estimate float32
}

// listState is a mounted List: where it is scrolled to and what it knows
// about its items' heights.
type listState struct {
	offset, viewH float32
	heights       map[string]float32
	first, last   int // the rendered items, last exclusive
	keys          []string

	renderedTop, renderedBottom float32 // the content rows the rendered items cover
}

const (
	listTopKey    = "\x00top"
	listBottomKey = "\x00bottom"
	defaultItem   = 60 // the height assumed before any item has been measured
)

func (s *listSpec) keyOf(i int) string {
	if s.key != nil {
		return s.key(i)
	}
	return strconv.Itoa(i)
}

func (ls *listState) estimate(s *listSpec) float32 {
	if s.estimate > 0 {
		return s.estimate
	}
	if len(ls.heights) == 0 {
		return defaultItem
	}
	var sum float32
	for _, h := range ls.heights {
		sum += h
	}
	return sum / float32(len(ls.heights))
}

func (ls *listState) heightOf(s *listSpec, i int, est float32) float32 {
	if h, ok := ls.heights[s.keyOf(i)]; ok {
		return h
	}
	return est
}

// window is the range of items to render — the visible ones plus one view
// height above and below — and the space taken by the items before and
// after it.
func (ls *listState) window(s *listSpec, viewH float32) (first, last int, top, bottom float32) {
	if ls.viewH > 0 {
		viewH = ls.viewH
	}
	lo, hi := ls.offset-viewH, ls.offset+2*viewH
	est := ls.estimate(s)
	var y float32
	first, last = s.count, s.count
	for i := 0; i < s.count; i++ {
		h := ls.heightOf(s, i, est)
		switch {
		case y+h <= lo:
			top += h
		case y < hi:
			if first == s.count {
				first = i
			}
			last = i + 1
		default:
			bottom += h
		}
		y += h
	}
	if first == s.count {
		first, last = s.count, s.count
	}
	return first, last, top, bottom
}

// reconcileList renders the list's current window as its children: a spacer
// for the items above, the visible items, a spacer for the items below.
func (rt *runtime) reconcileList(in *instance) {
	spec := in.el.list
	if in.list == nil {
		in.list = &listState{heights: map[string]float32{}}
		rt.lists[in] = true
	}
	ls := in.list
	first, last, top, bottom := ls.window(spec, rt.viewport.H)
	if rt.hostLayout != nil {
		first, last, top, bottom = 0, spec.count, 0, 0 // the host scrolls and lays out every item
	}
	els := make([]*Element, 0, last-first+2)
	els = append(els, View(Style{Height: Pt(top)}).Key(listTopKey))
	ls.keys = ls.keys[:0]
	for i := first; i < last; i++ {
		el := spec.item(i)
		if el == nil {
			continue
		}
		k := spec.keyOf(i)
		ls.keys = append(ls.keys, k)
		els = append(els, el.Key(k))
	}
	els = append(els, View(Style{Height: Pt(bottom)}).Key(listBottomKey))
	ls.first, ls.last = first, last
	est := ls.estimate(spec)
	ls.renderedTop, ls.renderedBottom = top, top
	for i := first; i < last; i++ {
		ls.renderedBottom += ls.heightOf(spec, i, est)
	}
	rt.reconcileChildren(in, els)
}

// measureLists runs after layout: it records the heights of the items just
// laid out and the list's own height, and reports whether any list must
// render again because its window or spacers are now wrong.
func (rt *runtime) measureLists() bool {
	again := false
	for in := range rt.lists {
		ls := in.list
		changed := false
		if h := in.node.H; h > 0 && h != ls.viewH {
			ls.viewH, changed = h, true
		}
		items := in.children
		if len(items) >= 2 {
			items = items[1 : len(items)-1] // between the spacers
		}
		for i, c := range items {
			h := hostRoot(c)
			if h == nil || i >= len(ls.keys) {
				continue
			}
			n := h.node
			height := n.H + n.Style.Margin.Top + n.Style.Margin.Bottom
			if old, ok := ls.heights[ls.keys[i]]; !ok || abs32(old-height) > 0.5 {
				ls.heights[ls.keys[i]] = height
				changed = true
			}
		}
		if changed {
			// New heights move the spacers, and may change the window.
			rt.rechild[in] = true
			again = true
		}
	}
	return again
}

// scrolled moves a list's window. It renders again only when the screen
// comes within half a screen of the rendered items' edge (or the end of the
// list), so scrolling a few rows costs nothing.
func (rt *runtime) scrolled(in *instance, m *render.Scrolled) {
	if f := in.el.onScroll; f != nil {
		f(m.X, m.Y)
	}
	ls := in.list
	if ls == nil {
		return
	}
	ls.offset = m.Y
	if m.H > 0 {
		ls.viewH = m.H
	}
	viewH := ls.viewH
	if viewH == 0 {
		viewH = rt.viewport.H
	}
	spec := in.el.list
	covered := (ls.first == 0 || ls.offset-viewH/2 >= ls.renderedTop) &&
		(ls.last == spec.count || ls.offset+viewH*1.5 <= ls.renderedBottom)
	if covered {
		return
	}
	first, last, _, _ := ls.window(spec, rt.viewport.H)
	if first != ls.first || last != ls.last {
		rt.rechild[in] = true
		rt.dirty = true
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
