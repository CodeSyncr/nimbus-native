package guide

import (
	"github.com/CodeSyncr/nimbus-native/plugins/splitview"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// OrdersSplit lists orders beside the open one on a tablet, and pushes the
// open one on a phone.
type OrdersSplit struct{}

type orderDetail struct{ id string }

func (o orderDetail) Render(c *ui.Ctx) ui.Component {
	return ui.View(ui.Style{Padding: ui.All(24), Gap: 8},
		ui.Text("Order "+o.id, ui.Style{FontSize: 28, FontWeight: 700}),
		ui.Text("Latte, oat milk. ₹180.", ui.Style{}))
}

func (OrdersSplit) Render(c *ui.Ctx) ui.Component {
	split := splitview.Use(c)
	var rows []ui.Component
	for _, id := range []string{"1042", "1041", "1040"} {
		id := id
		rows = append(rows, ui.Pressable(func() { split.Open(id) }, ui.Style{},
			ui.ListRow{Title: "Order " + id, Subtitle: "Latte, oat milk"}))
	}
	return ui.Stack("Orders", splitview.View{
		Split:  split,
		List:   ui.View(ui.Style{}, rows...),
		Detail: func(id string) ui.Component { return orderDetail{id} },
		Title:  func(id string) string { return "Order " + id },
		Empty:  ui.Text("Pick an order", ui.Style{}),
		Style:  ui.Style{Grow: 1},
	}, ui.Flex(1))
}
