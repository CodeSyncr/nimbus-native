package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/plugins/lightsensor"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Ambient says how bright the room is. iOS has no such reading, so it gets a
// line of its own.
type Ambient struct{}

func (Ambient) Render(c *ui.Ctx) ui.Component {
	r, setReading := ui.State(c, lightsensor.Reading{})
	none, setNone := ui.State(c, false)
	ui.Effect(c, func(ctx context.Context) func() {
		go func() {
			err := lightsensor.Watch(ctx, 2, setReading)
			if plugin.IsCode(err, plugin.Unavailable) {
				setNone(true)
			}
		}()
		return nil
	}, ui.Once)
	if none {
		return ui.SafeArea(ui.Style{Padding: ui.All(24)}, ui.Text("This phone can't read the light.", ui.Style{}))
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24)},
		ui.Text(fmt.Sprintf("%.0f lux", r.Lux), ui.Style{FontSize: 28}))
}
