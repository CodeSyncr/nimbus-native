// Package magnetometer reads the magnetic field around the phone: for a
// metal detector, say, or a compass of your own.
//
//	nimbus native plugin add magnetometer
//
//	go magnetometer.Watch(ctx, 10, func(r magnetometer.Reading) { setField(r.Strength()) })
package magnetometer

import (
	"context"
	"math"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Reading is the field on each axis, in microteslas, as the phone is held
// upright.
type Reading struct {
	X float64 `json:"x"` // across the screen, positive to the right
	Y float64 `json:"y"` // up the screen, positive to the top
	Z float64 `json:"z"` // out of the screen, positive toward you
}

// Strength is the field's total size, in microteslas. The Earth's is about
// 25 to 65.
func (r Reading) Strength() float64 { return math.Sqrt(r.X*r.X + r.Y*r.Y + r.Z*r.Z) }

// Available reports whether this phone has the sensor.
func Available(ctx context.Context) bool {
	r, err := plugin.Call[struct {
		Available bool `json:"available"`
	}](ctx, "magnetometer", "available", nil)
	return err == nil && r.Available
}

// Watch calls f about hz times a second until ctx ends. It blocks, so run it
// from a goroutine in an effect. Where there's no sensor, as on the iOS
// simulator, it fails with plugin.Unavailable.
//
//nimbus:param hz readings a second; 10 if it's 0 or less
//nimbus:param f called with each reading, on the goroutine that called Watch
//nimbus:returns why it stopped, which is ctx.Err() when ctx ends
func Watch(ctx context.Context, hz int, f func(Reading)) error {
	if hz <= 0 {
		hz = 10
	}
	if _, err := plugin.CallJSON(ctx, "magnetometer", "start", map[string]int{"hz": hz}); err != nil {
		return err
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = plugin.CallJSON(c, "magnetometer", "stop", nil)
	}()
	tick := time.NewTicker(time.Second / time.Duration(hz))
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
		r, err := plugin.Call[Reading](ctx, "magnetometer", "read", nil)
		if err != nil {
			return err
		}
		f(r)
	}
}
