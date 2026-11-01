// Package backgroundlocation keeps following the phone after the person
// leaves your app: a delivery, a run, a trip. It runs where they can see it,
// with a notification on Android and the location indicator on iOS, so it
// needs no "always allow" permission. Start it from a screen on show.
//
//	nimbus native plugin add background-location
//
//	if ok, _ := backgroundlocation.Request(ctx); ok {
//		backgroundlocation.Start(ctx, backgroundlocation.Options{Title: "Trip running", MinDistance: 25})
//	}
//	points, err := backgroundlocation.Points(ctx)
//	backgroundlocation.Stop(ctx)
//
// To notice when someone arrives somewhere, geofencing does that with far
// less battery.
package backgroundlocation

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Options say how to follow.
type Options struct {
	Title       string  `json:"title,omitempty"`       // Android's notification heading
	Text        string  `json:"text,omitempty"`        // Android's notification text
	MinDistance float64 `json:"minDistance,omitempty"` // metres to move before the next point; 0 means 10
}

// Point is one place it passed.
type Point struct {
	Latitude  float64   `json:"latitude"`  // in degrees
	Longitude float64   `json:"longitude"` // in degrees
	Accuracy  float64   `json:"accuracy"`  // metres
	Speed     float64   `json:"speed"`     // metres a second; below 0 when unknown
	At        time.Time `json:"-"`         // when it was there
	Time      float64   `json:"time"`      // the same, in seconds since 1970
}

// Request asks to use the location, and on Android to show its notification.
func Request(ctx context.Context) (bool, error) {
	r, err := plugin.Call[struct {
		Granted bool `json:"granted"`
	}](ctx, "backgroundlocation", "request", nil)
	return r.Granted, err
}

// Start begins following, until Stop. Call it from a screen on show: both
// systems refuse to begin from the background. Without permission it fails
// with plugin.Denied.
//
//nimbus:param o how to follow
func Start(ctx context.Context, o Options) error {
	if o.MinDistance <= 0 {
		o.MinDistance = 10
	}
	_, err := plugin.Call[any](ctx, "backgroundlocation", "start", o)
	return err
}

// Stop ends it. Points not yet read stay there for Points.
func Stop(ctx context.Context) error {
	_, err := plugin.Call[any](ctx, "backgroundlocation", "stop", nil)
	return err
}

// Running reports whether it's following.
func Running(ctx context.Context) (bool, error) {
	r, err := plugin.Call[struct {
		Running bool `json:"running"`
	}](ctx, "backgroundlocation", "running", nil)
	return r.Running, err
}

// Points gives what's been collected since the last call, oldest first. The
// phone holds the newest 5,000, in memory, so read them when the app returns.
//
//nimbus:returns the points
func Points(ctx context.Context) ([]Point, error) {
	list, err := plugin.Call[[]Point](ctx, "backgroundlocation", "points", nil)
	for i := range list {
		list[i].At = time.UnixMilli(int64(list[i].Time * 1000))
	}
	return list, err
}

// Watch calls f with new points until ctx ends. It blocks, so run it from a
// goroutine in an effect.
//
//nimbus:param f called with each batch of points
//nimbus:returns why it stopped, which is ctx.Err() when ctx ends
func Watch(ctx context.Context, f func([]Point)) error {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
		list, err := Points(ctx)
		if err != nil {
			return err
		}
		if len(list) > 0 {
			f(list)
		}
	}
}
