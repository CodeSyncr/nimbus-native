// Package lightsensor reads how bright it is around the phone. It's Android
// only: iOS doesn't let apps read this sensor.
//
//	nimbus native plugin add light-sensor
//
//	go lightsensor.Watch(ctx, 2, func(r lightsensor.Reading) { setDim(r.Lux < 10) })
package lightsensor

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Reading is one sample.
type Reading struct {
	Lux float64 `json:"lux"` // about 10 in a dim room, 400 in an office, 10,000 outdoors
}

// Available reports whether this phone has the sensor.
func Available(ctx context.Context) bool {
	r, err := plugin.Call[struct {
		Available bool `json:"available"`
	}](ctx, "lightsensor", "available", nil)
	return err == nil && r.Available
}

// Watch calls f about hz times a second until ctx ends. It blocks, so run it
// from a goroutine in an effect. Where there's no sensor it fails with
// plugin.Unavailable.
//
//nimbus:param hz readings a second; 2 if it's 0 or less
//nimbus:param f called with each reading, on the goroutine that called Watch
//nimbus:returns why it stopped, which is ctx.Err() when ctx ends
func Watch(ctx context.Context, hz int, f func(Reading)) error {
	if hz <= 0 {
		hz = 2
	}
	if _, err := plugin.CallJSON(ctx, "lightsensor", "start", map[string]int{"hz": hz}); err != nil {
		return err
	}
	defer func() {
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_, _ = plugin.CallJSON(c, "lightsensor", "stop", nil)
	}()
	tick := time.NewTicker(time.Second / time.Duration(hz))
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
		r, err := plugin.Call[Reading](ctx, "lightsensor", "read", nil)
		if err != nil {
			return err
		}
		f(r)
	}
}
