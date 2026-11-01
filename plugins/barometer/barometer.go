// Package barometer reads the air pressure, and how far the phone has climbed.
//
//	nimbus native plugin add barometer
//
//	go barometer.Watch(ctx, 1, func(r barometer.Reading) { setFloors(r.Altitude / 3) })
package barometer

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Reading is one sample.
type Reading struct {
	Pressure float64 `json:"pressure"` // in hectopascals; about 1013 at sea level
	Altitude float64 `json:"altitude"` // metres climbed since Watch began; negative is down
}

// Available reports whether this phone has the sensor.
func Available(ctx context.Context) bool {
	r, err := plugin.Call[struct {
		Available bool `json:"available"`
	}](ctx, "barometer", "available", nil)
	return err == nil && r.Available
}

// Watch calls f about hz times a second until ctx ends. It blocks, so run it
// from a goroutine in an effect. Where there's no sensor it fails with
// plugin.Unavailable.
//
//nimbus:param hz readings a second; 1 if it's 0 or less
//nimbus:param f called with each reading, on the goroutine that called Watch
//nimbus:returns why it stopped, which is ctx.Err() when ctx ends
func Watch(ctx context.Context, hz int, f func(Reading)) error {
	if hz <= 0 {
		hz = 1
	}
	if _, err := plugin.CallJSON(ctx, "barometer", "start", map[string]int{"hz": hz}); err != nil {
		return err
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = plugin.CallJSON(c, "barometer", "stop", nil)
	}()
	tick := time.NewTicker(time.Second / time.Duration(hz))
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
		r, err := plugin.Call[Reading](ctx, "barometer", "read", nil)
		if err != nil {
			return err
		}
		f(r)
	}
}
