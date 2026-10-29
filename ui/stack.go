package ui

import (
	"strconv"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// Stack is a navigation stack: screens shown one at a time under the
// platform's navigation bar, with its push and pop transitions and back
// gestures. root is the first screen. Components inside it push and pop
// screens through Navigation.
//
//	ui.Stack("Orders", OrdersScreen{}, ui.Flex(1))
func Stack(title string, root Component, s Style) *Element {
	return &Element{kind: render.KindStack, style: s, stack: &stackSpec{title: title, root: root}}
}

type stackSpec struct {
	title string
	root  Component
}

type stackState struct {
	screens []screen
	w, h    float32 // the size screens get, reported by the host
	nextKey int
}

type screen struct {
	title string
	comp  Component
	key   string
}

// Nav pushes and pops the screens of the Stack a component is in. Its
// methods are safe from any goroutine; changes show on the next render.
type Nav struct {
	rt *runtime
	in *instance
}

// Navigation is the Stack the component is shown in, or nil outside one.
func Navigation(c *Ctx) *Nav {
	for p := c.inst.parent; p != nil; p = p.parent {
		if p.el.stack != nil {
			return &Nav{rt: c.rt, in: p}
		}
	}
	return nil
}

// Push shows screen on top, titled title, with the platform's transition.
func (n *Nav) Push(title string, screenComp Component) {
	if n == nil {
		logf("ui: Navigation().Push outside a Stack")
		return
	}
	n.change(func(st *stackState) {
		st.nextKey++
		st.screens = append(st.screens, screen{title: title, comp: screenComp, key: "s" + strconv.Itoa(st.nextKey)})
	})
}

// Pop goes back one screen. The root screen stays.
func (n *Nav) Pop() {
	n.change(func(st *stackState) {
		if len(st.screens) > 1 {
			st.screens = st.screens[:len(st.screens)-1]
		}
	})
}

// PopToRoot goes back to the first screen.
func (n *Nav) PopToRoot() {
	n.change(func(st *stackState) { st.screens = st.screens[:1] })
}

// Depth is how many screens are on the stack.
func (n *Nav) Depth() int {
	if n == nil || n.in.stack == nil {
		return 0
	}
	return len(n.in.stack.screens)
}

func (n *Nav) change(f func(*stackState)) {
	if n == nil {
		return
	}
	rt, in := n.rt, n.in
	rt.do(func() {
		if in.unmounted || in.stack == nil {
			return
		}
		f(in.stack)
		rt.rechild[in] = true
		rt.dirty = true
	})
}

// reconcileStack renders a stack's screens: each a full-size view holding
// the screen's component, titled with a Screen op.
func (rt *runtime) reconcileStack(in *instance) {
	if in.stack == nil {
		in.stack = &stackState{screens: []screen{{title: in.el.stack.title, comp: in.el.stack.root, key: "s0"}}}
	}
	st := in.stack
	st.screens[0].comp = in.el.stack.root // the root screen follows the element's props
	w, h := st.w, st.h
	if w == 0 || h == 0 {
		// Until the host reports it: the viewport below the bar.
		bar := float32(44)
		if rt.viewport.Platform == "android" {
			bar = 56
		}
		w, h = rt.viewport.W, rt.viewport.H-rt.viewport.SafeTop-bar
	}
	bg := rt.theme().Background
	els := make([]*Element, len(st.screens))
	for i, sc := range st.screens {
		els[i] = View(Style{Absolute: true, Left: Pt(0), Top: Pt(0), Width: Pt(w), Height: Pt(h), Background: bg},
			C(sc.comp)).Key(sc.key)
	}
	rt.reconcileChildren(in, els)
	for i, c := range in.children {
		if r := hostRoot(c); r != nil && i < len(st.screens) && r.title != st.screens[i].title {
			r.title = st.screens[i].title
			rt.op(&render.Screen{Tag: r.tag, Title: r.title})
		}
	}
}

// popped follows a back gesture or button the host already animated.
func (rt *runtime) popped(in *instance, depth int) {
	st := in.stack
	if st == nil || depth < 1 || depth >= len(st.screens) {
		return
	}
	st.screens = st.screens[:depth]
	rt.rechild[in] = true
	rt.dirty = true
}

// resized gives a stack's screens or tabs' screens the size the host reports.
func (rt *runtime) resized(in *instance, w, h, bottom float32) {
	switch {
	case in.stack != nil:
		if in.stack.w == w && in.stack.h == h {
			return
		}
		in.stack.w, in.stack.h = w, h
	case in.tabs != nil:
		t := in.tabs
		if t.w == w && t.h == h && t.bottom == bottom {
			return
		}
		t.w, t.h, t.bottom = w, h, bottom
		rt.forceAll = true // SafeAreas in the tabs pad by the new bottom
	default:
		return
	}
	rt.rechild[in] = true
	rt.dirty = true
}
