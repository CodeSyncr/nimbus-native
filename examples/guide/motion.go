package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/plugins/motion"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Level shows the tilt on each axis, live.
type Level struct{}

func (Level) Render(c *ui.Ctx) ui.Component {
	r, setReading := ui.State(c, motion.Reading{})
	on, setOn := ui.State(c, true)
	none, setNone := ui.State(c, false)

	ui.Effect(c, func(ctx context.Context) func() {
		if on {
			go func() {
				err := motion.Watch(ctx, 30, setReading)
				if plugin.IsCode(err, plugin.Unavailable) {
					setNone(true)
				}
			}()
		}
		return nil
	}, on)

	if none {
		return ui.SafeArea(ui.Style{Grow: 1, Padding: ui.All(24)},
			ui.Text("This phone has no motion sensors.", ui.Style{}))
	}
	label := "Stop"
	if !on {
		label = "Start"
	}
	return ui.SafeArea(ui.Style{Grow: 1, Padding: ui.All(24), Gap: 12},
		ui.Text(fmt.Sprintf("x %.2f g", r.Accel.X), ui.Style{FontSize: 28}),
		ui.Text(fmt.Sprintf("y %.2f g", r.Accel.Y), ui.Style{FontSize: 28}),
		ui.Text(fmt.Sprintf("z %.2f g", r.Accel.Z), ui.Style{FontSize: 28}),
		ui.Button{Label: label, OnPress: func() { setOn(!on) }},
	)
}
