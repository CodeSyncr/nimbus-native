package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/dialog"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// CancelOrder asks first, and confirms with a toast.
type CancelOrder struct{}

func cancelOrder(ctx context.Context, _ struct{}) (string, error) {
	sure, err := dialog.Confirm(ctx, "Cancel this order?", "It can't be undone.", "Cancel order", true)
	if err != nil || !sure {
		return "Kept", err
	}
	return "Cancelled", dialog.Toast(ctx, "Order cancelled")
}

func (CancelOrder) Render(c *ui.Ctx) ui.Component {
	cancel := ui.UseAction(c, cancelOrder)
	status := cancel.Result
	if cancel.Err != nil {
		status = cancel.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Cancel order", Variant: ui.Destructive, OnPress: func() { cancel.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
