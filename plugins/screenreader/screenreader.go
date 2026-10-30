// Package screenreader works with VoiceOver and TalkBack: whether one is on,
// and saying something that changed on its own.
//
//	nimbus native plugin add screen-reader
//
//	on, err := screenreader.Enabled(ctx)
//	screenreader.Announce(ctx, "Order placed")
package screenreader

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Enabled reports whether a screen reader is on.
func Enabled(ctx context.Context) (bool, error) {
	r, err := plugin.Call[struct {
		Enabled bool `json:"enabled"`
	}](ctx, "screenreader", "enabled", nil)
	return r.Enabled, err
}

// Announce has the screen reader say text, if one is on.
func Announce(ctx context.Context, text string) error {
	_, err := plugin.CallJSON(ctx, "screenreader", "announce", map[string]string{"text": text})
	return err
}
