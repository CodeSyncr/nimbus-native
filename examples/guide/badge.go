package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/badge"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Unread keeps the number on the app's icon.
type Unread struct{}

func setBadge(ctx context.Context, n int) (string, error) {
	if n == 0 {
		return "Cleared", badge.Clear(ctx)
	}
	return "The icon says 3", badge.Set(ctx, n)
}

func (Unread) Render(c *ui.Ctx) ui.Component {
	set := ui.UseAction(c, setBadge)
	status := set.Result
	if set.Err != nil {
		status = set.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "3 unread", OnPress: func() { set.Run(3) }},
		ui.Button{Label: "Mark all as read", Variant: ui.Tonal, OnPress: func() { set.Run(0) }},
		ui.Text(status, ui.Style{}))
}
