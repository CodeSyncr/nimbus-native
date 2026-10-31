// Package calendar adds an event with the system's own screen, so people
// see it and choose the calendar before it's saved.
//
//	nimbus native plugin add calendar
//
//	calendar.Add(ctx, calendar.Event{Title: "Table for two", Start: at, End: at.Add(time.Hour)})
package calendar

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Event is something in a calendar.
type Event struct {
	Title    string
	Location string
	Notes    string
	Start    time.Time
	End      time.Time // the Start plus an hour when zero
}

// Add opens the screen and returns when it closes: true if they saved it.
// Android's calendar doesn't say, so there it's true once the screen closes.
func Add(ctx context.Context, e Event) (bool, error) {
	if e.End.IsZero() {
		e.End = e.Start.Add(time.Hour)
	}
	r, err := plugin.Call[struct {
		Saved bool `json:"saved"`
	}](ctx, "calendar", "add", map[string]any{"title": e.Title, "location": e.Location, "notes": e.Notes,
		"start": e.Start.Unix(), "end": e.End.Unix()})
	return r.Saved, err
}
