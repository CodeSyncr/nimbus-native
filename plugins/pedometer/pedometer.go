// Package pedometer counts steps.
//
//	nimbus native plugin add pedometer
//
//	if ok, _ := pedometer.Request(ctx); ok {
//		go pedometer.Watch(ctx, setSteps)
//	}
package pedometer

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Available reports whether this phone has the sensor.
func Available(ctx context.Context) bool {
	r, err := plugin.Call[struct {
		Available bool `json:"available"`
	}](ctx, "pedometer", "available", nil)
	return err == nil && r.Available
}

// Request asks to read their steps, once.
func Request(ctx context.Context) (bool, error) {
	r, err := plugin.Call[struct {
		Granted bool `json:"granted"`
	}](ctx, "pedometer", "request", nil)
	return r.Granted, err
}

// Watch calls f with the steps taken since it began, each time the count
// changes, until ctx ends. It blocks, so run it from a goroutine in an effect.
// Where there's no sensor, as on the iOS simulator, it fails with
// plugin.Unavailable.
//
//nimbus:param f called with the steps so far, on the goroutine that called Watch
//nimbus:returns why it stopped, which is ctx.Err() when ctx ends
func Watch(ctx context.Context, f func(steps int)) error {
	if _, err := plugin.CallJSON(ctx, "pedometer", "start", nil); err != nil {
		return err
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = plugin.CallJSON(c, "pedometer", "stop", nil)
	}()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	last := -1
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
		r, err := plugin.Call[struct {
			Steps int `json:"steps"`
		}](ctx, "pedometer", "read", nil)
		if err != nil {
			return err
		}
		if r.Steps != last {
			last = r.Steps
			f(r.Steps)
		}
	}
}

// Between counts the steps taken in a past period. iOS keeps about a week;
// Android keeps nothing, so there it fails with plugin.Unavailable.
//
//nimbus:param from when it starts
//nimbus:param to when it ends
//nimbus:returns the steps taken
//nimbus:platforms iOS
func Between(ctx context.Context, from, to time.Time) (int, error) {
	r, err := plugin.Call[struct {
		Steps int `json:"steps"`
	}](ctx, "pedometer", "between", map[string]float64{
		"from": float64(from.UnixMilli()) / 1000, "to": float64(to.UnixMilli()) / 1000,
	})
	return r.Steps, err
}
