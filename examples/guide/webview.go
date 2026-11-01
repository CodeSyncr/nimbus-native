package guide

import (
	"github.com/CodeSyncr/nimbus-native/plugins/webview"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Reservations puts a booking page under a header of your own.
type Reservations struct{}

func (Reservations) Render(c *ui.Ctx) ui.Component {
	return ui.SafeArea(ui.Style{Grow: 1},
		ui.Text("Book a table", ui.Style{FontSize: 20, Padding: ui.All(16)}),
		webview.View{URL: "https://nimbusgo.space", Style: ui.Style{Grow: 1}})
}
