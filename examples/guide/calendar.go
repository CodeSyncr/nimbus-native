package guide

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugins/calendar"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Booking adds a table to their calendar, on the system's screen.
type Booking struct{}

func addBooking(ctx context.Context, _ struct{}) (string, error) {
	saved, err := calendar.Add(ctx, calendar.Event{
		Title:    "Table for 2",
		Location: "Monsoon Coffee, Koregaon Park",
		Start:    time.Now().Add(24 * time.Hour),
		End:      time.Now().Add(26 * time.Hour),
	})
	if err != nil || !saved {
		return "Not added", err
	}
	return "Check your calendar", nil // Android can't say whether they saved it
}

func (Booking) Render(c *ui.Ctx) ui.Component {
	add := ui.UseAction(c, addBooking)
	status := add.Result
	if add.Err != nil {
		status = add.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Add to calendar", OnPress: func() { add.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
