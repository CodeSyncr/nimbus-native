package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/orientation"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// KitchenScreen holds the screen sideways while it's showing.
type KitchenScreen struct{}

func (KitchenScreen) Render(c *ui.Ctx) ui.Component {
	ui.Effect(c, func(ctx context.Context) func() {
		go orientation.Lock(ctx, orientation.Landscape)
		return func() { go orientation.Unlock(context.Background()) }
	}, ui.Once)
	return ui.SafeArea(ui.Style{Padding: ui.All(24)},
		ui.Text("Orders, sideways", ui.Style{FontSize: 20}))
}
