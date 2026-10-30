package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// What sits over the whole app, drawn in Go above the root (a Modal, which
// the platform presents, stays above these).

type overlayEntry struct {
	id   int64
	kind string
	show func(close func()) *Element
}

var (
	overlayList = NewStore(func() []overlayEntry { return nil })
	overlayIDs  atomic.Int64
)

// overlayRoot is the app and, above it, whatever overlays are open.
type overlayRoot struct{ app Component }

func (o overlayRoot) Render(c *Ctx) *Element {
	kids := []*Element{C(o.app).Key("app")}
	for _, e := range overlayList.Use(c) {
		id := e.id
		kids = append(kids, e.show(func() { removeOverlay(c, id) }).Key(fmt.Sprint(id)))
	}
	return View(Style{Grow: 1}, kids...)
}

func removeOverlay(c *Ctx, id int64) {
	overlayList.Update(c, func(l []overlayEntry) []overlayEntry {
		out := make([]overlayEntry, 0, len(l))
		for _, e := range l {
			if e.id != id {
				out = append(out, e)
			}
		}
		return out
	})
}

// OverlayNav shows things over the whole app. Safe from any goroutine.
type OverlayNav struct{ c *Ctx }

// Overlays shows toasts, sheets and menus from any component.
func Overlays(c *Ctx) OverlayNav { return OverlayNav{c} }

func (o OverlayNav) show(kind string, show func(close func()) *Element) {
	id := overlayIDs.Add(1)
	overlayList.Update(o.c, func(l []overlayEntry) []overlayEntry {
		out := make([]overlayEntry, 0, len(l)+1)
		for _, e := range l {
			if kind == "" || e.kind != kind { // a new toast replaces the last
				out = append(out, e)
			}
		}
		return append(out, overlayEntry{id, kind, show})
	})
}

// Dismiss closes every overlay at once.
func (o OverlayNav) Dismiss() {
	overlayList.Update(o.c, func([]overlayEntry) []overlayEntry { return nil })
}

// Toast shows a short message for a moment, without stopping anything:
//
//	ui.Overlays(c).Toast("Order placed", ui.Good)
func (o OverlayNav) Toast(message string, tone ...Tone) {
	t := Neutral
	if len(tone) > 0 {
		t = tone[0]
	}
	o.show("toast", func(close func()) *Element { return C(toastView{message, t, close}) })
}

type toastView struct {
	message string
	tone    Tone
	close   func()
}

func (t toastView) Render(c *Ctx) *Element {
	th, vp := UseTheme(c), Viewport(c)
	in, setIn := State(c, false)
	Effect(c, func(ctx context.Context) func() {
		go func() {
			for _, step := range []struct {
				after time.Duration
				f     func()
			}{{30 * time.Millisecond, func() { setIn(true) }}, {2400 * time.Millisecond, func() { setIn(false) }}, {300 * time.Millisecond, t.close}} {
				select {
				case <-ctx.Done():
					return
				case <-time.After(step.after):
					step.f()
				}
			}
		}()
		return nil
	}, Once)
	top, opacity := float32(vp.H), float32(0.01)
	if in {
		top, opacity = vp.H-vp.SafeBottom-84, 1
	}
	icon := map[Tone]string{Good: "check", Caution: "alert", Bad: "alert", Accent: "info"}[t.tone]
	kids := []*Element{}
	if icon != "" {
		kids = append(kids, Icon(icon, 19, t.tone.color(th)))
	}
	kids = append(kids, Text(t.message, Style{Shrink: 1, FontSize: 14.5, FontWeight: 500, Color: th.Background}))
	return View(Style{Absolute: true, Left: Pt(0), Right: Pt(0), Top: Pt(top), AlignItems: AlignCenter, Opacity: opacity, Transition: 280},
		View(Style{Direction: Row, AlignItems: AlignCenter, Gap: 9, MaxWidth: Pt(vp.W - 40), Padding: XY(18, 13), Radius: 999, Background: th.Text,
			Shadow: &Shadow{Color: Hex("#00000040"), Radius: 16, Y: 6}}, kids...))
}

// Sheet is a panel that rises from the bottom. Drag it down, or tap outside
// it, to close.
type Sheet struct {
	Title   string
	Content Component
	Height  float32 // a share of the screen; 0 means 0.5
}

// Sheet opens a sheet over the app:
//
//	ui.Overlays(c).Sheet(ui.Sheet{Title: "Filters", Content: Filters{}})
func (o OverlayNav) Sheet(s Sheet) {
	o.show("", func(close func()) *Element { return C(sheetView{s, close}) })
}

type sheetView struct {
	Sheet
	close func()
}

func (s sheetView) Render(c *Ctx) *Element {
	th, vp := UseTheme(c), Viewport(c)
	up, setUp := State(c, false)
	drag, setDrag := State(c, float32(0))
	dragging, setDragging := State(c, false)
	Effect(c, func(ctx context.Context) func() {
		go func() {
			select {
			case <-ctx.Done():
			case <-time.After(30 * time.Millisecond):
				setUp(true)
			}
		}()
		return nil
	}, Once)
	leave := func() {
		setDragging(false)
		setUp(false)
		go func() {
			time.Sleep(300 * time.Millisecond)
			s.close()
		}()
	}
	share := s.Height
	if share == 0 {
		share = 0.5
	}
	h := vp.H * share
	top, dim := vp.H, float32(0.01)
	if up {
		top, dim = vp.H-h+drag, 1
	}
	move := 300
	if dragging {
		move = 0
	}
	kids := []*Element{View(Style{AlignItems: AlignCenter, Padding: Edges{Top: 10, Bottom: 8}},
		View(Style{Width: Pt(38), Height: Pt(5), Radius: 2.5, Background: th.Border}))}
	if s.Title != "" {
		kids = append(kids, View(Style{Padding: Edges{Left: 20, Right: 20, Bottom: 10}}, Text(s.Title, Style{FontSize: th.Heading + 1, FontWeight: 600, Color: th.Text})))
	}
	grip := View(Style{}, kids...).OnPanY(func(p Pan) {
		switch p.Phase {
		case Began:
			setDragging(true)
		case Changed:
			setDrag(max(p.Y, 0))
		case Ended, Cancelled:
			if p.Y > h*0.3 || p.VY > 700 {
				leave()
				return
			}
			setDragging(false)
			setDrag(0)
		}
	})
	body := View(Style{Grow: 1, Padding: Edges{Left: 20, Right: 20, Bottom: vp.SafeBottom + 12}}, C(s.Content))
	return View(Style{Absolute: true, Left: Pt(0), Right: Pt(0), Top: Pt(0), Bottom: Pt(0)},
		Pressable(leave, Style{Absolute: true, Left: Pt(0), Right: Pt(0), Top: Pt(0), Bottom: Pt(0), Background: Hex("#00000059"), Opacity: dim, Transition: 300}).Label("Close"),
		View(Style{Absolute: true, Left: Pt(0), Right: Pt(0), Top: Pt(top), Height: Pt(h + 40), Radius: 22, Corners: CornersTop, Background: th.Surface, Transition: move,
			Shadow: &Shadow{Color: Hex("#00000030"), Radius: 24, Y: -4}}, grip, body))
}

// MenuAction is a choice in an ActionSheet or Menu.
type MenuAction struct {
	Label       string
	Icon        string
	Destructive bool
	OnPress     func()
}

// ActionSheet asks what to do, from a sheet of choices:
//
//	ui.Overlays(c).ActionSheet("Order 1042",
//		ui.MenuAction{Label: "Mark ready", Icon: "check", OnPress: ready},
//		ui.MenuAction{Label: "Cancel order", Icon: "trash", Destructive: true, OnPress: cancel})
func (o OverlayNav) ActionSheet(title string, actions ...MenuAction) {
	h := float32(len(actions)*54 + 90)
	if title != "" {
		h += 34
	}
	share := min(h/Viewport(o.c).H, 0.9)
	o.Sheet(Sheet{Title: title, Height: share, Content: actionList{actions, func() { o.closeTop() }}})
}

func (o OverlayNav) closeTop() {
	overlayList.Update(o.c, func(l []overlayEntry) []overlayEntry {
		if len(l) == 0 {
			return l
		}
		return l[:len(l)-1]
	})
}

type actionList struct {
	actions []MenuAction
	close   func()
}

func (a actionList) Render(c *Ctx) *Element {
	th := UseTheme(c)
	rows := make([]*Element, len(a.actions))
	for i, x := range a.actions {
		rows[i] = actionRow(th, x, a.close, 54).Key(x.Label)
	}
	return View(Style{}, rows...)
}

func actionRow(th Theme, x MenuAction, close func(), height float32) *Element {
	color := th.Text
	if x.Destructive {
		color = th.Danger
	}
	kids := []*Element{}
	if x.Icon != "" {
		kids = append(kids, Icon(x.Icon, 21, color))
	}
	kids = append(kids, Text(x.Label, Style{FontSize: th.Body + 1, FontWeight: 500, Color: color, Lines: 1}))
	return Pressable(func() {
		close()
		if x.OnPress != nil {
			x.OnPress()
		}
	}, Style{Height: Pt(height), Direction: Row, AlignItems: AlignCenter, Gap: 12}, kids...).Label(x.Label)
}

// viewFrame is where a view is in the window, asked of the host: it alone
// knows the scroll offsets and the native bars above it.
func viewFrame(ctx context.Context, tag uint32) (x, y, w, h float32, err error) {
	raw, err := plugin.CallJSON(ctx, "nimbusView", "frame", map[string]uint32{"tag": tag})
	if err != nil {
		return 0, 0, 0, 0, err
	}
	var f struct{ X, Y, W, H float32 }
	err = json.Unmarshal(raw, &f)
	return f.X, f.Y, f.W, f.H, err
}

// anchored finds this component's view and calls f with its frame, or with
// the screen's middle where the host can't say (the web).
func anchored(c *Ctx, f func(x, y, w, h float32)) {
	host := hostRoot(c.inst.child)
	vp := Viewport(c)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		x, y, w, h, err := float32(0), float32(0), float32(0), float32(0), error(nil)
		if host != nil {
			x, y, w, h, err = viewFrame(ctx, host.tag)
		}
		if host == nil || err != nil {
			x, y, w, h = vp.W/2-40, vp.H/3, 80, 0
		}
		f(x, y, w, h)
	}()
}

// Menu opens a small list of actions beside whatever is tapped:
//
//	ui.C(ui.Menu{Trigger: ui.Icon("more", 24, 0), Items: []ui.MenuAction{
//		{Label: "Edit", Icon: "edit", OnPress: edit},
//		{Label: "Delete", Icon: "trash", Destructive: true, OnPress: remove},
//	}})
type Menu struct {
	Trigger *Element
	Items   []MenuAction
	Label   string // what the trigger does, for screen readers
}

func (m Menu) Render(c *Ctx) *Element {
	label := m.Label
	if label == "" {
		label = "More"
	}
	return Pressable(func() {
		anchored(c, func(x, y, w, h float32) {
			Overlays(c).show("", func(close func()) *Element { return C(menuView{m.Items, x, y, w, h, close}) })
		})
	}, Style{AlignSelf: AlignStart}, m.Trigger).Label(label)
}

type menuView struct {
	items      []MenuAction
	x, y, w, h float32
	close      func()
}

func (m menuView) Render(c *Ctx) *Element {
	th, vp := UseTheme(c), Viewport(c)
	in, setIn := State(c, false)
	Effect(c, func(ctx context.Context) func() {
		go func() {
			select {
			case <-ctx.Done():
			case <-time.After(20 * time.Millisecond):
				setIn(true)
			}
		}()
		return nil
	}, Once)
	const width = 210
	height := float32(len(m.items)*46 + 12)
	left := min(max(m.x+m.w-width, 12), vp.W-width-12)
	top := m.y + m.h + 6
	if top+height > vp.H-vp.SafeBottom-12 {
		top = m.y - height - 6 // no room below: above it instead
	}
	opacity, scale := float32(0.01), float32(0.92)
	if in {
		opacity, scale = 1, 1
	}
	rows := make([]*Element, len(m.items))
	for i, x := range m.items {
		rows[i] = actionRow(th, x, m.close, 46).Key(x.Label)
	}
	return View(Style{Absolute: true, Left: Pt(0), Right: Pt(0), Top: Pt(0), Bottom: Pt(0)},
		Pressable(m.close, Style{Absolute: true, Left: Pt(0), Right: Pt(0), Top: Pt(0), Bottom: Pt(0)}).Label("Close"),
		View(Style{Absolute: true, Left: Pt(left), Top: Pt(top), Width: Pt(width), Padding: XY(14, 6), Radius: th.Radius + 2, Background: th.Surface,
			BorderWidth: 1, BorderColor: th.Border, Opacity: opacity, Scale: scale, Transition: 140, Shadow: &Shadow{Color: Hex("#00000030"), Radius: 22, Y: 8}}, rows...))
}

// Tooltip explains what its child is: tap it for a short note above it.
type Tooltip struct {
	Text  string
	Child *Element
}

func (t Tooltip) Render(c *Ctx) *Element {
	return Pressable(func() {
		anchored(c, func(x, y, w, h float32) {
			Overlays(c).show("toast", func(close func()) *Element { return C(tipView{t.Text, x, y, w, close}) })
		})
	}, Style{AlignSelf: AlignStart}, t.Child).Label(t.Text)
}

type tipView struct {
	text    string
	x, y, w float32
	close   func()
}

func (t tipView) Render(c *Ctx) *Element {
	th, vp := UseTheme(c), Viewport(c)
	Effect(c, func(ctx context.Context) func() {
		go func() {
			select {
			case <-ctx.Done():
			case <-time.After(2500 * time.Millisecond):
				t.close()
			}
		}()
		return nil
	}, Once)
	const width = 220
	left := min(max(t.x+t.w/2-width/2, 12), vp.W-width-12)
	return View(Style{Absolute: true, Left: Pt(left), Top: Pt(max(t.y-54, vp.SafeTop+4)), Width: Pt(width), AlignItems: AlignCenter},
		View(Style{Padding: XY(12, 8), Radius: 10, Background: th.Text},
			Text(t.text, Style{FontSize: 13, Color: th.Background, TextAlign: AlignCenter})))
}

// Banner is a message in the page that stays until it's dealt with:
//
//	ui.C(ui.Banner{Tone: ui.Caution, Title: "You're offline", Message: "Orders will send when you're back."})
type Banner struct {
	Title, Message string
	Tone           Tone
	Action         string
	OnAction       func()
	OnClose        func()
}

func (b Banner) Render(c *Ctx) *Element {
	th := UseTheme(c)
	color := b.Tone.color(th)
	icon := map[Tone]string{Good: "check", Caution: "alert", Bad: "alert"}[b.Tone]
	if icon == "" {
		icon = "info"
	}
	words := []*Element{Text(b.Title, Style{FontSize: th.Body, FontWeight: 600, Color: th.Text})}
	if b.Message != "" {
		words = append(words, Text(b.Message, Style{FontSize: th.Caption + 0.5, Color: th.TextMuted}))
	}
	if b.Action != "" {
		words = append(words, Pressable(b.OnAction, Style{Margin: Edges{Top: 4}, AlignSelf: AlignStart}, Text(b.Action, Style{FontSize: th.Caption + 0.5, FontWeight: 600, Color: color})).Label(b.Action))
	}
	kids := []*Element{Icon(icon, 21, color), View(Style{Grow: 1, Shrink: 1, Gap: 2}, words...)}
	if b.OnClose != nil {
		kids = append(kids, Pressable(b.OnClose, Style{}, Icon("close", 18, th.TextMuted)).Label("Dismiss"))
	}
	return View(Style{Direction: Row, Gap: 11, Padding: All(14), Radius: th.Radius, Background: fade(color, 0.1), BorderWidth: 1, BorderColor: fade(color, 0.25)}, kids...)
}
