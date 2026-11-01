package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/plugins/screencapture"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// CardNumber is protected while it's showing, and counts any screenshots.
type CardNumber struct{}

func (CardNumber) Render(c *ui.Ctx) ui.Component {
	shots, setShots := ui.State(c, 0)
	seen := ui.Ref(c, 0)
	note, setNote := ui.State(c, "")
	ui.Effect(c, func(ctx context.Context) func() {
		go screencapture.Prevent(ctx)
		go func() {
			err := screencapture.Watch(ctx, func() {
				*seen++
				setShots(*seen)
			})
			if plugin.IsCode(err, plugin.Unavailable) {
				setNote("This phone can't report screenshots")
			}
		}()
		return func() { go screencapture.Allow(context.Background()) }
	}, ui.Once)
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Text("4242 4242 4242 4242", ui.Style{FontSize: 24}),
		ui.Text(fmt.Sprintf("Screenshots: %d", shots), ui.Style{}),
		ui.Text(note, ui.Style{}))
}
