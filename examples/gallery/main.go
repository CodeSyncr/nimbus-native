// gallery shows the Phase 0d engine: components, state, every primitive,
// and a 1,000-row list, all rendered as native views.
package main

import (
	"fmt"

	"github.com/CodeSyncr/nimbus-native/ui"
)

func init() { ui.Mount(App{}) }

// main is unused: the app is built as a library and the host owns the process.
func main() {}

var (
	ink     = ui.Hex("#111827")
	muted   = ui.Hex("#6b7280")
	line    = ui.Hex("#e5e7eb")
	accent  = ui.Hex("#2563eb")
	surface = ui.Hex("#f9fafb")
)

type order struct {
	id       int
	customer string
	items    int
	total    float64
	status   string
}

var statuses = []string{"Paid", "Packed", "Shipped", "Delivered"}
var names = []string{"Asha Rao", "Vikram Shah", "Meera Iyer", "Rohan Das", "Priya Nair", "Kabir Mehta"}

func orders() []order {
	out := make([]order, 1000)
	for i := range out {
		out[i] = order{id: 10000 + i, customer: names[i%len(names)], items: 1 + i%5,
			total: float64(199+(i*37)%2400) + 0.5, status: statuses[i%len(statuses)]}
	}
	return out
}

// App is the root component.
type App struct{}

func (App) Render(c *ui.Ctx) *ui.Element {
	vp := ui.Viewport(c)
	taps, setTaps := ui.State(c, 0)
	name, setName := ui.State(c, "")
	list := ui.Memo(c, orders, ui.Once)

	greeting := "Type your name above"
	if name != "" {
		greeting = "Hello, " + name + " 👋"
	}

	rows := make([]*ui.Element, len(list))
	for i, o := range list {
		rows[i] = ui.C(OrderRow{o}).Key(fmt.Sprint(o.id))
	}

	return ui.View(ui.Style{Background: surface, Padding: ui.Edges{Top: vp.SafeTop, Bottom: vp.SafeBottom}},
		ui.View(ui.Style{Padding: ui.XY(16, 12), Gap: 12, Background: ui.RGB(255, 255, 255), BorderWidth: 0},
			ui.View(ui.Style{Direction: ui.Row, AlignItems: ui.AlignCenter, Gap: 12},
				ui.Image("https://picsum.photos/id/1062/96", ui.Style{Width: ui.Pt(40), Height: ui.Pt(40), Radius: 20, Clip: true}),
				ui.View(ui.Flex(1),
					ui.Text("Nimbus Native", ui.Style{FontSize: 22, FontWeight: 700, Color: ink}),
					ui.Text(fmt.Sprintf("%d orders · rendered from Go", len(list)), ui.Style{FontSize: 13, Color: muted}),
				),
				ui.Spinner(ui.Style{Color: accent}),
			),
			ui.View(ui.Style{Direction: ui.Row, Gap: 12, AlignItems: ui.AlignCenter},
				ui.Pressable(func() { setTaps(taps + 1) },
					ui.Style{Background: accent, Radius: 10, Padding: ui.XY(16, 10)},
					ui.Text(fmt.Sprintf("Tapped %d times", taps), ui.Style{Color: ui.RGB(255, 255, 255), FontWeight: 600}),
				).TestID("counter").Label("Tap counter"),
				ui.TextInput(name, setName, ui.Flex(1).With(ui.Style{BorderWidth: 1, BorderColor: line, Radius: 10, FontSize: 15})).
					Placeholder("Your name").TestID("name"),
			),
			ui.Text(greeting, ui.Style{FontSize: 15, Color: ink}).TestID("greeting"),
		),
		ui.Scroll(ui.Flex(1), rows...).TestID("list"),
	)
}

// OrderRow is one line of the list.
type OrderRow struct{ o order }

func (r OrderRow) Render(c *ui.Ctx) *ui.Element {
	o := r.o
	badge := map[string]ui.Color{"Paid": ui.Hex("#dbeafe"), "Packed": ui.Hex("#fef3c7"),
		"Shipped": ui.Hex("#ede9fe"), "Delivered": ui.Hex("#dcfce7")}[o.status]
	return ui.View(ui.Style{Direction: ui.Row, AlignItems: ui.AlignCenter, Gap: 12, Padding: ui.XY(16, 10),
		Background: ui.RGB(255, 255, 255), Margin: ui.Edges{Bottom: 1}},
		ui.View(ui.Style{Width: ui.Pt(40), Height: ui.Pt(40), Radius: 20, Background: avatar(o.id),
			Justify: ui.JustifyCenter, AlignItems: ui.AlignCenter},
			ui.Text(o.customer[:1], ui.Style{Color: ui.RGB(255, 255, 255), FontWeight: 700}),
		),
		ui.View(ui.Flex(1).With(ui.Style{Gap: 2}),
			ui.Text(fmt.Sprintf("Order #%d · %s", o.id, o.customer), ui.Style{FontSize: 15, FontWeight: 600, Color: ink, Lines: 1}),
			ui.Text(fmt.Sprintf("%d items · ₹%.2f", o.items, o.total), ui.Style{FontSize: 13, Color: muted}),
		),
		ui.View(ui.Style{Background: badge, Radius: 8, Padding: ui.XY(8, 4)},
			ui.Text(o.status, ui.Style{FontSize: 12, FontWeight: 600, Color: ink}),
		),
	)
}

func avatar(id int) ui.Color {
	palette := []ui.Color{ui.Hex("#2563eb"), ui.Hex("#db2777"), ui.Hex("#059669"), ui.Hex("#d97706"), ui.Hex("#7c3aed")}
	return palette[id%len(palette)]
}
