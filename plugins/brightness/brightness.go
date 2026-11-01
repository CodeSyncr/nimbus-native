// Package brightness reads and sets how bright the screen is: up for a
// boarding pass or a QR code, down for reading in bed.
//
//	nimbus native plugin add brightness
//
//	brightness.Set(ctx, 1)
//	defer brightness.Reset(ctx)
package brightness

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Get says how bright the screen is, from 0 to 1.
func Get(ctx context.Context) (float64, error) {
	r, err := plugin.Call[struct {
		Level float64 `json:"level"`
	}](ctx, "brightness", "get", nil)
	return r.Level, err
}

// Set changes it. On Android that lasts while your app is on screen. On iOS
// it changes the phone's own setting, so call Reset before you leave.
//
//nimbus:param level how bright, from 0 to 1
func Set(ctx context.Context, level float64) error {
	_, err := plugin.Call[any](ctx, "brightness", "set", map[string]float64{"level": min(max(level, 0), 1)})
	return err
}

// Reset goes back to how it was before Set.
func Reset(ctx context.Context) error {
	_, err := plugin.Call[any](ctx, "brightness", "reset", nil)
	return err
}
