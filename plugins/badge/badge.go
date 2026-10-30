// Package badge sets the number on the app's icon. It works on iOS, where
// the first call asks for permission. Android has no such number of its
// own: launchers show one for unread notifications, so here it does nothing.
//
//	nimbus native plugin add badge
//
//	badge.Set(ctx, 3)
//	badge.Clear(ctx)
package badge

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Set puts n on the icon.
func Set(ctx context.Context, n int) error {
	_, err := plugin.CallJSON(ctx, "badge", "set", map[string]int{"count": n})
	return err
}

// Clear takes it off.
func Clear(ctx context.Context) error { return Set(ctx, 0) }
