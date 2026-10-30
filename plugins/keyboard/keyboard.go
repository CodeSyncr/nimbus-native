// Package keyboard puts the on-screen keyboard away. While it's up,
// ui.Viewport(c).Keyboard is its height, and ui.SafeArea keeps clear of it.
//
//	nimbus native plugin add keyboard
//
//	keyboard.Dismiss(ctx)
package keyboard

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Dismiss puts the keyboard away.
func Dismiss(ctx context.Context) error {
	_, err := plugin.CallJSON(ctx, "keyboard", "dismiss", nil)
	return err
}
