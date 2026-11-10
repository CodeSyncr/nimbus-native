package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/keepawake"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Till keeps the screen on for as long as it's showing.
type Till struct{}

func (Till) Render(c *ui.Ctx) ui.Component {
	ui.Effect(c, func(ctx context.Context) func() {
		go keepawake.On(ctx)
		return func() { go keepawake.Off(context.WithoutCancel(ctx)) }
	}, ui.Once)
	return ui.SafeArea(ui.Style{Padding: ui.All(24)},
		ui.Text("This screen stays on.", ui.Style{FontSize: 20}))
}
