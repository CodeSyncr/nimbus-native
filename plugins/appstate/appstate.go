// Package appstate says when the app goes to the background or comes back:
// to pause something, or refresh what's on screen.
//
//	nimbus native plugin add app-state
//
//	go appstate.Watch(ctx, func(active bool) {
//		if active {
//			refresh()
//		}
//	})
package appstate

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Active reports whether the app is in front.
func Active(ctx context.Context) (bool, error) {
	r, err := plugin.Call[struct {
		Active bool `json:"active"`
	}](ctx, "appstate", "get", nil)
	return r.Active, err
}

// Watch calls f when the app comes to the front or leaves it, until ctx ends.
// It blocks, so run it from a goroutine in an effect.
func Watch(ctx context.Context, f func(active bool)) error {
	last, err := Active(ctx)
	if err != nil {
		return err
	}
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
		now, err := Active(ctx)
		if err != nil {
			return err
		}
		if now != last {
			last = now
			f(now)
		}
	}
}
