// Package statusbar sets the look of the status bar, the strip with the
// clock. By default it follows your Theme; these override that.
//
//	nimbus native plugin add status-bar
//
//	statusbar.SetStyle(ctx, statusbar.LightContent) // over a dark photo
//	statusbar.Hide(ctx)
package statusbar

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Style is the color of what's in the status bar.
type Style string

const (
	Auto         Style = "auto"  // follow the theme
	DarkContent  Style = "dark"  // dark icons, for a light background
	LightContent Style = "light" // light icons, for a dark background
)

func set(ctx context.Context, args map[string]any) error {
	_, err := plugin.CallJSON(ctx, "statusbar", "set", args)
	return err
}

// SetStyle sets the color of the clock and icons.
func SetStyle(ctx context.Context, s Style) error { return set(ctx, map[string]any{"style": s}) }

// Hide hides the status bar.
func Hide(ctx context.Context) error { return set(ctx, map[string]any{"hidden": true}) }

// Show brings it back.
func Show(ctx context.Context) error { return set(ctx, map[string]any{"hidden": false}) }

// SetColor sets the bar's background. Android only: on iOS the bar is
// transparent and your screen shows through it.
func SetColor(ctx context.Context, c ui.Color) error {
	return set(ctx, map[string]any{"color": uint32(c)})
}
