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

// Vector is a reading on each axis: x across the screen, y up it, z out of it.
type Vector struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// Reading is one moment.
type Reading struct {
	Accel Vector `json:"accel"` // in g, gravity included
	Gyro  Vector `json:"gyro"`  // radians a second
}

// Watch calls f about hz times a second until ctx ends. It blocks, so run it
// from a goroutine in an effect. Where there's no sensor, as on the iOS
// simulator, it fails with plugin.Unavailable.
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
