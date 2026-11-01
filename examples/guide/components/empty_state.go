package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// NoOrders says what to do when there's nothing yet.
type NoOrders struct{}

func (NoOrders) Render(c *ui.Ctx) ui.Component {
	created, setCreated := ui.State(c, false)
	if created {
		return ui.SafeArea(ui.Style{Padding: ui.All(24)}, ui.Text("Order 1042", ui.Style{FontSize: 20}))
	}
	return ui.SafeArea(ui.Style{Grow: 1, Justify: ui.JustifyCenter},
		ui.EmptyState{Icon: "cart", Title: "No orders yet", Message: "They'll show here as they come in.",
			Action: "New order", OnAction: func() { setCreated(true) }})
}
