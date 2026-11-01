// Package navigationbar sets the look of the bar at the bottom of an Android
// phone, or hides it. iOS has no such bar, so there these calls do nothing,
// and shared code needn't check.
//
//	nimbus native plugin add navigation-bar
//
//	navigationbar.SetColor(ctx, ui.Hex("#10201f"))
//	navigationbar.SetStyle(ctx, navigationbar.LightButtons)
package navigationbar

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Style is the color of the bar's buttons.
type Style string

const (
	DarkButtons  Style = "dark"  // for a light bar
	LightButtons Style = "light" // for a dark bar
)

func set(ctx context.Context, args map[string]any) error {
	_, err := plugin.Call[any](ctx, "navigationbar", "set", args)
	return err
}

// SetStyle sets the buttons' color.
//
//nimbus:param s dark or light buttons
func SetStyle(ctx context.Context, s Style) error { return set(ctx, map[string]any{"style": s}) }

// SetColor sets the bar's color. Android 15 ignores it for apps that target
// it, and draws your screen behind the bar instead.
//
//nimbus:param c the bar's color
func SetColor(ctx context.Context, c ui.Color) error {
	return set(ctx, map[string]any{"color": uint32(c)})
}

// Hide hides the bar. A swipe up brings it back for a moment.
func Hide(ctx context.Context) error { return set(ctx, map[string]any{"hidden": true}) }

// Show brings it back.
func Show(ctx context.Context) error { return set(ctx, map[string]any{"hidden": false}) }
