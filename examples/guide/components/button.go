package components

import (
	"context"
	"fmt"
	"time"

	"github.com/CodeSyncr/nimbus-native/ui"
)

// Checkout places an order, with a spinner while it works.
type Checkout struct{}

func placeOrder(ctx context.Context, id int) (string, error) {
	time.Sleep(800 * time.Millisecond) // call your server here
	return fmt.Sprintf("Order %d placed", id), nil
}

func (Checkout) Render(c *ui.Ctx) ui.Component {
	place := ui.UseAction(c, placeOrder)
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Place order", Icon: "check", Full: true, Loading: place.Running,
			OnPress: func() { place.Run(1042) }},
		ui.Button{Label: "Add a note", Variant: ui.Tonal, Full: true, OnPress: func() {}},
		ui.Button{Label: "Cancel order", Variant: ui.Destructive, Full: true, OnPress: func() {}},
		ui.Text(place.Result, ui.Style{}))
}
