package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugins/screenreader"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// OrderReady says out loud what changed by itself.
type OrderReady struct{}

func announce(ctx context.Context, text string) (string, error) {
	on, err := screenreader.Enabled(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Screen reader on: %v", on), screenreader.Announce(ctx, text)
}

func (OrderReady) Render(c *ui.Ctx) ui.Component {
	say := ui.UseAction(c, announce)
	status := say.Result
	if say.Err != nil {
		status = say.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Order is ready", OnPress: func() { say.Run("Your order is ready") }},
		ui.Text(status, ui.Style{}))
}
