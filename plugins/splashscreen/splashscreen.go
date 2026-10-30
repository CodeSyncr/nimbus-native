// Package splashscreen keeps the launch screen's color up until your first
// screen has what it needs, so the app doesn't flash empty.
//
//	nimbus native plugin add splash-screen
//
//	ui.Effect(c, func(ctx context.Context) func() {
//		go func() {
//			load(ctx)
//			splashscreen.Hide(ctx)
//		}()
//		return nil
//	}, ui.Once)
//
// It hides by itself after 10 seconds, so a bug can't leave the app stuck.
package splashscreen

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Hide fades the launch screen away.
func Hide(ctx context.Context) error {
	_, err := plugin.CallJSON(ctx, "splashscreen", "hide", nil)
	return err
}
