// Package orientation locks the screen to portrait or landscape.
//
//	nimbus native plugin add orientation
//
//	orientation.Lock(ctx, orientation.Landscape)
//	orientation.Unlock(ctx)
package orientation

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Mode is a way up.
type Mode string

const (
	Portrait  Mode = "portrait"
	Landscape Mode = "landscape"
)

// Lock holds the screen in a mode.
func Lock(ctx context.Context, m Mode) error {
	_, err := plugin.CallJSON(ctx, "orientation", "set", map[string]Mode{"mode": m})
	return err
}

// Unlock follows the phone again.
func Unlock(ctx context.Context) error {
	_, err := plugin.CallJSON(ctx, "orientation", "set", map[string]Mode{"mode": ""})
	return err
}
