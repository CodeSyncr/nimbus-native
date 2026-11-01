package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/splashscreen"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// FirstScreen keeps the launch color up until its data is here.
type FirstScreen struct{}

func loadMenu(ctx context.Context) (string, error) {
	return "Today's menu", nil // fetch it from your server here
}

func (FirstScreen) Render(c *ui.Ctx) ui.Component {
	menu := ui.UseQuery(c, "menu", loadMenu)
	ui.Effect(c, func(ctx context.Context) func() {
		if !menu.Loading {
			go splashscreen.Hide(ctx)
		}
		return nil
	}, menu.Loading)
	return ui.SafeArea(ui.Style{Padding: ui.All(24)},
		ui.Text(menu.Data, ui.Style{FontSize: 20}))
}
