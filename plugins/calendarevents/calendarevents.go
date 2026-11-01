// Package calendarevents reads and writes the phone's calendars directly. To
// add one event, the calendar plugin is simpler and needs no permission: it
// opens the system's screen. This is for showing someone's calendar in your
// app, or keeping it in step with your server.
//
//	nimbus native plugin add calendar-events
//
//	if ok, _ := calendarevents.Request(ctx); ok {
//		id, err := calendarevents.Create(ctx, calendarevents.Event{Title: "Table for two", Start: at})
//		week, err := calendarevents.List(ctx, time.Now(), time.Now().AddDate(0, 0, 7))
//	}
package calendarevents

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Calendar is one of their calendars.
type Calendar struct {
	ID       string `json:"id"`       // for Event.CalendarID
	Title    string `json:"title"`    // its name
	Writable bool   `json:"writable"` // whether you can add to it
	Default  bool   `json:"default"`  // where new events go
}

// Event is something in a calendar. A repeating event comes back once for
// each time it happens, all with the same ID.
type Event struct {
	ID         string    // set by Create; leave it empty for a new one
	CalendarID string    // empty means the default calendar
	Title      string    // its name
	Location   string    // where; optional
	Notes      string    // more detail; optional
	Start      time.Time // when it starts
	End        time.Time // the Start plus an hour when zero
	AllDay     bool      // it has no time of day
}

// wire is an Event as the phone sees it, with times in seconds.
type wire struct {
	ID         string  `json:"id"`
	CalendarID string  `json:"calendarId"`
	Title      string  `json:"title"`
	Location   string  `json:"location"`
	Notes      string  `json:"notes"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	AllDay     bool    `json:"allDay"`
}

func seconds(t time.Time) float64 { return float64(t.UnixMilli()) / 1000 }

func moment(s float64) time.Time { return time.UnixMilli(int64(s * 1000)) }

func (e Event) wire() wire {
	if e.End.IsZero() {
		e.End = e.Start.Add(time.Hour)
	}
	return wire{e.ID, e.CalendarID, e.Title, e.Location, e.Notes, seconds(e.Start), seconds(e.End), e.AllDay}
}

// Request asks to read and write their calendars, once.
func Request(ctx context.Context) (bool, error) {
	r, err := plugin.Call[struct {
		Granted bool `json:"granted"`
	}](ctx, "calendarevents", "request", nil)
	return r.Granted, err
}

// Calendars lists them. Call Request first, as with everything here.
//
//nimbus:returns their calendars
func Calendars(ctx context.Context) ([]Calendar, error) {
	return plugin.Call[[]Calendar](ctx, "calendarevents", "calendars", nil)
}

// NewCalendar makes a calendar of your own, kept on the phone: a good home
// for what your app adds.
//
//nimbus:param title its name
//nimbus:returns its ID
func NewCalendar(ctx context.Context, title string) (string, error) {
	r, err := plugin.Call[struct {
		ID string `json:"id"`
	}](ctx, "calendarevents", "newCalendar", map[string]string{"title": title})
	return r.ID, err
}

// DeleteCalendar removes a calendar and everything in it.
//
//nimbus:param id the Calendar's ID
func DeleteCalendar(ctx context.Context, id string) error {
	_, err := plugin.Call[any](ctx, "calendarevents", "deleteCalendar", map[string]string{"id": id})
	return err
}

// List gives the events between two times, earliest first.
//
//nimbus:param from the start of the period
//nimbus:param to the end of the period
//nimbus:returns the events
func List(ctx context.Context, from, to time.Time) ([]Event, error) {
	list, err := plugin.Call[[]wire](ctx, "calendarevents", "list", map[string]float64{"from": seconds(from), "to": seconds(to)})
	out := make([]Event, len(list))
	for i, w := range list {
		out[i] = Event{w.ID, w.CalendarID, w.Title, w.Location, w.Notes, moment(w.Start), moment(w.End), w.AllDay}
	}
	return out, err
}

// Create adds an event with no screen, so tell them first.
//
//nimbus:param e the event
//nimbus:returns its ID
func Create(ctx context.Context, e Event) (string, error) {
	r, err := plugin.Call[struct {
		ID string `json:"id"`
	}](ctx, "calendarevents", "create", e.wire())
	return r.ID, err
}

// Update replaces an event's details. For a repeating event it changes every
// time it happens.
//
//nimbus:param e the event, with its ID
func Update(ctx context.Context, e Event) error {
	_, err := plugin.Call[any](ctx, "calendarevents", "update", e.wire())
	return err
}

// Delete removes an event, and every repeat of it.
//
//nimbus:param id the Event's ID
func Delete(ctx context.Context, id string) error {
	_, err := plugin.Call[any](ctx, "calendarevents", "delete", map[string]string{"id": id})
	return err
}
