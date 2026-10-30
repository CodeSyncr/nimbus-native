// gallery shows the Phase 0d engine: components, state, every primitive,
// and a 1,000-row list, all rendered as native views.
package main

import (
	"fmt"
	"strings"

	"github.com/CodeSyncr/nimbus-native/ui"
	"github.com/CodeSyncr/nimbus-native/ui/render"
)

func init() { ui.Mount(App{}) }

// main runs only under `nimbus native dev`; on a device the host owns the process.
func main() { ui.Main() }

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

// App is the root component: two tabs, each with its own navigation stack.
type App struct{}

func (App) Render(c *ui.Ctx) *ui.Element {
	return ui.Tabs(ui.Flex(1),
		ui.Tab{Title: "Orders", Icon: "list.bullet", Screen: OrdersTab{}},
		ui.Tab{Title: "Account", Icon: "person.crop.circle", Screen: AccountTab{}},
	).TestID("tabs")
}

type OrdersTab struct{}

func (OrdersTab) Render(c *ui.Ctx) *ui.Element {
	return ui.Stack("Orders", OrdersScreen{}, ui.Flex(1)).TestID("stack")
}

type AccountTab struct{}

func (AccountTab) Render(c *ui.Ctx) *ui.Element {
	return ui.Stack("Account", AccountScreen{}, ui.Flex(1))
}

// AccountScreen is the second tab's first screen.
type AccountScreen struct{}

func (AccountScreen) Render(c *ui.Ctx) *ui.Element {
	tabs, th := ui.TabsOf(c), ui.UseTheme(c)
	return ui.SafeArea(ui.Style{Grow: 1, Padding: ui.All(th.SpaceL), Gap: th.SpaceM},
		ui.Text("Asha Rao", ui.Style{FontSize: th.Title, FontWeight: 700}).TestID("account-title"),
		ui.Text("asha@example.com · Pune", ui.Style{FontSize: th.Body, Color: th.TextMuted}),
		button(th, "Show my orders", func() { tabs.Select(0) }).TestID("to-orders"),
	)
}

// button is the gallery's primary button.
func button(th ui.Theme, label string, onPress func()) *ui.Element {
	return ui.Pressable(onPress,
		ui.Style{Background: th.Primary, Radius: th.Radius, Padding: ui.XY(16, 14), AlignItems: ui.AlignCenter},
		ui.Text(label, ui.Style{Color: th.OnPrimary, FontWeight: 600}),
	)
}

// OrdersScreen is the first screen.
type OrdersScreen struct{}

func (OrdersScreen) Render(c *ui.Ctx) *ui.Element {
	taps, setTaps := ui.State(c, 0)
	name, setName := ui.State(c, "")
	list := ui.Memo(c, orders, ui.Once)
	modals, th := ui.Modals(c), ui.UseTheme(c)

	greeting := "Type your name above"
	if name != "" {
		greeting = "Hello, " + name + " 👋"
	}

	return ui.SafeArea(ui.Style{Grow: 1},
		ui.View(ui.Style{Padding: ui.XY(16, 12), Gap: 12, Background: th.Surface},
			ui.View(ui.Style{Direction: ui.Row, AlignItems: ui.AlignCenter, Gap: 12},
				ui.Image("https://picsum.photos/id/1062/96", ui.Style{Width: ui.Pt(40), Height: ui.Pt(40), Radius: 20, Clip: true}),
				ui.View(ui.Flex(1),
					ui.Text("Nimbus Native", ui.Style{FontSize: 22, FontWeight: 700}),
					ui.Text(fmt.Sprintf("%d orders · rendered from Go", len(list)), ui.Style{FontSize: th.Caption, Color: th.TextMuted}),
				),
				ui.Spinner(ui.Style{}),
			),
			ui.View(ui.Style{Direction: ui.Row, Gap: 12, AlignItems: ui.AlignCenter},
				ui.Pressable(func() { setTaps(taps + 1) },
					ui.Style{Background: th.Primary, Radius: 10, Padding: ui.XY(16, 10)},
					ui.Text(fmt.Sprintf("Tapped %d times", taps), ui.Style{Color: th.OnPrimary, FontWeight: 600}),
				).TestID("counter").Label("Tap counter"),
				ui.TextInput(name, setName, ui.Flex(1).With(ui.Style{BorderWidth: 1, BorderColor: th.Border, Radius: 10, FontSize: th.Body})).
					Placeholder("Your name").TestID("name"),
			),
			ui.View(ui.Style{Direction: ui.Row, Justify: ui.JustifySpaceBetween, AlignItems: ui.AlignCenter},
				ui.Text(greeting, ui.Style{FontSize: th.Body}).TestID("greeting"),
				ui.Pressable(func() { modals.Present("New order", NewOrder{}) },
					ui.Style{BorderWidth: 1, BorderColor: th.Primary, Radius: 10, Padding: ui.XY(12, 6)},
					ui.Text("+ New order", ui.Style{Color: th.Primary, FontWeight: 600, FontSize: 14}),
				).TestID("new-order"),
			),
		),
		ui.List(len(list), func(i int) ui.Component { return ui.C(OrderRow{list[i]}) }, ui.Flex(1)).
			ItemKey(func(i int) string { return fmt.Sprint(list[i].id) }).TestID("list"),
	)
}

// OrderRow is one line of the list.
type OrderRow struct{ o order }

var badges = map[string]ui.Color{"Paid": ui.Hex("#dbeafe"), "Packed": ui.Hex("#fef3c7"),
	"Shipped": ui.Hex("#ede9fe"), "Delivered": ui.Hex("#dcfce7")}

func (r OrderRow) Render(c *ui.Ctx) *ui.Element {
	o := r.o
	nav, th := ui.Navigation(c), ui.UseTheme(c)
	return ui.Pressable(func() { nav.Push(fmt.Sprintf("Order #%d", o.id), OrderDetail{o}) },
		ui.Style{Direction: ui.Row, AlignItems: ui.AlignCenter, Gap: 12, Padding: ui.XY(16, 10),
			Background: th.Surface, Margin: ui.Edges{Bottom: 1}},
		ui.View(ui.Style{Width: ui.Pt(40), Height: ui.Pt(40), Radius: 20, Background: avatar(o.id),
			Justify: ui.JustifyCenter, AlignItems: ui.AlignCenter},
			ui.Text(o.customer[:1], ui.Style{Color: ui.RGB(255, 255, 255), FontWeight: 700}),
		),
		ui.View(ui.Flex(1).With(ui.Style{Gap: 2}),
			ui.Text(fmt.Sprintf("Order #%d · %s", o.id, o.customer), ui.Style{FontSize: th.Body, FontWeight: 600, Lines: 1}),
			ui.Text(fmt.Sprintf("%d items · ₹%.2f", o.items, o.total), ui.Style{FontSize: th.Caption, Color: th.TextMuted}),
		),
		ui.View(ui.Style{Background: badges[o.status], Radius: 8, Padding: ui.XY(8, 4)},
			ui.Text(o.status, ui.Style{FontSize: 12, FontWeight: 600, Color: ui.Hex("#111827")}), // dark on the pastel badge
		),
	).TestID(fmt.Sprintf("row-%d", o.id))
}

// NewOrder is the modal the "+ New order" button presents: a small form.
type NewOrder struct{}

type newOrderForm struct{ Customer, Phone string }

// validateNewOrder is the form's rules. An app talking to a Nimbus server
// can pass the server's own validation.ValidateStruct here instead.
func validateNewOrder(v newOrderForm) error {
	errs := ui.FieldErrors{}
	if len(strings.TrimSpace(v.Customer)) < 2 {
		errs["customer"] = []string{"Enter the customer's name"}
	}
	digits := 0
	for _, r := range v.Phone {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	if digits != 10 {
		errs["phone"] = []string{"Phone numbers have 10 digits"}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func (NewOrder) Render(c *ui.Ctx) *ui.Element {
	form := ui.UseForm(c, newOrderForm{}, validateNewOrder)
	modals, th := ui.Modals(c), ui.UseTheme(c)
	return ui.SafeArea(ui.Style{Grow: 1, Padding: ui.All(th.SpaceL), Gap: 16},
		ui.Text("Who is this order for?", ui.Style{FontSize: th.Heading, FontWeight: 600}).TestID("new-order-form"),
		ui.TextField(form.Field("customer", func(v *newOrderForm) *string { return &v.Customer }),
			ui.FieldOptions{Label: "Customer", Placeholder: "Customer name"}),
		ui.TextField(form.Field("phone", func(v *newOrderForm) *string { return &v.Phone }),
			ui.FieldOptions{Label: "Phone", Placeholder: "10-digit mobile", Keyboard: render.KeyboardNumber}),
		button(th, "Place order", form.Submit(func(newOrderForm) { modals.Dismiss() })).TestID("place-order"),
	)
}

// OrderDetail is the screen a row opens.
type OrderDetail struct{ o order }

func (d OrderDetail) Render(c *ui.Ctx) *ui.Element {
	o, nav, th := d.o, ui.Navigation(c), ui.UseTheme(c)
	field := func(label, value string) *ui.Element {
		return ui.View(ui.Style{Direction: ui.Row, Justify: ui.JustifySpaceBetween, Padding: ui.XY(0, 12)},
			ui.Text(label, ui.Style{FontSize: th.Body, Color: th.TextMuted}),
			ui.Text(value, ui.Style{FontSize: th.Body, FontWeight: 600}),
		)
	}
	return ui.View(ui.Style{Grow: 1, Padding: ui.All(th.SpaceL), Gap: 4},
		ui.View(ui.Style{Width: ui.Pt(64), Height: ui.Pt(64), Radius: 32, Background: avatar(o.id),
			Justify: ui.JustifyCenter, AlignItems: ui.AlignCenter, Margin: ui.Edges{Bottom: 12}},
			ui.Text(o.customer[:1], ui.Style{Color: ui.RGB(255, 255, 255), FontSize: 26, FontWeight: 700}),
		),
		ui.Text(o.customer, ui.Style{FontSize: th.Title, FontWeight: 700}).TestID("detail-title"),
		field("Order", fmt.Sprintf("#%d", o.id)),
		field("Items", fmt.Sprint(o.items)),
		field("Total", fmt.Sprintf("₹%.2f", o.total)),
		field("Status", o.status),
		ui.View(ui.Style{Margin: ui.Edges{Top: th.SpaceL}}, button(th, "Mark delivered", func() { nav.Pop() }).TestID("deliver")),
	)
}

func avatar(id int) ui.Color {
	palette := []ui.Color{ui.Hex("#2563eb"), ui.Hex("#db2777"), ui.Hex("#059669"), ui.Hex("#d97706"), ui.Hex("#7c3aed")}
	return palette[id%len(palette)]
}
