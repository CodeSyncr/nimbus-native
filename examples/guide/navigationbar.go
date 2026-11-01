package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/navigationbar"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Kiosk matches the bar to a dark screen, and hides it on request.
type Kiosk struct{}

func (Kiosk) Render(c *ui.Ctx) ui.Component {
	hidden, setHidden := ui.State(c, false)
	ui.Effect(c, func(ctx context.Context) func() {
		go func() {
			navigationbar.SetColor(ctx, ui.Hex("#10201f"))
			navigationbar.SetStyle(ctx, navigationbar.LightButtons)
		}()
		return func() { go navigationbar.Show(context.Background()) }
	}, ui.Once)
	toggle := func() {
		go func() {
			if hidden {
				navigationbar.Show(context.Background())
			} else {
				navigationbar.Hide(context.Background())
			}
		}()
		setHidden(!hidden)
	}
	label := "Hide the bar"
	if hidden {
		label = "Show the bar"
	}
	return ui.View(ui.Style{Grow: 1, Padding: ui.All(64), Background: ui.Hex("#10201f")},
		ui.Button{Label: label, OnPress: toggle})
}
