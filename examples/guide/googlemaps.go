package guide

import (
	"github.com/CodeSyncr/nimbus-native/plugins/googlemaps"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Delivery shows the cafe, and says which pin was tapped.
type Delivery struct{}

func (Delivery) Render(c *ui.Ctx) ui.Component {
	tapped, setTapped := ui.State(c, "Tap a pin")
	cafe := googlemaps.LatLng{Latitude: 18.5362, Longitude: 73.8930}
	return ui.SafeArea(ui.Style{Grow: 1},
		googlemaps.Map{
			Center:   cafe,
			Zoom:     14,
			Markers:  []googlemaps.Marker{{ID: "cafe", At: cafe, Title: "Monsoon Coffee"}},
			OnMarker: setTapped,
			Style:    ui.Style{Grow: 1},
		},
		ui.Text(tapped, ui.Style{Padding: ui.All(16)}))
}
