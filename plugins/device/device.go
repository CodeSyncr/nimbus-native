// Package device describes the phone the app is running on.
//
//	nimbus native plugin add device
//
//	d, err := device.Get(ctx)
//	b, err := device.GetBattery(ctx)
package device

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Info is the phone.
type Info struct {
	Platform     string `json:"platform"`     // "ios" or "android"
	Model        string `json:"model"`        // e.g. "iPhone17,1", "Pixel 9"
	Manufacturer string `json:"manufacturer"` // e.g. "Apple", "Google"
	OSVersion    string `json:"osVersion"`
	Memory       int64  `json:"memory"`  // bytes
	Virtual      bool   `json:"virtual"` // a simulator or emulator
}

// Battery is how much charge is left.
type Battery struct {
	Level    float32 `json:"level"` // 0..1; below 0 when the phone can't say
	Charging bool    `json:"charging"`
}

// Get returns the phone's Info.
func Get(ctx context.Context) (Info, error) { return plugin.Call[Info](ctx, "device", "info", nil) }

// GetBattery returns the battery's state.
func GetBattery(ctx context.Context) (Battery, error) {
	return plugin.Call[Battery](ctx, "device", "battery", nil)
}
