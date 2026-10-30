package ui

import (
	"fmt"
	"math"
)

// AppBar is a screen's top bar when it has no navigation Stack of its own:
//
//	ui.C(ui.AppBar{Title: "Orders", Actions: []*ui.Element{ui.C(ui.IconButton{Icon: "search", Label: "Search"})}})
type AppBar struct {
	Title    string
	Subtitle string
	OnBack   func()     // adds a back arrow
	Leading  *Element   // or anything else at the start
	Actions  []*Element // at the end
	Large    bool       // the title on its own line, larger
}

func (b AppBar) Render(c *Ctx) *Element {
	th, vp := UseTheme(c), Viewport(c)
	lead := b.Leading
	if b.OnBack != nil {
		lead = C(IconButton{Icon: "arrow-left", Label: "Back", OnPress: b.OnBack})
	}
	words := []*Element{}
	if !b.Large {
		words = append(words, Text(b.Title, Style{FontSize: th.Heading + 1, FontWeight: 600, Color: th.Text, Lines: 1}))
		if b.Subtitle != "" {
			words = append(words, Text(b.Subtitle, Style{FontSize: th.Caption, Color: th.TextMuted, Lines: 1}))
		}
	}
	row := []*Element{}
	if lead != nil {
		row = append(row, lead)
	}
	row = append(row, View(Style{Grow: 1, Shrink: 1}, words...))
	row = append(row, b.Actions...)
	kids := []*Element{View(Style{Direction: Row, AlignItems: AlignCenter, Gap: 8, MinHeight: Pt(52), Padding: XY(8, 0)}, row...)}
	if b.Large {
		kids = append(kids, View(Style{Padding: Edges{Left: 16, Right: 16, Bottom: 10}, Gap: 2},
			Text(b.Title, Style{FontSize: th.Title + 6, FontWeight: 700, LetterSpacing: -0.5, Color: th.Text}),
			Text(b.Subtitle, Style{FontSize: th.Body, Color: th.TextMuted, Hidden: b.Subtitle == ""})))
	}
	return View(Style{Padding: Edges{Top: vp.SafeTop}, Background: th.Bar, BorderColor: th.Border}, kids...)
}

// Grid lays its items out in equal columns:
//
//	ui.C(ui.Grid{Columns: 2, Gap: 12, Items: tiles})
type Grid struct {
	Columns int // 0 means 2
	Gap     float32
	Items   []*Element
}

func (g Grid) Render(c *Ctx) *Element {
	n := g.Columns
	if n == 0 {
		n = 2
	}
	var rows []*Element
	for i := 0; i < len(g.Items); i += n {
		cells := make([]*Element, n)
		for k := range cells {
			var item *Element
			if i+k < len(g.Items) {
				item = g.Items[i+k]
			}
			cells[k] = View(Style{Grow: 1, Shrink: 1, Basis: Pt(0), MinWidth: Pt(0)}, item)
		}
		rows = append(rows, View(Style{Direction: Row, Gap: g.Gap}, cells...))
	}
	return View(Style{Gap: g.Gap}, rows...)
}

// AccordionItem is a heading and what it opens.
type AccordionItem struct {
	Title   string
	Content *Element
}

// Accordion is a list of headings that open in place. With Single, opening
// one closes the rest.
type Accordion struct {
	Items  []AccordionItem
	Single bool
	Open   int // the one open at the start; 0 for the first, -1 for none
}

func (a Accordion) Render(c *Ctx) *Element {
	th := UseTheme(c)
	open, setOpen := State(c, map[int]bool{a.Open: a.Open >= 0})
	kids := make([]*Element, 0, len(a.Items)*2)
	for i, it := range a.Items {
		turn := float32(0)
		if open[i] {
			turn = 180
		}
		head := Pressable(func() {
			next := map[int]bool{i: !open[i]}
			if !a.Single {
				for k, v := range open {
					if k != i {
						next[k] = v
					}
				}
			}
			setOpen(next)
		}, Style{Direction: Row, AlignItems: AlignCenter, Gap: 12, Padding: XY(0, 14)},
			View(Style{Grow: 1, Shrink: 1}, Text(it.Title, Style{FontSize: th.Body + 1, FontWeight: 500, Color: th.Text})),
			View(Style{Rotate: turn, Transition: 200}, Icon("chevron-down", 19, th.TextMuted))).Label(it.Title)
		var body *Element
		if open[i] {
			body = View(Style{Padding: Edges{Bottom: 14}}, it.Content)
		}
		kids = append(kids, View(Style{}, head, body, C(Divider{})).Key(fmt.Sprint(i)))
	}
	return View(Style{}, kids...)
}

// Carousel shows one page at a time; swipe sideways for the next, with dots
// to show where you are.
type Carousel struct {
	Pages    []*Element
	Height   float32
	Width    float32 // 0 means the screen's width
	OnChange func(int)
}

func (r Carousel) Render(c *Ctx) *Element {
	th := UseTheme(c)
	w := r.Width
	if w == 0 {
		w = Viewport(c).W
	}
	at, setAt := State(c, 0)
	drag, setDrag := State(c, float32(0))
	move := 300
	if drag != 0 {
		move = 0
	}
	pages := make([]*Element, len(r.Pages))
	for i, p := range r.Pages {
		pages[i] = View(Style{Width: Pt(w), Height: Pt(r.Height)}, p).Key(fmt.Sprint(i))
	}
	strip := View(Style{Absolute: true, Top: Pt(0), Left: Pt(-float32(at)*w + drag), Direction: Row, Transition: move}, pages...)
	frame := View(Style{Width: Pt(w), Height: Pt(r.Height), Clip: true}, strip).OnPanX(func(p Pan) {
		switch p.Phase {
		case Changed:
			setDrag(p.X)
		case Ended, Cancelled:
			setDrag(0)
			step := 0
			if math.Abs(float64(p.X)) > float64(w)/4 || math.Abs(float64(p.VX)) > 500 {
				step = 1
				if p.X > 0 {
					step = -1
				}
			}
			if to := min(max(at+step, 0), len(r.Pages)-1); to != at {
				setAt(to)
				if r.OnChange != nil {
					r.OnChange(to)
				}
			}
		}
	})
	dots := make([]*Element, len(r.Pages))
	for i := range dots {
		s := Style{Width: Pt(7), Height: Pt(7), Radius: 3.5, Background: th.Border, Transition: 200}
		if i == at {
			s.Background, s.Width = th.Primary, Pt(20)
		}
		dots[i] = View(s)
	}
	return View(Style{Gap: 12}, frame, View(Style{Direction: Row, Gap: 6, Justify: JustifyCenter}, dots...))
}

// SwipeAction is a button a SwipeRow reveals.
type SwipeAction struct {
	Label   string
	Icon    string
	Tone    Tone // Bad for a delete
	OnPress func()
}

// SwipeRow reveals actions when its content is swiped to the left. It works
// inside a list: an up or down drag still scrolls.
type SwipeRow struct {
	Content *Element
	Actions []SwipeAction
}

func (r SwipeRow) Render(c *Ctx) *Element {
	th := UseTheme(c)
	const each = 82
	full := float32(len(r.Actions) * each)
	opened, setOpened := State(c, false)
	drag, setDrag := State(c, float32(0))
	dragging, setDragging := State(c, false)
	left := drag
	if opened {
		left = -full + drag
	}
	left = min(max(left, -full-24), 0)
	move := 240
	if dragging {
		move = 0
	}
	buttons := make([]*Element, len(r.Actions))
	for i, a := range r.Actions {
		color := a.Tone.color(th)
		if a.Tone == Neutral {
			color = th.Primary
		}
		buttons[i] = Pressable(func() {
			setOpened(false)
			if a.OnPress != nil {
				a.OnPress()
			}
		}, Style{Width: Pt(each), Background: color, Justify: JustifyCenter, AlignItems: AlignCenter, Gap: 3},
			Icon(a.Icon, 21, th.OnPrimary), Text(a.Label, Style{FontSize: 12, FontWeight: 600, Color: th.OnPrimary})).Label(a.Label)
	}
	return View(Style{Clip: true},
		View(Style{Absolute: true, Right: Pt(0), Top: Pt(0), Bottom: Pt(0), Direction: Row}, buttons...),
		View(Style{Left: Pt(left), Background: th.Background, Transition: move}, r.Content).OnPanX(func(p Pan) {
			switch p.Phase {
			case Began:
				setDragging(true)
			case Changed:
				setDrag(p.X)
			case Ended, Cancelled:
				setDragging(false)
				setDrag(0)
				switch {
				case !opened && (p.X < -full/2 || p.VX < -500):
					setOpened(true)
				case opened && (p.X > full/3 || p.VX > 500):
					setOpened(false)
				}
			}
		}))
}
