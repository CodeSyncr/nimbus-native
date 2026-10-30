// showcase is every component on a screen of its own, the source of the
// guide's pictures: the app of a small cafe. SHOWCASE=<name> opens one
// directly.
package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugins/dialog"
	"github.com/CodeSyncr/nimbus-native/ui"
)

var (
	teal  = ui.Hex("#0d7377")
	paper = ui.Hex("#faf6f0")
	ink   = ui.Hex("#211d1a")
)

func init() {
	light, dark := ui.LightTheme, ui.DarkTheme
	light.Background, light.Surface, light.Text, light.TextMuted, light.Border = paper, ui.Hex("#ffffff"), ink, ui.Hex("#6e665f"), ui.Hex("#e7e0d6")
	light.Primary, light.Bar, light.BarText, light.Radius = teal, paper, ink, 14
	dark.Background, dark.Surface, dark.Text, dark.TextMuted, dark.Border = ui.Hex("#101816"), ui.Hex("#18211f"), ui.Hex("#f2eee7"), ui.Hex("#a3a097"), ui.Hex("#2a3532")
	dark.Primary, dark.Bar, dark.BarText, dark.Radius = ui.Hex("#3fb6b0"), ui.Hex("#101816"), ui.Hex("#f2eee7"), 14
	ui.SetTheme(light, dark)
	ui.Mount(App{})
}

// main runs only under `nimbus native dev`; on a device the host owns the process.
func main() { ui.Main() }

// screens are the components, by the name their guide page has.
var screens = map[string]func() ui.Component{
	"button":       func() ui.Component { return buttons{} },
	"icon-button":  func() ui.Component { return iconButtons{} },
	"floating":     func() ui.Component { return floating{} },
	"icon":         func() ui.Component { return icons{} },
	"checkbox":     func() ui.Component { return checkboxes{} },
	"radio":        func() ui.Component { return radios{} },
	"segmented":    func() ui.Component { return segmented{} },
	"stepper":      func() ui.Component { return steppers{} },
	"select":       func() ui.Component { return selects{} },
	"date-field":   func() ui.Component { return dates{} },
	"search-bar":   func() ui.Component { return searchBars{} },
	"otp":          func() ui.Component { return otps{} },
	"avatar":       func() ui.Component { return avatars{} },
	"badge":        func() ui.Component { return badges{} },
	"chip":         func() ui.Component { return chips{} },
	"divider":      func() ui.Component { return dividers{} },
	"list-row":     func() ui.Component { return listRows{} },
	"progress":     func() ui.Component { return progress{} },
	"skeleton":     func() ui.Component { return skeletons{} },
	"empty":        func() ui.Component { return empties{} },
	"sheet":        func() ui.Component { return sheets{} },
	"action-sheet": func() ui.Component { return actionSheets{} },
	"toast":        func() ui.Component { return toasts{} },
	"banner":       func() ui.Component { return banners{} },
	"menu":         func() ui.Component { return menus{} },
	"tooltip":      func() ui.Component { return tooltips{} },
	"app-bar":      func() ui.Component { return appBars{} },
	"grid":         func() ui.Component { return grids{} },
	"accordion":    func() ui.Component { return accordions{} },
	"carousel":     func() ui.Component { return carousels{} },
	"swipe-row":    func() ui.Component { return swipeRows{} },
	"gestures":     func() ui.Component { return gestures{} },
}

// App opens the screen SHOWCASE names, or the list of them.
type App struct{}

func (App) Render(c *ui.Ctx) *ui.Element {
	if f := screens[os.Getenv("SHOWCASE")]; f != nil {
		return ui.C(f())
	}
	open, setOpen := ui.State(c, "")
	shown := ui.Ref(c, "")
	// SHOWCASE_FILE names the screen in a file: the picture script moves
	// through them without a rebuild.
	ui.Effect(c, func(ctx context.Context) func() {
		path := os.Getenv("SHOWCASE_FILE")
		if path == "" {
			return nil
		}
		go func() {
			for ctx.Err() == nil {
				if b, err := os.ReadFile(path); err == nil {
					if name := strings.TrimSpace(string(b)); name != *shown {
						*shown = name
						ui.Overlays(c).Dismiss()
						setOpen(name)
					}
				}
				time.Sleep(150 * time.Millisecond)
			}
		}()
		return nil
	}, ui.Once)
	if f := screens[open]; f != nil {
		if os.Getenv("SHOWCASE_FILE") != "" {
			return ui.View(ui.Style{Grow: 1}, ui.Key(open, f()))
		}
		return ui.View(ui.Style{Grow: 1}, ui.C(f()), ui.View(ui.Style{Absolute: true, Top: ui.Pt(ui.Viewport(c).SafeTop), Right: ui.Pt(8)},
			ui.IconButton{Icon: "close", Label: "Back", OnPress: func() { setOpen("") }}))
	}
	names := make([]string, 0, len(screens))
	for n := range screens {
		names = append(names, n)
	}
	sort.Strings(names)
	rows := make([]ui.Component, len(names))
	for i, n := range names {
		rows[i] = ui.Key(n, ui.ListRow{Title: n, OnPress: func() { setOpen(n) }})
	}
	return ui.View(ui.Style{Grow: 1, Background: ui.UseTheme(c).Background},
		ui.AppBar{Title: "Components", Large: true},
		ui.Scroll(ui.Style{Grow: 1, Padding: ui.XY(16, 0)}, rows...))
}

// page is a showcase screen: a title, a line, and what it shows.
func page(c *ui.Ctx, title, line string, kids ...ui.Component) *ui.Element {
	th := ui.UseTheme(c)
	return ui.SafeArea(ui.Style{Grow: 1, Background: th.Background, Padding: ui.Edges{Left: 20, Right: 20, Top: 18}},
		ui.Text("Monsoon Coffee", ui.Style{FontSize: 13, FontWeight: 600, Color: th.Primary}),
		ui.Text(title, ui.Style{FontSize: 31, FontWeight: 700, LetterSpacing: -0.6, Color: th.Text, Margin: ui.Edges{Top: 4}}),
		ui.Text(line, ui.Style{FontSize: 15, Color: th.TextMuted, Margin: ui.Edges{Top: 4, Bottom: 22}}),
		ui.View(ui.Style{Grow: 1, Gap: 14}, kids...))
}

func heading(th ui.Theme, s string) *ui.Element {
	return ui.Text(s, ui.Style{FontSize: 13, FontWeight: 600, Color: th.TextMuted, Margin: ui.Edges{Top: 6}})
}

func row(gap float32, kids ...ui.Component) *ui.Element {
	return ui.View(ui.Style{Direction: ui.Row, AlignItems: ui.AlignCenter, Gap: gap, Wrap: true, LineGap: gap}, kids...)
}

// wall is a tiled colour standing in for a photo.
func wall(c1, c2 string, label string) *ui.Element {
	return ui.View(ui.Style{Grow: 1, Radius: 18, Justify: ui.JustifyEnd, Padding: ui.All(16),
		Gradient: ui.LinearGradient(ui.Point{X: 0, Y: 0}, ui.Point{X: 1, Y: 1}, ui.Hex(c1), ui.Hex(c2))},
		ui.Text(label, ui.Style{FontSize: 19, FontWeight: 700, Color: ui.Hex("#ffffff")}))
}

type buttons struct{}

func (buttons) Render(c *ui.Ctx) *ui.Element {
	nop := func() {}
	return page(c, "Your order", "Two flat whites and a cardamom bun.",
		ui.Button{Label: "Place order", Icon: "check", Full: true, Size: ui.SizeLarge, OnPress: nop},
		ui.Button{Label: "Add a note", Variant: ui.Tonal, Icon: "edit", Full: true, OnPress: nop},
		ui.Button{Label: "Save for later", Variant: ui.Outlined, Full: true, OnPress: nop},
		ui.Button{Label: "Sending…", Loading: true, Full: true, OnPress: nop},
		ui.Button{Label: "Sold out", Disabled: true, Full: true, OnPress: nop},
		row(10, ui.Button{Label: "Cancel", Variant: ui.Destructive, Size: ui.SizeSmall, OnPress: nop},
			ui.Button{Label: "Keep it", Variant: ui.Plain, Size: ui.SizeSmall, OnPress: nop}))
}

type iconButtons struct{}

func (iconButtons) Render(c *ui.Ctx) *ui.Element {
	th, nop := ui.UseTheme(c), func() {}
	return page(c, "Flat white", "Double shot, oat milk.",
		ui.View(ui.Style{Height: ui.Pt(200)}, wall("#c98a5a", "#5a3a2a", "Flat white")),
		row(12,
			ui.IconButton{Icon: "heart", Label: "Save", Variant: ui.Tonal, OnPress: nop},
			ui.IconButton{Icon: "share", Label: "Share", Variant: ui.Outlined, OnPress: nop},
			ui.IconButton{Icon: "bell", Label: "Remind me", OnPress: nop},
			ui.IconButton{Icon: "plus", Label: "Add", Filled: true, OnPress: nop},
			ui.IconButton{Icon: "cart", Label: "Basket", Filled: true, Size: ui.SizeLarge, OnPress: nop}),
		heading(th, "Three sizes"),
		row(12, ui.IconButton{Icon: "star", Label: "Rate", Variant: ui.Tonal, Size: ui.SizeSmall, OnPress: nop},
			ui.IconButton{Icon: "star", Label: "Rate", Variant: ui.Tonal, OnPress: nop},
			ui.IconButton{Icon: "star", Label: "Rate", Variant: ui.Tonal, Size: ui.SizeLarge, OnPress: nop}))
}

type floating struct{}

func (floating) Render(c *ui.Ctx) *ui.Element {
	return ui.View(ui.Style{Grow: 1},
		page(c, "Today", "Eleven orders so far.",
			ui.ListRow{Title: "Asha Rao", Subtitle: "2 flat whites", Leading: ui.Avatar{Name: "Asha Rao"}},
			ui.ListRow{Title: "Vikram Shah", Subtitle: "Filter coffee, bun", Leading: ui.Avatar{Name: "Vikram Shah"}},
			ui.ListRow{Title: "Meera Iyer", Subtitle: "Masala chai", Leading: ui.Avatar{Name: "Meera Iyer"}}),
		ui.FloatingButton{Icon: "plus", Label: "New order", Extended: true, OnPress: func() {}})
}

type icons struct{}

func (icons) Render(c *ui.Ctx) *ui.Element {
	th := ui.UseTheme(c)
	names := ui.IconNames()
	sort.Strings(names)
	cells := make([]ui.Component, len(names))
	for i, n := range names {
		cells[i] = ui.View(ui.Style{Height: ui.Pt(52), Radius: 12, Background: th.Surface, Justify: ui.JustifyCenter, AlignItems: ui.AlignCenter},
			ui.Icon(n, 24, th.Primary))
	}
	return page(c, "Icons", fmt.Sprintf("%d of our own, the same everywhere.", len(names)),
		ui.Scroll(ui.Style{Grow: 1}, ui.Grid{Columns: 6, Gap: 8, Items: cells}))
}

type checkboxes struct{}

func (checkboxes) Render(c *ui.Ctx) *ui.Element {
	a, setA := ui.State(c, true)
	b, setB := ui.State(c, false)
	d, setD := ui.State(c, true)
	return page(c, "Extras", "Make it yours.",
		ui.Checkbox{Label: "Extra shot", Checked: a, OnChange: setA},
		ui.Checkbox{Label: "Oat milk", Checked: d, OnChange: setD},
		ui.Checkbox{Label: "Cinnamon on top", Checked: b, OnChange: setB},
		ui.Checkbox{Label: "Decaf (sold out)", Disabled: true})
}

type radios struct{}

func (radios) Render(c *ui.Ctx) *ui.Element {
	s, setS := ui.State(c, 1)
	return page(c, "Pay with", "Choose one.",
		ui.RadioGroup{Options: []string{"Card", "UPI", "Cash at the counter"}, Selected: s, OnChange: setS})
}

type segmented struct{}

func (segmented) Render(c *ui.Ctx) *ui.Element {
	th := ui.UseTheme(c)
	s, setS := ui.State(c, 1)
	return page(c, "Sales", "How the week is going.",
		ui.SegmentedControl{Options: []string{"Day", "Week", "Month"}, Selected: s, OnChange: setS},
		ui.View(ui.Style{Height: ui.Pt(14)}),
		ui.Text([]string{"₹18,240", "₹1,24,600", "₹5,31,900"}[s], ui.Style{FontSize: 42, FontWeight: 700, LetterSpacing: -1, Color: th.Text, TabularDigits: true}),
		ui.Badge{Label: "12% up", Tone: ui.Good})
}

type steppers struct{}

func (steppers) Render(c *ui.Ctx) *ui.Element {
	th := ui.UseTheme(c)
	q, setQ := ui.State(c, 2)
	r, setR := ui.State(c, 1)
	item := func(name, price string, v int, f func(int)) *ui.Element {
		return ui.View(ui.Style{Direction: ui.Row, AlignItems: ui.AlignCenter, Gap: 12},
			ui.View(ui.Style{Grow: 1, Gap: 2}, ui.Text(name, ui.Style{FontSize: 16, FontWeight: 500, Color: th.Text}), ui.Text(price, ui.Style{FontSize: 13, Color: th.TextMuted})),
			ui.Stepper{Value: v, Min: 0, Max: 9, OnChange: f})
	}
	return page(c, "Basket", "Change anything before you pay.",
		item("Flat white", "₹220", q, setQ), ui.Divider{}, item("Cardamom bun", "₹140", r, setR))
}

type selects struct{}

func (selects) Render(c *ui.Ctx) *ui.Element {
	s, setS := ui.State(c, 0)
	t, setT := ui.State(c, -1)
	return page(c, "Pickup", "Where shall we have it ready?",
		ui.Select{Label: "Branch", Options: []string{"Koregaon Park", "Baner", "Aundh"}, Selected: s, OnChange: setS},
		ui.Select{Label: "Table", Placeholder: "Choose a table", Options: []string{"Window", "Garden", "Counter"}, Selected: t, OnChange: setT})
}

type dates struct{}

func (dates) Render(c *ui.Ctx) *ui.Element {
	d, setD := ui.State(c, time.Date(2026, 11, 14, 9, 30, 0, 0, time.Local))
	e, setE := ui.State(c, time.Time{})
	return page(c, "Book a table", "We'll hold it for fifteen minutes.",
		ui.DateField{Label: "Date", Value: d, OnChange: setD},
		ui.DateField{Label: "Time", Mode: dialog.Time, Placeholder: "Choose a time", Value: e, OnChange: setE})
}

type searchBars struct{}

func (searchBars) Render(c *ui.Ctx) *ui.Element {
	q, setQ := ui.State(c, "chai")
	menu := []string{"Masala chai", "Ginger chai", "Iced chai latte"}
	rows := []ui.Component{ui.SearchBar{Value: q, OnChange: setQ, Placeholder: "Search the menu"}}
	for _, m := range menu {
		if strings.Contains(strings.ToLower(m), strings.ToLower(q)) {
			rows = append(rows, ui.Key(m, ui.ListRow{Title: m, Subtitle: "₹180", Icon: "tag"}))
		}
	}
	return page(c, "Menu", "Everything we're serving today.", rows...)
}

type otps struct{}

func (otps) Render(c *ui.Ctx) *ui.Element {
	v, setV := ui.State(c, "4821")
	return page(c, "Verify", "We sent a code to +91 98••• ••210.",
		ui.OTPInput{Length: 6, Value: v, OnChange: setV},
		ui.View(ui.Style{Height: ui.Pt(8)}),
		ui.Button{Label: "Confirm", Full: true, Disabled: len(v) < 6, OnPress: func() {}})
}

type avatars struct{}

func (avatars) Render(c *ui.Ctx) *ui.Element {
	th := ui.UseTheme(c)
	return page(c, "The team", "Everyone on shift today.",
		row(12, ui.Avatar{Name: "Asha Rao", Size: 64}, ui.Avatar{Name: "Vikram Shah", Size: 64}, ui.Avatar{Name: "Meera Iyer", Size: 64}, ui.Avatar{Name: "Rohan Das", Size: 64}),
		heading(th, "Any size"),
		row(12, ui.Avatar{Name: "Priya Nair", Size: 28}, ui.Avatar{Name: "Priya Nair", Size: 40}, ui.Avatar{Name: "Priya Nair", Size: 56}, ui.Avatar{Name: "Priya Nair", Size: 80}))
}

type badges struct{}

func (badges) Render(c *ui.Ctx) *ui.Element {
	row2 := func(name string, b ui.Badge) *ui.Element {
		return ui.C(ui.ListRow{Title: name, Subtitle: "Order #" + fmt.Sprint(1000+len(name)), Trailing: ui.C(b)})
	}
	return page(c, "Orders", "What's happening now.",
		row2("Asha Rao", ui.Badge{Label: "Paid", Tone: ui.Good}),
		row2("Vikram Shah", ui.Badge{Label: "Preparing", Tone: ui.Accent}),
		row2("Meera Iyer", ui.Badge{Label: "Waiting", Tone: ui.Caution}),
		row2("Rohan Das", ui.Badge{Label: "Refunded", Tone: ui.Bad}),
		row2("Priya Nair", ui.Badge{Label: "Draft"}),
		row2("Kabir Sen", ui.Badge{Label: "3", Tone: ui.Bad, Solid: true}))
}

type chips struct{}

func (chips) Render(c *ui.Ctx) *ui.Element {
	th := ui.UseTheme(c)
	on, setOn := ui.State(c, map[string]bool{"Vegan": true, "Coffee": true})
	tags, setTags := ui.State(c, []string{"Under ₹200", "Nearby"})
	var opts []ui.Component
	for _, n := range []string{"Vegan", "Coffee", "Tea", "Bakes", "Cold"} {
		opts = append(opts, ui.Key(n, ui.Chip{Label: n, Selected: on[n], OnPress: func() {
			next := map[string]bool{}
			for k, v := range on {
				next[k] = v
			}
			next[n] = !on[n]
			setOn(next)
		}}))
	}
	var applied []ui.Component
	for _, t := range tags {
		applied = append(applied, ui.Key(t, ui.Chip{Label: t, Icon: "tag", OnRemove: func() {
			var next []string
			for _, x := range tags {
				if x != t {
					next = append(next, x)
				}
			}
			setTags(next)
		}}))
	}
	return page(c, "Filters", "Narrow the menu down.",
		heading(th, "Show me"), row(8, opts...), heading(th, "Applied"), row(8, applied...))
}

type dividers struct{}

func (dividers) Render(c *ui.Ctx) *ui.Element {
	return page(c, "Receipt", "Table 7, 8:12 pm.",
		ui.ListRow{Title: "Flat white × 2", Trailing: ui.Text("₹440", ui.Style{TabularDigits: true})},
		ui.Divider{},
		ui.ListRow{Title: "Cardamom bun", Trailing: ui.Text("₹140", ui.Style{TabularDigits: true})},
		ui.Divider{Inset: 40},
		ui.ListRow{Title: "Total", Trailing: ui.Text("₹580", ui.Style{FontWeight: 700, TabularDigits: true})})
}

type listRows struct{}

func (listRows) Render(c *ui.Ctx) *ui.Element {
	on, setOn := ui.State(c, true)
	nop := func() {}
	return page(c, "Settings", "Your cafe, your way.",
		ui.ListRow{Title: "Notifications", Subtitle: "New orders and low stock", Icon: "bell", Trailing: ui.Switch(on, setOn, ui.Style{})},
		ui.ListRow{Title: "Payments", Subtitle: "Card, UPI and cash", Icon: "card", OnPress: nop},
		ui.ListRow{Title: "Team", Subtitle: "6 people", Icon: "users", OnPress: nop},
		ui.ListRow{Title: "Asha Rao", Subtitle: "Owner", Leading: ui.Avatar{Name: "Asha Rao"}, OnPress: nop})
}

type progress struct{}

func (progress) Render(c *ui.Ctx) *ui.Element {
	th := ui.UseTheme(c)
	return page(c, "Loyalty", "Seven more coffees to a free one.",
		ui.ProgressBar{Value: 0.65, Height: 10},
		heading(th, "This month's target"), ui.ProgressBar{Value: 0.82, Tone: ui.Good},
		heading(th, "Stock of oat milk"), ui.ProgressBar{Value: 0.18, Tone: ui.Bad},
		heading(th, "Syncing the menu"), ui.ProgressBar{Busy: true})
}

type skeletons struct{}

func (skeletons) Render(c *ui.Ctx) *ui.Element {
	line := func() *ui.Element {
		return ui.View(ui.Style{Direction: ui.Row, Gap: 12, AlignItems: ui.AlignCenter},
			ui.Skeleton{Height: 44, Circle: true},
			ui.View(ui.Style{Grow: 1, Gap: 8}, ui.Skeleton{Width: ui.Pct(70)}, ui.Skeleton{Width: ui.Pct(40), Height: 12}))
	}
	return page(c, "Orders", "Loading today's orders…", line(), line(), line(), ui.Skeleton{Height: 140, Radius: 16})
}

type empties struct{}

func (empties) Render(c *ui.Ctx) *ui.Element {
	return page(c, "Orders", "Nothing here right now.",
		ui.EmptyState{Icon: "cart", Title: "No orders yet", Message: "New orders show up here the moment they're placed.", Action: "Open the till", OnAction: func() {}})
}

type sheetBody struct{}

func (sheetBody) Render(c *ui.Ctx) *ui.Element {
	s, setS := ui.State(c, 1)
	return ui.View(ui.Style{Gap: 14},
		ui.SegmentedControl{Options: []string{"Small", "Regular", "Large"}, Selected: s, OnChange: setS},
		ui.Checkbox{Label: "Oat milk", Checked: true, OnChange: func(bool) {}},
		ui.Checkbox{Label: "Extra shot", OnChange: func(bool) {}},
		ui.Button{Label: "Add to basket · ₹220", Full: true, OnPress: func() {}})
}

type sheets struct{}

func (sheets) Render(c *ui.Ctx) *ui.Element {
	ov := ui.Overlays(c)
	open := func() { ov.Sheet(ui.Sheet{Title: "Flat white", Content: sheetBody{}, Height: 0.48}) }
	autoOpen(c, open)
	return page(c, "Menu", "Tap anything to choose.",
		ui.ListRow{Title: "Flat white", Subtitle: "₹220", Icon: "tag", OnPress: open})
}

type actionSheets struct{}

func (actionSheets) Render(c *ui.Ctx) *ui.Element {
	ov := ui.Overlays(c)
	open := func() {
		ov.ActionSheet("Order 1042",
			ui.MenuAction{Label: "Mark as ready", Icon: "check"},
			ui.MenuAction{Label: "Print receipt", Icon: "file"},
			ui.MenuAction{Label: "Cancel order", Icon: "trash", Destructive: true})
	}
	autoOpen(c, open)
	return page(c, "Orders", "Tap one for options.",
		ui.ListRow{Title: "Order 1042", Subtitle: "Table 7", Icon: "cart", OnPress: open})
}

// autoOpen opens an overlay as the screen appears, for pictures.
func autoOpen(c *ui.Ctx, f func()) {
	ui.Effect(c, func(context.Context) func() {
		if os.Getenv("SHOWCASE") != "" || os.Getenv("SHOWCASE_FILE") != "" {
			go func() {
				time.Sleep(400 * time.Millisecond)
				f()
			}()
		}
		return nil
	}, ui.Once)
}

type toasts struct{}

func (toasts) Render(c *ui.Ctx) *ui.Element {
	ov := ui.Overlays(c)
	open := func() { ov.Toast("Order placed", ui.Good) }
	autoOpen(c, open)
	return page(c, "Your order", "Two flat whites.",
		ui.Button{Label: "Place order", Full: true, OnPress: open},
		ui.Button{Label: "Out of stock", Variant: ui.Outlined, Full: true, OnPress: func() { ov.Toast("Oat milk has run out", ui.Bad) }})
}

type banners struct{}

func (banners) Render(c *ui.Ctx) *ui.Element {
	return page(c, "Till", "Taking orders.",
		ui.Banner{Tone: ui.Caution, Title: "You're offline", Message: "Orders will send when you're back."},
		ui.Banner{Tone: ui.Good, Title: "Payment received", Message: "₹580 from Asha Rao."},
		ui.Banner{Tone: ui.Bad, Title: "Printer not found", Action: "Try again", OnAction: func() {}, OnClose: func() {}},
		ui.Banner{Tone: ui.Accent, Title: "New menu tomorrow", OnClose: func() {}})
}

type menus struct{}

func (menus) Render(c *ui.Ctx) *ui.Element {
	return page(c, "Orders", "Tap the dots for more.",
		ui.ListRow{Title: "Order 1042", Subtitle: "Table 7", Icon: "cart", Trailing: ui.Menu{Trigger: ui.Icon("more", 26, 0), Items: []ui.MenuAction{
			{Label: "Edit", Icon: "edit"}, {Label: "Share", Icon: "share"}, {Label: "Delete", Icon: "trash", Destructive: true}}}})
}

type tooltips struct{}

func (tooltips) Render(c *ui.Ctx) *ui.Element {
	th := ui.UseTheme(c)
	return page(c, "Service charge", "Tap the icon to learn more.",
		row(8, ui.Text("Service charge", ui.Style{FontSize: 16, Color: th.Text}),
			ui.Tooltip{Text: "Shared between today's team.", Child: ui.Icon("info", 20, th.TextMuted)}))
}

type appBars struct{}

func (appBars) Render(c *ui.Ctx) *ui.Element {
	th, nop := ui.UseTheme(c), func() {}
	return ui.View(ui.Style{Grow: 1, Background: th.Background, Gap: 18},
		ui.AppBar{Title: "Order 1042", Subtitle: "Table 7", OnBack: nop, Actions: []ui.Component{
			ui.IconButton{Icon: "share", Label: "Share", OnPress: nop}, ui.IconButton{Icon: "more", Label: "More", OnPress: nop}}},
		ui.AppBar{Title: "Today", Subtitle: "Eleven orders so far", Large: true, Actions: []ui.Component{ui.IconButton{Icon: "search", Label: "Search", OnPress: nop}}})
}

type grids struct{}

func (grids) Render(c *ui.Ctx) *ui.Element {
	tile := func(a, b, n string) *ui.Element {
		return ui.View(ui.Style{Height: ui.Pt(150), Direction: ui.Row}, wall(a, b, n))
	}
	return page(c, "Bakes", "Fresh this morning.",
		ui.Grid{Columns: 2, Gap: 12, Items: []ui.Component{
			tile("#c98a5a", "#7a4a2a", "Cardamom bun"), tile("#8fb98a", "#3f6a4a", "Matcha loaf"),
			tile("#e0b15a", "#a8702a", "Almond croissant"), tile("#b56a7a", "#6a2a4a", "Rose cake")}})
}

type accordions struct{}

func (accordions) Render(c *ui.Ctx) *ui.Element {
	th := ui.UseTheme(c)
	para := func(s string) *ui.Element { return ui.Text(s, ui.Style{FontSize: 15, Color: th.TextMuted}) }
	return page(c, "Help", "The things people ask.",
		ui.Accordion{Items: []ui.AccordionItem{
			{Title: "When do you open?", Content: para("Every day from 7:30 am to 10 pm, and until midnight on Saturdays.")},
			{Title: "Do you take bookings?", Content: para("Yes, for up to eight people.")},
			{Title: "Is there vegan food?", Content: para("Most of the menu can be made vegan.")}}})
}

type carousels struct{}

func (carousels) Render(c *ui.Ctx) *ui.Element {
	w := ui.Viewport(c).W - 40
	return page(c, "Specials", "Swipe to see what's new.",
		ui.Carousel{Width: w, Height: 260, Pages: []ui.Component{
			wall("#0d7377", "#0a3a3c", "Cold brew, on tap"), wall("#c98a5a", "#5a3a2a", "Winter spice latte"), wall("#b56a7a", "#6a2a4a", "Rose cake")}})
}

type swipeRows struct{}

func (swipeRows) Render(c *ui.Ctx) *ui.Element {
	nop := func() {}
	item := func(n, s string) *ui.Element {
		return ui.C(ui.SwipeRow{Content: ui.ListRow{Title: n, Subtitle: s, Leading: ui.Avatar{Name: n}}, Actions: []ui.SwipeAction{
			{Label: "Done", Icon: "check", Tone: ui.Good, OnPress: nop}, {Label: "Delete", Icon: "trash", Tone: ui.Bad, OnPress: nop}}})
	}
	return page(c, "Orders", "Swipe a row to the left.", item("Asha Rao", "2 flat whites"), item("Vikram Shah", "Filter coffee"), item("Meera Iyer", "Masala chai"))
}

// gestures drags a tile, flicks a deck and pinches a square.
type gestures struct{}

func (gestures) Render(c *ui.Ctx) *ui.Element {
	th := ui.UseTheme(c)
	x, setX := ui.State(c, float32(0))
	y, setY := ui.State(c, float32(0))
	from := ui.Ref(c, [2]float32{})
	swipes, setSwipes := ui.State(c, "none yet")
	scale, setScale := ui.State(c, float32(1))
	return page(c, "Gestures", "Drag, flick and pinch.",
		ui.View(ui.Style{Height: ui.Pt(120)},
			ui.View(ui.Style{Absolute: true, Left: ui.Pt(x), Top: ui.Pt(y), Width: ui.Pt(80), Height: ui.Pt(80), Radius: 18,
				Background: th.Primary}).OnPan(func(p ui.Pan) {
				if p.Phase == ui.Began {
					*from = [2]float32{x, y}
				}
				setX(from[0] + p.X)
				setY(from[1] + p.Y)
			}).TestID("tile")),
		ui.Text("Swipe: "+swipes, ui.Style{FontWeight: 600, Color: th.Text}).TestID("swiped"),
		ui.View(ui.Style{Height: ui.Pt(90), Radius: 18, Background: th.Surface, Justify: ui.JustifyCenter, AlignItems: ui.AlignCenter},
			ui.Text("Flick me", ui.Style{Color: th.TextMuted})).OnSwipe(func(d ui.Swipe) {
			setSwipes(map[ui.Swipe]string{ui.SwipeLeft: "left", ui.SwipeRight: "right", ui.SwipeUp: "up", ui.SwipeDown: "down"}[d])
		}),
		ui.Text(fmt.Sprintf("Pinch: %.2f", scale), ui.Style{FontWeight: 600, Color: th.Text}).TestID("pinched"),
		ui.View(ui.Style{Height: ui.Pt(90), Radius: 18, Background: th.Surface}).OnPinch(func(p ui.Pinch) { setScale(p.Scale) }),
	)
}
