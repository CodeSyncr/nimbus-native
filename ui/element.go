package ui

import (
	"reflect"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// Element describes one piece of UI: a native view, or a component that
// renders to one. Elements are cheap values, rebuilt on every render; the
// runtime compares them with what is on screen and changes only what differs.
type Element struct {
	kind     render.Kind
	comp     Component
	key      string
	style    Style
	children []*Element

	text        string
	src         string
	fit         render.Fit
	placeholder string
	secure      bool
	keyboard    render.Keyboard
	horizontal  bool
	label       string
	testID      string
	onPress     func()
	onChange    func(string)
	multiline   bool
	on          bool       // a Switch's state
	onToggle    func(bool) // a Switch's handler; nil disables it
	onRefresh   func()     // pull to refresh on a Scroll or List
	refreshing  bool
	list        *listSpec
	stack       *stackSpec
	tabs        *tabsSpec
}

// Component is a reusable piece of UI. Its fields are its props; Render runs
// on every render and may call hooks (State, Effect, Memo, Ref) on c.
//
//	type Counter struct{ Start int }
//
//	func (p Counter) Render(c *ui.Ctx) *ui.Element {
//		n, setN := ui.State(c, p.Start)
//		return ui.Pressable(func() { setN(n + 1) }, ui.Style{},
//			ui.Text(fmt.Sprint(n), ui.Style{}))
//	}
type Component interface {
	Render(c *Ctx) *Element
}

// C places a component in the tree.
func C(comp Component) *Element { return &Element{comp: comp} }

// View is a container: a flex column unless its style says otherwise.
func View(s Style, children ...*Element) *Element {
	return &Element{kind: render.KindView, style: s, children: children}
}

// Text shows a string.
func Text(text string, s Style) *Element {
	return &Element{kind: render.KindText, style: s, text: text}
}

// Image shows a picture from a URL. Give it a size: it has none of its own.
func Image(url string, s Style) *Element {
	return &Element{kind: render.KindImage, style: s, src: url}
}

// Pressable is a container that calls onPress when tapped, dimming while
// pressed.
func Pressable(onPress func(), s Style, children ...*Element) *Element {
	return &Element{kind: render.KindPressable, style: s, children: children, onPress: onPress}
}

// Scroll scrolls its children vertically (Horizontal() for sideways). Give
// it a size or Flex: its content is as long as its children.
func Scroll(s Style, children ...*Element) *Element {
	return &Element{kind: render.KindScroll, style: s, children: children}
}

// TextInput is a single-line text field showing value; onChange gets every
// edit. Keep value in State and set it from onChange.
func TextInput(value string, onChange func(string), s Style) *Element {
	return &Element{kind: render.KindInput, style: s, text: value, onChange: onChange}
}

// Spinner is the platform's activity indicator.
func Spinner(s Style) *Element { return &Element{kind: render.KindSpinner, style: s} }

// Switch is the platform's on/off toggle, colored with the theme's Primary
// (or s.Color). onChange gets the new state; pass nil to disable it.
//
//	notify, setNotify := ui.State(c, true)
//	ui.Switch(notify, setNotify, ui.Style{})
func Switch(on bool, onChange func(bool), s Style) *Element {
	return &Element{kind: render.KindSwitch, style: s, on: on, onToggle: onChange}
}

// Key identifies an element among its siblings, so the runtime can keep its
// native view and state when the list changes order.
func (e *Element) Key(k string) *Element { e.key = k; return e }

// Placeholder sets a text input's hint.
func (e *Element) Placeholder(p string) *Element { e.placeholder = p; return e }

// Secure hides a text input's characters.
func (e *Element) Secure() *Element { e.secure = true; return e }

// Keyboard picks a text input's soft keyboard.
func (e *Element) Keyboard(k render.Keyboard) *Element { e.keyboard = k; return e }

// Fit sets how an image fills its frame.
func (e *Element) Fit(f render.Fit) *Element { e.fit = f; return e }

// Horizontal makes a Scroll scroll sideways.
func (e *Element) Horizontal() *Element { e.horizontal = true; return e }

// Multiline lets a TextInput wrap and take new lines; it is three lines
// tall unless its style sets a height.
func (e *Element) Multiline() *Element { e.multiline = true; return e }

// OnRefresh adds pull to refresh to a Scroll or List: pulling calls
// onRefresh, and the indicator shows while refreshing is true. With a query:
//
//	ui.List(n, row, ui.Flex(1)).OnRefresh(q.Loading, q.Refetch)
func (e *Element) OnRefresh(refreshing bool, onRefresh func()) *Element {
	e.refreshing, e.onRefresh = refreshing, onRefresh
	return e
}

// Label is the accessibility label screen readers speak. On a component, it
// labels what the component renders.
func (e *Element) Label(l string) *Element { e.label = l; return e }

// TestID names the element for UI tests (accessibilityIdentifier on iOS,
// the view tag on Android). On a component, it names what the component
// renders.
func (e *Element) TestID(id string) *Element { e.testID = id; return e }

// sameType reports whether e can update an instance made from o.
func (e *Element) sameType(o *Element) bool {
	if (e.comp == nil) != (o.comp == nil) {
		return false
	}
	if e.comp != nil {
		return reflect.TypeOf(e.comp) == reflect.TypeOf(o.comp)
	}
	return e.kind == o.kind && e.horizontal == o.horizontal && (e.list == nil) == (o.list == nil) &&
		(e.stack == nil) == (o.stack == nil) && (e.tabs == nil) == (o.tabs == nil)
}
