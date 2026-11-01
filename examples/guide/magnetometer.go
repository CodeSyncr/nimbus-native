package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/plugins/magnetometer"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Detector shows when something magnetic is close.
type Detector struct{}

func (Detector) Render(c *ui.Ctx) ui.Component {
	r, setReading := ui.State(c, magnetometer.Reading{})
	none, setNone := ui.State(c, false)
	ui.Effect(c, func(ctx context.Context) func() {
		go func() {
			err := magnetometer.Watch(ctx, 10, setReading)
			if plugin.IsCode(err, plugin.Unavailable) {
				setNone(true)
			}
		}()
		return nil
	}, ui.Once)
	if none {
		return ui.SafeArea(ui.Style{Padding: ui.All(24)}, ui.Text("This phone has no magnetometer.", ui.Style{}))
	}
	verdict := "Nothing nearby"
	if r.Strength() > 100 {
		verdict = "Something's close"
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Text(fmt.Sprintf("%.0f µT", r.Strength()), ui.Style{FontSize: 28}),
		ui.Text(verdict, ui.Style{}))
}
