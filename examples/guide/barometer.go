package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/plugins/barometer"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Floors shows the pressure, and how many floors they've climbed.
type Floors struct{}

func (Floors) Render(c *ui.Ctx) ui.Component {
	r, setReading := ui.State(c, barometer.Reading{})
	none, setNone := ui.State(c, false)
	ui.Effect(c, func(ctx context.Context) func() {
		go func() {
			err := barometer.Watch(ctx, 1, setReading)
			if plugin.IsCode(err, plugin.Unavailable) {
				setNone(true)
			}
		}()
		return nil
	}, ui.Once)
	if none {
		return ui.SafeArea(ui.Style{Padding: ui.All(24)}, ui.Text("This phone has no barometer.", ui.Style{}))
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Text(fmt.Sprintf("%.1f hPa", r.Pressure), ui.Style{FontSize: 28}),
		ui.Text(fmt.Sprintf("%.0f m climbed", r.Altitude), ui.Style{}))
}
