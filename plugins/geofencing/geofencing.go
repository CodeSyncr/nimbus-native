// Package geofencing notices when the phone enters or leaves a place, even
// while the app is in the background. Events wait on the phone until the
// app asks for them.
//
//	nimbus native plugin add geofencing
//
//	geofencing.Request(ctx)
//	geofencing.Add(ctx, geofencing.Region{ID: "cafe", Latitude: 18.53, Longitude: 73.89, Radius: 150})
//	go geofencing.Watch(ctx, func(e geofencing.Event) { greet(e) })
package geofencing

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Region is a circle on the map. Below about 100 metres it's unreliable.
type Region struct {
	ID        string  `json:"id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Radius    float64 `json:"radius"` // metres
}

// Event is a crossing of a Region's edge.
type Event struct {
	ID    string    `json:"id"`
	Enter bool      `json:"enter"` // false: left
	At    time.Time `json:"-"`
	Time  float64   `json:"time"` // seconds since 1970
}

// Request asks to watch the location in the background. Both platforms ask
// in two steps, and may send the person to Settings for the second.
func Request(ctx context.Context) (bool, error) {
	r, err := plugin.Call[struct {
		Granted bool `json:"granted"`
	}](ctx, "geofencing", "request", nil)
	return r.Granted, err
}

// Add starts watching a region. iOS allows 20 at a time.
func Add(ctx context.Context, r Region) error {
	_, err := plugin.CallJSON(ctx, "geofencing", "add", r)
	return err
}

// Remove stops watching a region.
func Remove(ctx context.Context, id string) error {
	_, err := plugin.CallJSON(ctx, "geofencing", "remove", map[string]string{"id": id})
	return err
}

// Events returns what has happened since the last call.
func Events(ctx context.Context) ([]Event, error) {
	r, err := plugin.Call[struct {
		Events []Event `json:"events"`
	}](ctx, "geofencing", "events", nil)
	for i := range r.Events {
		r.Events[i].At = time.Unix(int64(r.Events[i].Time), 0)
	}
	return r.Events, err
}

// Watch calls f with each event until ctx ends. It blocks, so run it from a
// goroutine in an effect.
func Watch(ctx context.Context, f func(Event)) error {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		es, err := Events(ctx)
		if err != nil {
			return err
		}
		for _, e := range es {
			f(e)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}
