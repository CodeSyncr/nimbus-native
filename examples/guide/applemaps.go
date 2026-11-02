package guide

import (
	"github.com/CodeSyncr/nimbus-native/plugins/applemaps"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Directions shows the cafe on Apple Maps, and a list of the way on Android,
// where there's no Apple Maps.
type Directions struct{}

func (Directions) Render(c *ui.Ctx) ui.Component {
	tapped, setTapped := ui.State(c, "Tap the pin")
	cafe := applemaps.LatLng{Latitude: 18.5362, Longitude: 73.8930}
	return ui.SafeArea(ui.Style{Grow: 1},
		applemaps.Map{
			Center:   cafe,
			Zoom:     15,
			Markers:  []applemaps.Marker{{ID: "cafe", At: cafe, Title: "Monsoon Coffee", Subtitle: "Open till 11 pm"}},
			OnMarker: setTapped,
			Style:    ui.Style{Grow: 1},
			Fallback: ui.Text("Monsoon Coffee, Lane 6, Koregaon Park", ui.Style{Padding: ui.All(16)}),
		},
		ui.Text(tapped, ui.Style{Padding: ui.All(16)}))
}
