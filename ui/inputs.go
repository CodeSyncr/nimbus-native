package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugins/dialog"
	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// Checkbox is a tick box with a label:
//
//	ui.C(ui.Checkbox{Label: "Send receipts", Checked: on, OnChange: setOn})
type Checkbox struct {
	Label    string
	Checked  bool
	OnChange func(bool)
	Disabled bool
}

func (b Checkbox) Render(c *Ctx) Component {
	th := UseTheme(c)
	box := Style{Width: Pt(24), Height: Pt(24), Radius: 7, BorderWidth: 1.5, BorderColor: fade(th.Text, 0.35), Justify: JustifyCenter, AlignItems: AlignCenter, Transition: 140}
	var tick Component
	if b.Checked {
		box.Background, box.BorderColor = th.Primary, th.Primary
		tick = Icon("check", 17, th.OnPrimary)
	}
	return pickRow(th, b.Label, b.Disabled, func() { b.OnChange(!b.Checked) }, View(box, tick))
}

// pickRow is a mark and its label, tappable as one.
func pickRow(th Theme, label string, disabled bool, onPress func(), mark *Element) *Element {
	s := Style{Direction: Row, AlignItems: AlignCenter, Gap: 12, Padding: XY(0, 8)}
	if disabled {
		s.Opacity, onPress = 0.45, nil
	}
	return Pressable(onPress, s, mark, Text(label, Style{FontSize: th.Body, Color: th.Text, Shrink: 1})).Label(label)
}

// RadioGroup picks one of its options:
//
//	ui.C(ui.RadioGroup{Options: []string{"Card", "UPI", "Cash"}, Selected: pay, OnChange: setPay})
type RadioGroup struct {
	Options  []string
	Selected int
	OnChange func(int)
	Disabled bool
}

func (r RadioGroup) Render(c *Ctx) Component {
	th := UseTheme(c)
	rows := make([]Component, len(r.Options))
	for i, o := range r.Options {
		ring := Style{Width: Pt(24), Height: Pt(24), Radius: 12, BorderWidth: 1.5, BorderColor: fade(th.Text, 0.35), Justify: JustifyCenter, AlignItems: AlignCenter, Transition: 140}
		dot := Style{Width: Pt(12), Height: Pt(12), Radius: 6, Scale: 0.01, Background: th.Primary, Transition: 140}
		if i == r.Selected {
			ring.BorderColor, dot.Scale = th.Primary, 1
		}
		rows[i] = pickRow(th, o, r.Disabled, func() { r.OnChange(i) }, View(ring, View(dot))).Key(o)
	}
	return View(Style{}, rows...)
}

// SegmentedControl picks one of a few views of the same thing, with a thumb
// that slides to the selection:
//
//	ui.C(ui.SegmentedControl{Options: []string{"Day", "Week", "Month"}, Selected: at, OnChange: setAt})
type SegmentedControl struct {
	Options  []string
	Selected int
	OnChange func(int)
}

func (g SegmentedControl) Render(c *Ctx) Component {
	th := UseTheme(c)
	share := 100 / float32(max(len(g.Options), 1))
	track := th.Border
	thumb := th.Surface
	if th.Dark {
		thumb = th.Background
	}
	kids := []Component{View(Style{Absolute: true, Top: Pt(0), Bottom: Pt(0), Left: Pct(share * float32(g.Selected)), Width: Pct(share),
		Radius: th.Radius - 3, Background: thumb, Transition: 220, Shadow: &Shadow{Color: Hex("#00000022"), Radius: 4, Y: 1}})}
	for i, o := range g.Options {
		weight, color := uint16(500), th.TextMuted
		if i == g.Selected {
			weight, color = 600, th.Text
		}
		kids = append(kids, Pressable(func() { g.OnChange(i) }, Style{Grow: 1, Basis: Pt(0), Justify: JustifyCenter, AlignItems: AlignCenter},
			Text(o, Style{FontSize: 13.5, FontWeight: weight, Color: color, Lines: 1})).Label(o))
	}
	return View(Style{Height: Pt(40), Padding: All(3), Radius: th.Radius, Background: track},
		View(Style{Grow: 1, Direction: Row}, kids...))
}

// Stepper changes a number a step at a time:
//
//	ui.C(ui.Stepper{Value: qty, Min: 1, Max: 9, OnChange: setQty})
type Stepper struct {
	Value, Min, Max int // Max 0 has no limit
	Step            int // 0 means 1
	OnChange        func(int)
}

func (s Stepper) Render(c *Ctx) Component {
	th := UseTheme(c)
	step := max(s.Step, 1)
	press := func(icon, label string, to int, ok bool) *Element {
		st := Style{Width: Pt(44), Height: Pt(40), Justify: JustifyCenter, AlignItems: AlignCenter}
		var f func()
		if ok {
			f = func() { s.OnChange(to) }
		} else {
			st.Opacity = 0.35
		}
		return Pressable(f, st, Icon(icon, 18, th.Text)).Label(label)
	}
	return View(Style{Direction: Row, AlignItems: AlignCenter, AlignSelf: AlignStart, Radius: th.Radius, BorderWidth: 1, BorderColor: th.Border},
		press("minus", "Less", s.Value-step, s.Value-step >= s.Min),
		View(Style{MinWidth: Pt(38), AlignItems: AlignCenter},
			Text(fmt.Sprint(s.Value), Style{FontSize: 16, FontWeight: 600, Color: th.Text, TabularDigits: true})),
		press("plus", "More", s.Value+step, s.Max == 0 || s.Value+step <= s.Max))
}

// fieldBox is the look of a one-line field.
func fieldBox(th Theme) Style {
	return Style{Height: Pt(48), Padding: XY(14, 0), Radius: th.Radius, Background: th.Surface, BorderWidth: 1, BorderColor: th.Border,
		Direction: Row, AlignItems: AlignCenter, Gap: 10}
}

// SearchBar is a one-line search box with a clear button:
//
//	ui.C(ui.SearchBar{Value: q, OnChange: setQ, OnSubmit: search})
type SearchBar struct {
	Value       string
	Placeholder string // "Search" when empty
	OnChange    func(string)
	OnSubmit    func()
}

func (b SearchBar) Render(c *Ctx) Component {
	th := UseTheme(c)
	hint := b.Placeholder
	if hint == "" {
		hint = "Search"
	}
	s := fieldBox(th)
	s.Radius, s.BorderWidth = 24, 0
	input := TextInput(b.Value, b.OnChange, Style{Grow: 1, FontSize: th.Body, Color: th.Text}).Placeholder(hint)
	if b.OnSubmit != nil {
		input = input.OnSubmit(b.OnSubmit)
	}
	kids := []Component{Icon("search", 20, th.TextMuted), input}
	if b.Value != "" {
		kids = append(kids, Pressable(func() { b.OnChange("") }, Style{}, Icon("close", 18, th.TextMuted)).Label("Clear"))
	}
	return View(s, kids...)
}

// OTPInput takes a short code, a box for each digit:
//
//	ui.C(ui.OTPInput{Length: 6, Value: code, OnChange: setCode})
type OTPInput struct {
	Length   int // 0 means 6
	Value    string
	OnChange func(string)
}

func (o OTPInput) Render(c *Ctx) Component {
	th := UseTheme(c)
	n := o.Length
	if n == 0 {
		n = 6
	}
	cells := make([]Component, n)
	for i := range cells {
		ch, border := "", th.Border
		if i < len(o.Value) {
			ch = o.Value[i : i+1]
		}
		if i == len(o.Value) {
			border = th.Primary
		}
		cells[i] = View(Style{Grow: 1, Basis: Pt(0), Height: Pt(54), Radius: th.Radius, Background: th.Surface, BorderWidth: 1.5, BorderColor: border,
			Justify: JustifyCenter, AlignItems: AlignCenter, Transition: 120},
			Text(ch, Style{FontSize: 22, FontWeight: 600, Color: th.Text}))
	}
	cells = append(cells, TextInput(o.Value, func(v string) {
		var d strings.Builder
		for _, r := range v {
			if r >= '0' && r <= '9' && d.Len() < n {
				d.WriteRune(r)
			}
		}
		o.OnChange(d.String())
	}, Style{Absolute: true, Left: Pt(0), Right: Pt(0), Top: Pt(0), Bottom: Pt(0), Opacity: 0.01}).Keyboard(render.KeyboardNumber))
	return View(Style{Direction: Row, Gap: 8}, cells...)
}

// Select is a field that picks one of its options from a list the platform
// shows:
//
//	ui.C(ui.Select{Label: "City", Options: cities, Selected: city, OnChange: setCity})
type Select struct {
	Label       string
	Placeholder string
	Options     []string
	Selected    int // -1 for none
	OnChange    func(int)
}

func (p Select) Render(c *Ctx) Component {
	th := UseTheme(c)
	shown, color := p.Placeholder, th.TextMuted
	if p.Selected >= 0 && p.Selected < len(p.Options) {
		shown, color = p.Options[p.Selected], th.Text
	}
	field := Pressable(func() {
		go func() {
			if i, err := dialog.Choose(context.Background(), p.Label, p.Options...); err == nil && i >= 0 {
				c.rt.do(func() { p.OnChange(i) })
			}
		}()
	}, fieldBox(th), Text(shown, Style{Grow: 1, Shrink: 1, FontSize: th.Body, Color: color, Lines: 1}), Icon("chevron-down", 18, th.TextMuted)).Label(p.Label)
	return labelled(th, p.Label, field)
}

// DateField is a field that picks a date or time with the platform's picker:
//
//	ui.C(ui.DateField{Label: "Delivery", Value: at, OnChange: setAt})
type DateField struct {
	Label       string
	Placeholder string
	Value       time.Time // zero for none
	Mode        dialog.Mode
	Layout      string // how the value shows; "2 Jan 2006" by default
	OnChange    func(time.Time)
}

func (d DateField) Render(c *Ctx) Component {
	th := UseTheme(c)
	layout := d.Layout
	if layout == "" {
		layout = map[dialog.Mode]string{dialog.Time: "3:04 pm", dialog.DateTime: "2 Jan 2006, 3:04 pm"}[d.Mode]
	}
	if layout == "" {
		layout = "2 Jan 2006"
	}
	shown, color := d.Placeholder, th.TextMuted
	if !d.Value.IsZero() {
		shown, color = d.Value.Format(layout), th.Text
	}
	icon := "calendar"
	if d.Mode == dialog.Time {
		icon = "clock"
	}
	field := Pressable(func() {
		start := d.Value
		if start.IsZero() {
			start = time.Now()
		}
		go func() {
			if t, ok, err := dialog.PickDate(context.Background(), d.Mode, start); err == nil && ok {
				c.rt.do(func() { d.OnChange(t) })
			}
		}()
	}, fieldBox(th), Text(shown, Style{Grow: 1, Shrink: 1, FontSize: th.Body, Color: color, Lines: 1}), Icon(icon, 19, th.TextMuted)).Label(d.Label)
	return labelled(th, d.Label, field)
}

// labelled puts a field under its label.
func labelled(th Theme, label string, field *Element) *Element {
	if label == "" {
		return field
	}
	return View(Style{Gap: 6}, Text(label, Style{FontSize: th.Caption, FontWeight: 500, Color: th.TextMuted}), field)
}
