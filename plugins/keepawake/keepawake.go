// Package keepawake keeps the screen on, for a till, a recipe or a map.
//
//	nimbus native plugin add keep-awake
//
//	keepawake.On(ctx)
//	defer keepawake.Off(ctx)
package keepawake

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

func set(ctx context.Context, on bool) error {
	_, err := plugin.CallJSON(ctx, "keepawake", "set", map[string]bool{"on": on})
	return err
}

// On keeps the screen on while the app is in front.
func On(ctx context.Context) error { return set(ctx, true) }

// Off lets it sleep again.
func Off(ctx context.Context) error { return set(ctx, false) }
