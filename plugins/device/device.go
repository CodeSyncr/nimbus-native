// Package device describes the phone the app is running on.
//
//	nimbus native plugin add device
//
//	d, err := device.Get(ctx)
//	b, err := device.GetBattery(ctx)
package device

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Info is the phone.
type Info struct {
	Platform     string `json:"platform"`     // "ios", "android" or "web"
	Model        string `json:"model"`        // e.g. "iPhone17,1", "Pixel 9"
	Manufacturer string `json:"manufacturer"` // e.g. "Apple", "Google"
	OSVersion    string `json:"osVersion"`    // the system version
	Memory       int64  `json:"memory"`       // bytes
	Virtual      bool   `json:"virtual"`      // a simulator or emulator
}

// Battery is how much charge is left.
type Battery struct {
	Level    float32 `json:"level"`    // 0..1; below 0 when the phone can't say
	Charging bool    `json:"charging"` // whether it is plugged in
	// LowPower is the phone's power saving mode: a good moment to do less.
	LowPower bool `json:"lowPower"`
}

// Get returns the phone's Info.
func Get(ctx context.Context) (Info, error) { return plugin.Call[Info](ctx, "device", "info", nil) }

// GetBattery returns the battery's state.
func GetBattery(ctx context.Context) (Battery, error) {
	return plugin.Call[Battery](ctx, "device", "battery", nil)
}

// WatchBattery calls f with the battery now and whenever it changes, until
// ctx ends. It blocks, so run it from a goroutine in an effect.
//
//nimbus:param f called with each change
//nimbus:returns why it stopped, which is ctx.Err() when ctx ends
func WatchBattery(ctx context.Context, f func(Battery)) error {
	var last Battery
	first := true
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		b, err := GetBattery(ctx)
		if err != nil {
			return err
		}
		if first || b != last {
			first, last = false, b
			f(b)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}
