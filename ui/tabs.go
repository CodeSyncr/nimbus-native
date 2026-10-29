package ui

import (
	"strconv"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// Tab is one tab of a Tabs bar.
type Tab struct {
	Title string
	// Icon is an SF Symbol name on iOS ("list.bullet"); on Android, the
	// drawable resource of the same name with dots as underscores
	// (res/drawable/list_bullet.xml). Without that resource Android shows
	// the title alone.
	Icon   string
	Screen Component
}

// Tabs is the platform's tab bar with a screen per tab: UITabBarController
// on iOS, a bottom navigation bar on Android. Every tab stays mounted, so
// switching keeps each tab's state, scroll position and navigation stack.
//
//	ui.Tabs(ui.Flex(1),
//		ui.Tab{Title: "Orders", Icon: "list.bullet", Screen: OrdersTab{}},
//		ui.Tab{Title: "Account", Icon: "person", Screen: AccountTab{}})
func Tabs(s Style, tabs ...Tab) *Element {
	return &Element{kind: render.KindTabs, style: s, tabs: &tabsSpec{tabs: tabs}}
}

type tabsSpec struct{ tabs []Tab }

type tabsState struct {
	selected int
	sent     int     // the selection the host last showed; -1 before the first
	w, h     float32 // the size tabs get, reported by the host
	bottom   float32 // how much of h the tab bar covers (iOS)
}

// TabsNav switches the Tabs a component is in. Safe from any goroutine.
type TabsNav struct {
	rt *runtime
	in *instance
}

// TabsOf is the Tabs the component is shown in, or nil outside one.
func TabsOf(c *Ctx) *TabsNav {
	for p := c.inst.parent; p != nil; p = p.parent {
		if p.el.tabs != nil {
			return &TabsNav{rt: c.rt, in: p}
		}
	}
	return nil
}

// Select shows the tab at index i.
func (t *TabsNav) Select(i int) {
	if t == nil {
		return
	}
	rt, in := t.rt, t.in
	rt.do(func() {
		if in.unmounted || in.tabs == nil || i < 0 || i >= len(in.el.tabs.tabs) {
			return
		}
		in.tabs.selected = i
		rt.rechild[in] = true
		rt.dirty = true
	})
}

// Selected is the index of the tab on screen.
func (t *TabsNav) Selected() int {
	if t == nil || t.in.tabs == nil {
		return 0
	}
	return t.in.tabs.selected
}

// reconcileTabs renders every tab's screen in a full-size view, and sends
// titles, icons and the selection when they change.
func (rt *runtime) reconcileTabs(in *instance) {
	if in.tabs == nil {
		in.tabs = &tabsState{sent: -1, selected: rt.savedTab()}
	}
	st, tabs := in.tabs, in.el.tabs.tabs
	if st.selected >= len(tabs) {
		st.selected = 0
	}
	w, h := st.w, st.h
	if w == 0 || h == 0 {
		// Until the host reports it: the whole viewport on iOS (tabs run
		// under the tab bar), the part above the bar on Android.
		w, h = rt.viewport.W, rt.viewport.H
		if rt.viewport.Platform == "android" {
			h -= 56
		}
	}
	bg := rt.theme().Background
	els := make([]*Element, len(tabs))
	for i, t := range tabs {
		els[i] = View(Style{Absolute: true, Left: Pt(0), Top: Pt(0), Width: Pt(w), Height: Pt(h), Background: bg},
			C(t.Screen)).Key("t" + strconv.Itoa(i))
	}
	rt.reconcileChildren(in, els)
	for i, c := range in.children {
		r := hostRoot(c)
		if r == nil || i >= len(tabs) {
			continue
		}
		if r.title != tabs[i].Title || r.icon != tabs[i].Icon {
			r.title, r.icon = tabs[i].Title, tabs[i].Icon
			rt.op(&render.TabItem{Tag: r.tag, Title: r.title, Icon: r.icon})
		}
	}
	if st.selected != st.sent {
		st.sent = st.selected
		rt.op(&render.TabSelect{Tag: in.tag, Index: uint32(st.selected)})
	}
}

// selected follows a tab the user picked; the host already shows it.
func (rt *runtime) selected(in *instance, i int) {
	st := in.tabs
	if st == nil || in.el.tabs == nil || i < 0 || i >= len(in.el.tabs.tabs) {
		return
	}
	st.selected, st.sent = i, i
	rt.saveNav() // no render follows: the host already switched
}

// tabsAround is the Tabs instance a component is in, or nil.
func tabsAround(c *Ctx) *instance {
	for p := c.inst.parent; p != nil; p = p.parent {
		if p.el.tabs != nil {
			return p
		}
	}
	return nil
}
