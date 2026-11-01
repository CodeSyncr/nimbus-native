// Package motion reads the accelerometer and gyroscope.
//
//	nimbus native plugin add motion
//
//	go motion.Watch(ctx, 30, func(r motion.Reading) { setTilt(r.Accel.X) })
package motion

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Vector is a reading on each axis, as the phone is held upright.
type Vector struct {
	X float64 `json:"x"` // across the screen, positive to the right
	Y float64 `json:"y"` // up the screen, positive to the top
	Z float64 `json:"z"` // out of the screen, positive toward you
}

// Reading is one sample from both sensors.
type Reading struct {
	Accel Vector `json:"accel"` // the accelerometer, in g, gravity included
	Gyro  Vector `json:"gyro"`  // the gyroscope, in radians a second
}

// Watch calls f about hz times a second until ctx ends. It blocks, so run it
// from a goroutine in an effect. Where there's no sensor, as on the iOS
// simulator, it fails with plugin.Unavailable.
//
//nimbus:param hz readings a second; 30 if it's 0 or less
//nimbus:param f called with each reading, on the goroutine that called Watch
//nimbus:returns why it stopped, which is ctx.Err() when ctx ends
func Watch(ctx context.Context, hz int, f func(Reading)) error {
	if hz <= 0 {
		hz = 30
	}
	if _, err := plugin.CallJSON(ctx, "motion", "start", map[string]int{"hz": hz}); err != nil {
		return err
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = plugin.CallJSON(c, "motion", "stop", nil)
	}()
	tick := time.NewTicker(time.Second / time.Duration(hz))
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
		r, err := plugin.Call[Reading](ctx, "motion", "read", nil)
		if err != nil {
			return err
		}
		f(r)
	}
}
