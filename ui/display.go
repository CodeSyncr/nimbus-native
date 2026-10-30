package ui

import (
	"context"
	"hash/fnv"
	"strings"
	"time"
)

// Avatar is a person: their picture, or their initials on a color of their
// own:
//
//	ui.C(ui.Avatar{Name: "Asha Rao", Size: 44})
type Avatar struct {
	Name  string
	Image string  // a picture's URL
	Size  float32 // 0 means 40
}

// avatarColors are the backgrounds initials sit on.
var avatarColors = []Color{Hex("#0d7377"), Hex("#2563eb"), Hex("#7c3aed"), Hex("#c2410c"), Hex("#be123c"), Hex("#15803d")}

func (a Avatar) Render(c *Ctx) *Element {
	size := a.Size
	if size == 0 {
		size = 40
	}
	s := Style{Width: Pt(size), Height: Pt(size), Radius: size / 2, Clip: true, Justify: JustifyCenter, AlignItems: AlignCenter}
	if a.Image != "" {
		return Image(a.Image, s).Label(a.Name)
	}
	h := fnv.New32a()
	h.Write([]byte(a.Name))
	s.Background = avatarColors[h.Sum32()%uint32(len(avatarColors))]
	initials := ""
	for _, w := range strings.Fields(a.Name) {
		if len(initials) < 2 {
			initials += strings.ToUpper(string([]rune(w)[:1]))
		}
	}
	return View(s, Text(initials, Style{Color: Hex("#ffffff"), FontSize: size * 0.38, FontWeight: 600})).Label(a.Name)
}

// Tone is what a Badge or Banner means.
type Tone uint8

const (
	Neutral Tone = iota
	Accent
	Good
	Caution
	Bad
)

func (t Tone) color(th Theme) Color {
	switch t {
	case Accent:
		return th.Primary
	case Good:
		return th.Success
	case Caution:
		return th.Warning
	case Bad:
		return th.Danger
	}
	return th.TextMuted
}

// Badge is a small status label:
//
//	ui.C(ui.Badge{Label: "Paid", Tone: ui.Good})
type Badge struct {
	Label string
	Tone  Tone
	Solid bool // white on the tone's color, for a count on an icon
}

func (b Badge) Render(c *Ctx) *Element {
	th := UseTheme(c)
	color := b.Tone.color(th)
	bg, fg := fade(color, 0.14), color
	if b.Solid {
		bg, fg = color, th.OnPrimary
	}
	// In a row around it, so a list row can centre it and a column won't stretch it.
	return View(Style{Direction: Row},
		View(Style{Padding: XY(9, 3), Radius: 999, Background: bg},
			Text(b.Label, Style{FontSize: 12, FontWeight: 600, Color: fg, Lines: 1})))
}

// Chip is a compact choice, filter or tag:
//
//	ui.C(ui.Chip{Label: "Vegetarian", Selected: veg, OnPress: toggle})
//
// OnRemove adds a close button.
type Chip struct {
	Label    string
	Icon     string
	Selected bool
	OnPress  func()
	OnRemove func()
}

func (p Chip) Render(c *Ctx) *Element {
	th := UseTheme(c)
	bg, fg, border := Color(0), th.Text, th.Border
	if p.Selected {
		bg, fg, border = fade(th.Primary, 0.12), th.Primary, th.Primary
	}
	var kids []Component
	if p.Icon != "" {
		kids = append(kids, Icon(p.Icon, 16, fg))
	}
	kids = append(kids, Text(p.Label, Style{FontSize: 13.5, FontWeight: 500, Color: fg, Lines: 1}))
	if p.OnRemove != nil {
		kids = append(kids, Pressable(p.OnRemove, Style{}, Icon("close", 15, fg)).Label("Remove "+p.Label))
	}
	return Pressable(p.OnPress, Style{AlignSelf: AlignStart, Height: Pt(34), Padding: XY(13, 0), Radius: 17, Background: bg, BorderWidth: 1, BorderColor: border,
		Direction: Row, AlignItems: AlignCenter, Gap: 6, Transition: 140}, kids...).Label(p.Label)
}

// Divider is a thin rule between things.
type Divider struct {
	Inset float32 // space before it, to line up with text in a row
}

func (d Divider) Render(c *Ctx) *Element {
	return View(Style{Height: Pt(1), Margin: Edges{Left: d.Inset}, Background: UseTheme(c).Border})
}

// ListRow is one row of a list or settings screen:
//
//	ui.C(ui.ListRow{Title: "Notifications", Subtitle: "On", Icon: "bell", OnPress: open})
type ListRow struct {
	Title    string
	Subtitle string
	Icon     string    // an Icon name, in a tinted square
	Leading  Component // anything else at the start: an Avatar, say
	Trailing Component // at the end: a Switch, a Badge, a price
	OnPress  func()    // adds a chevron unless there is a Trailing
}

func (r ListRow) Render(c *Ctx) *Element {
	th := UseTheme(c)
	var kids []Component
	switch {
	case element(r.Leading) != nil:
		kids = append(kids, r.Leading)
	case r.Icon != "":
		kids = append(kids, View(Style{Width: Pt(36), Height: Pt(36), Radius: 10, Background: fade(th.Primary, 0.12), Justify: JustifyCenter, AlignItems: AlignCenter},
			Icon(r.Icon, 20, th.Primary)))
	}
	words := []Component{Text(r.Title, Style{FontSize: th.Body + 1, FontWeight: 500, Color: th.Text, Lines: 1})}
	if r.Subtitle != "" {
		words = append(words, Text(r.Subtitle, Style{FontSize: th.Caption, Color: th.TextMuted, Lines: 2}))
	}
	kids = append(kids, View(Style{Grow: 1, Shrink: 1, Gap: 2}, words...))
	switch {
	case element(r.Trailing) != nil:
		kids = append(kids, r.Trailing)
	case r.OnPress != nil:
		kids = append(kids, Icon("chevron-right", 18, th.TextMuted))
	}
	s := Style{Direction: Row, AlignItems: AlignCenter, Gap: 12, Padding: XY(0, 10), MinHeight: Pt(56)}
	if r.OnPress != nil {
		return Pressable(r.OnPress, s, kids...).Label(r.Title)
	}
	return View(s, kids...)
}

// ProgressBar shows how far something has got. Value is 0..1, and a bar
// with Busy runs back and forth when you can't tell.
type ProgressBar struct {
	Value  float64
	Busy   bool
	Height float32 // 0 means 6
	Tone   Tone    // Accent when Neutral
}

func (b ProgressBar) Render(c *Ctx) *Element {
	th := UseTheme(c)
	h := b.Height
	if h == 0 {
		h = 6
	}
	color := th.Primary
	if b.Tone != Neutral {
		color = b.Tone.color(th)
	}
	left, width := float32(0), float32(min(max(b.Value, 0), 1))*100
	if b.Busy {
		on, setOn := State(c, false)
		tick(c, 900*time.Millisecond, func() { setOn(!on) }, on)
		width = 35
		if on {
			left = 65
		}
	}
	return View(Style{Height: Pt(h), Radius: h / 2, Background: th.Border, Clip: true},
		View(Style{Absolute: true, Top: Pt(0), Bottom: Pt(0), Left: Pct(left), Width: Pct(width), Radius: h / 2, Background: color, Transition: 800}))
}

// tick calls f every so often while the component is on screen.
func tick(c *Ctx, every time.Duration, f func(), state any) {
	Effect(c, func(ctx context.Context) func() {
		go func() {
			select {
			case <-ctx.Done():
			case <-time.After(every):
				f()
			}
		}()
		return nil
	}, state)
}

// Skeleton is a placeholder that pulses where content will load.
type Skeleton struct {
	Width  Value
	Height float32 // 0 means 16
	Radius float32 // 0 means 6
	Circle bool
}

func (k Skeleton) Render(c *Ctx) *Element {
	th := UseTheme(c)
	dim, setDim := State(c, false)
	tick(c, 800*time.Millisecond, func() { setDim(!dim) }, dim)
	h, r := k.Height, k.Radius
	if h == 0 {
		h = 16
	}
	if r == 0 {
		r = 6
	}
	s := Style{Width: k.Width, Height: Pt(h), Radius: r, Background: th.Border, Opacity: 1, Transition: 800}
	if k.Circle {
		s.Width, s.Radius = Pt(h), h/2
	}
	if dim {
		s.Opacity = 0.45
	}
	return View(s)
}

// EmptyState is what a screen shows when it has nothing yet:
//
//	ui.C(ui.EmptyState{Icon: "cart", Title: "No orders yet", Message: "New orders show up here.",
//		Action: "Open POS", OnAction: open})
type EmptyState struct {
	Icon, Title, Message string
	Action               string
	OnAction             func()
}

func (e EmptyState) Render(c *Ctx) *Element {
	th := UseTheme(c)
	kids := []Component{}
	if e.Icon != "" {
		kids = append(kids, View(Style{Width: Pt(72), Height: Pt(72), Radius: 36, Background: fade(th.Primary, 0.1), Justify: JustifyCenter, AlignItems: AlignCenter, Margin: Edges{Bottom: 8}},
			Icon(e.Icon, 34, th.Primary)))
	}
	kids = append(kids, Text(e.Title, Style{FontSize: th.Heading + 2, FontWeight: 600, Color: th.Text, TextAlign: AlignCenter}))
	if e.Message != "" {
		kids = append(kids, Text(e.Message, Style{FontSize: th.Body, Color: th.TextMuted, TextAlign: AlignCenter}))
	}
	if e.Action != "" {
		kids = append(kids, View(Style{Margin: Edges{Top: 10}}, Button{Label: e.Action, OnPress: e.OnAction}))
	}
	return View(Style{Grow: 1, Justify: JustifyCenter, AlignItems: AlignCenter, Gap: 6, Padding: XY(32, 24)}, kids...)
}
