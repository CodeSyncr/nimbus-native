package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/geofencing"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Arrivals notices when they reach the cafe.
type Arrivals struct{}

func (Arrivals) Render(c *ui.Ctx) ui.Component {
	status, setStatus := ui.State(c, "Tap to start")
	watching := ui.Ref(c, false)
	start := func() {
		if *watching {
			return
		}
		*watching = true
		go func() {
			ctx := context.Background()
			if ok, err := geofencing.Request(ctx); err != nil || !ok {
				setStatus("Location is off")
				return
			}
			cafe := geofencing.Region{ID: "cafe", Latitude: 18.5362, Longitude: 73.8930, Radius: 150}
			if err := geofencing.Add(ctx, cafe); err != nil {
				setStatus(err.Error())
				return
			}
			setStatus("Watching the cafe")
			geofencing.Watch(ctx, func(e geofencing.Event) {
				if e.Enter {
					setStatus("You've arrived")
				} else {
					setStatus("See you soon")
				}
			})
		}()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Tell me when I arrive", OnPress: start},
		ui.Text(status, ui.Style{}))
}
