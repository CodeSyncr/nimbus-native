// The orders app: lists orders from the server and places new ones, with
// the server's validation errors shown on the form.
package main

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/api"
	"github.com/CodeSyncr/nimbus-native/examples/fullstack/shared"
	"github.com/CodeSyncr/nimbus-native/ui"
	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// The server runs on this Mac; Android reaches it through `adb reverse`.
var client = api.New("http://127.0.0.1:3400")

func init() { ui.Mount(App{}) }

func main() { ui.Main() }

type App struct{}

func (App) Render(c *ui.Ctx) *ui.Element {
	return ui.Stack("Orders", OrdersScreen{}, ui.Flex(1))
}

type OrdersScreen struct{}

func (OrdersScreen) Render(c *ui.Ctx) *ui.Element {
	th, modals := ui.UseTheme(c), ui.Modals(c)
	q := ui.UseQuery(c, "orders", func(ctx context.Context) ([]shared.Order, error) {
		return api.Call(ctx, client, shared.ListOrders, shared.ListOrdersReq{})
	})

	header := ui.View(ui.Style{Direction: ui.Row, Justify: ui.JustifySpaceBetween, AlignItems: ui.AlignCenter, Padding: ui.All(th.SpaceL)},
		ui.Text(fmt.Sprintf("%d orders from the server", len(q.Data)), ui.Style{FontSize: th.Body, Color: th.TextMuted}).TestID("count"),
		ui.Pressable(func() { modals.Present("New order", NewOrderScreen{}) },
			ui.Style{Background: th.Primary, Radius: 10, Padding: ui.XY(14, 8)},
			ui.Text("+ New", ui.Style{Color: th.OnPrimary, FontWeight: 600}),
		).TestID("new"),
	)
	var body *ui.Element
	switch {
	case q.Err != nil && !q.Loaded:
		body = ui.Text("Could not reach the server: "+q.Err.Error(), ui.Style{Color: th.Danger, Padding: ui.All(th.SpaceL)}).TestID("error")
	case !q.Loaded:
		body = ui.View(ui.Style{Padding: ui.All(th.SpaceL)}, ui.Spinner(ui.Style{}))
	default:
		orders := q.Data
		body = ui.List(len(orders), func(i int) *ui.Element { return ui.C(orderRow{orders[i]}) }, ui.Flex(1)).
			ItemKey(func(i int) string { return fmt.Sprint(orders[i].ID) })
	}
	return ui.SafeArea(ui.Style{Grow: 1}, header, body)
}

type orderRow struct{ o shared.Order }

func (r orderRow) Render(c *ui.Ctx) *ui.Element {
	th := ui.UseTheme(c)
	return ui.View(ui.Style{Direction: ui.Row, Justify: ui.JustifySpaceBetween, Padding: ui.XY(th.SpaceL, 14),
		Background: th.Surface, Margin: ui.Edges{Bottom: 1}},
		ui.View(ui.Style{Gap: 2},
			ui.Text(r.o.Customer, ui.Style{FontSize: th.Body, FontWeight: 600}).TestID(fmt.Sprint("order-", r.o.ID)),
			ui.Text(r.o.Phone, ui.Style{FontSize: th.Caption, Color: th.TextMuted}),
		),
		ui.Text(r.o.Status, ui.Style{FontSize: th.Caption, Color: th.Primary, FontWeight: 600}),
	)
}

// NewOrderScreen has no rules of its own: the server validates, and its 422
// lands on the fields through form.SetErrors.
type NewOrderScreen struct{}

func (NewOrderScreen) Render(c *ui.Ctx) *ui.Element {
	th, modals := ui.UseTheme(c), ui.Modals(c)
	form := ui.UseForm(c, shared.NewOrder{}, nil)
	place := ui.UseAction(c, func(ctx context.Context, o shared.NewOrder) (shared.Order, error) {
		created, err := api.Call(ctx, client, shared.CreateOrder, o)
		form.SetErrors(err)
		return created, err
	}, func(shared.Order) {
		ui.RefetchQuery(c, "orders")
		modals.Dismiss()
	})
	label := "Place order"
	if place.Running {
		label = "Placing…"
	}
	return ui.SafeArea(ui.Style{Grow: 1, Padding: ui.All(th.SpaceL), Gap: 16},
		ui.TextField(form.Field("customer", func(o *shared.NewOrder) *string { return &o.Customer }),
			ui.FieldOptions{Label: "Customer", Placeholder: "Customer name"}),
		ui.TextField(form.Field("phone", func(o *shared.NewOrder) *string { return &o.Phone }),
			ui.FieldOptions{Label: "Phone", Placeholder: "10-digit mobile", Keyboard: render.KeyboardNumber}),
		ui.Pressable(func() { place.Run(form.Value()) },
			ui.Style{Background: th.Primary, Radius: th.Radius, Padding: ui.XY(16, 14), AlignItems: ui.AlignCenter},
			ui.Text(label, ui.Style{Color: th.OnPrimary, FontWeight: 600}),
		).TestID("place"),
	)
}
