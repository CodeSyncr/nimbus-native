package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugins/appstate"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// LiveOrders counts how often the app comes back, the moment to refresh
// anything that may have gone stale.
type LiveOrders struct{}

func (LiveOrders) Render(c *ui.Ctx) ui.Component {
	active, setActive := ui.State(c, true)
	returns, setReturns := ui.State(c, 0)
	seen := ui.Ref(c, 0)
	ui.Effect(c, func(ctx context.Context) func() {
		go appstate.Watch(ctx, func(on bool) {
			setActive(on)
			if on {
				*seen++
				setReturns(*seen)
			}
		})
		return nil
	}, ui.Once)
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 8},
		ui.Text(fmt.Sprintf("Active: %v", active), ui.Style{FontSize: 20}),
		ui.Text(fmt.Sprintf("Came back %d times", returns), ui.Style{}))
}
