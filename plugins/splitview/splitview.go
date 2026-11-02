// Package splitview shows a list beside what's open from it when there's
// room, as on a tablet, and one at a time on a phone: picking from the list
// pushes the detail onto the screen's Stack. On a foldable that's half open
// with a vertical fold, the panes meet at the fold. It's plain Go, so it
// needs no plugin add; add hinge for the fold.
//
//	split := splitview.Use(c)
//	return splitview.View{
//		Split:  split,
//		List:   ordersList(split.Open), // its rows call split.Open(id)
//		Detail: func(id string) ui.Component { return OrderScreen{ID: id} },
//		Title:  func(id string) string { return "Order " + id },
//		Empty:  ui.Text("Pick an order", ui.Style{}),
//		Style:  ui.Style{Grow: 1},
//	}
//
// Put it inside a ui.Stack, which a phone needs for the detail to push onto.
package splitview

import (
	"github.com/CodeSyncr/nimbus-native/plugins/hinge"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Split is what's open, and the way to open something.
type Split struct {
	Selected string // the ID open in the detail pane; "" for none
	st       *state
}

type state struct {
	set  func(string)
	wide bool
	push func(id string)
}

// Use keeps what's open for a View in this component.
//
//nimbus:param c the component that shows the View
func Use(c *ui.Ctx) Split {
	sel, set := ui.State(c, "")
	st := ui.Ref(c, state{})
	st.set = set
	return Split{Selected: sel, st: st}
}

// Open shows id: in the detail pane side by side, or pushed onto the Stack
// on a phone.
//
//nimbus:param id what to open, as Detail and Title take it
func (s Split) Open(id string) {
	if s.st == nil {
		return
	}
	s.st.set(id)
	if !s.st.wide && s.st.push != nil {
		s.st.push(id)
	}
}

// View is the list and the detail.
type View struct {
	Split     Split                        // from Use
	List      ui.Component                 // its rows call Split.Open
	Detail    func(id string) ui.Component // what's open
	Title     func(id string) string       // the pushed detail's title on a phone; nil for none
	Empty     ui.Component                 // the detail pane with nothing open; nil for blank
	ListWidth float32                      // the list's width side by side; 0 means 320
	MinWidth  float32                      // the window width from which they go side by side; 0 means 700
	Style     ui.Style                     // its size and place; usually Grow: 1
}

// panes says how to lay out a window w points wide with fold f: side by
// side or not, the list's width, and the gap between them (the fold's).
func (v View) panes(w float32, f hinge.Fold) (wide bool, list, gap float32) {
	if f.Present && f.Vertical && f.Separates && f.X > 0 && f.X < w {
		return true, f.X, f.W
	}
	min, list := v.MinWidth, v.ListWidth
	if min <= 0 {
		min = 700
	}
	if list <= 0 {
		list = 320
	}
	return w >= min, list, 0
}

// Render lays them out.
func (v View) Render(c *ui.Ctx) ui.Component {
	th := ui.UseTheme(c)
	w := ui.Viewport(c).W
	nav := ui.Navigation(c)
	return hinge.Aware{Style: v.Style, Content: func(f hinge.Fold) ui.Component {
		wide, list, gap := v.panes(w, f)
		if st := v.Split.st; st != nil {
			st.wide = wide
			st.push = func(id string) {
				if nav == nil || v.Detail == nil {
					return
				}
				title := ""
				if v.Title != nil {
					title = v.Title(id)
				}
				nav.Push(title, v.Detail(id))
			}
		}
		if !wide {
			return ui.View(ui.Style{Grow: 1}, v.List)
		}
		var detail ui.Component = ui.View(ui.Style{})
		switch {
		case v.Split.Selected != "" && v.Detail != nil:
			detail = v.Detail(v.Split.Selected)
		case v.Empty != nil:
			detail = ui.View(ui.Style{Grow: 1, AlignItems: ui.AlignCenter, Justify: ui.JustifyCenter}, v.Empty)
		}
		var line ui.Component = ui.View(ui.Style{Width: ui.Pt(1), Background: th.Border})
		if gap > 0 {
			line = ui.View(ui.Style{Width: ui.Pt(gap)}) // the fold itself divides them
		}
		return ui.View(ui.Style{Direction: ui.Row, Grow: 1},
			ui.View(ui.Style{Width: ui.Pt(list)}, v.List),
			line,
			ui.View(ui.Style{Grow: 1, Basis: ui.Pt(0)}, detail))
	}}
}
