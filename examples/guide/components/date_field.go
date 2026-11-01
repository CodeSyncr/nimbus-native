package components

import (
	"time"

	"github.com/CodeSyncr/nimbus-native/plugins/dialog"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Reservation takes a date and a time with the platform's pickers.
type Reservation struct{}

func (Reservation) Render(c *ui.Ctx) ui.Component {
	day, setDay := ui.State(c, time.Time{})
	at, setAt := ui.State(c, time.Time{})
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.DateField{Label: "Date", Placeholder: "Choose a date", Value: day, OnChange: setDay},
		ui.DateField{Label: "Time", Placeholder: "Choose a time", Mode: dialog.Time, Layout: "3:04 pm", Value: at, OnChange: setAt})
}
