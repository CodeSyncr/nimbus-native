package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/statusbar"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// DarkHero sets light icons over a dark screen, and hands back on leaving.
type DarkHero struct{}

func (DarkHero) Render(c *ui.Ctx) ui.Component {
	ui.Effect(c, func(ctx context.Context) func() {
		go statusbar.SetStyle(ctx, statusbar.LightContent)
		return func() { go statusbar.SetStyle(context.Background(), statusbar.Auto) }
	}, ui.Once)
	return ui.View(ui.Style{Grow: 1, Padding: ui.All(64), Background: ui.Hex("#10201f")},
		ui.Text("Roastery", ui.Style{FontSize: 28, Color: ui.Hex("#ffffff")}))
}
