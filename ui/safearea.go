package ui

// SafeArea is a View padded by the screen's safe-area insets — the notch or
// status bar, the home indicator, rounded corners — on top of any padding
// in s, so its children stay clear of them. Inside a Stack the navigation
// bar already covers the top, so only the other edges are padded.
//
//	ui.SafeArea(ui.Style{Padding: ui.All(16)}, ui.Text("Hello", ui.Style{}))
func SafeArea(s Style, children ...*Element) *Element {
	return C(safeArea{style: s, children: children})
}

type safeArea struct {
	style    Style
	children []*Element
}

func (a safeArea) Render(c *Ctx) *Element {
	vp := Viewport(c)
	s := a.style
	top := vp.SafeTop
	if inStack(c) {
		top = 0
	}
	s.Padding.Top += top
	s.Padding.Bottom += vp.SafeBottom
	s.Padding.Left += vp.SafeLeft
	s.Padding.Right += vp.SafeRight
	return View(s, a.children...)
}

func inStack(c *Ctx) bool {
	for p := c.inst.parent; p != nil; p = p.parent {
		if p.el.stack != nil {
			return true
		}
	}
	return false
}
