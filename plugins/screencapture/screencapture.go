// Package screencapture keeps a screen out of screenshots and recordings:
// for a card number, say, or a one-time code.
//
//	nimbus native plugin add screen-capture
//
//	screencapture.Prevent(ctx)
//	defer screencapture.Allow(ctx)
//
// Android blocks both. iOS can't block a screenshot: it covers the screen
// during a recording, and Watch tells you when a screenshot was taken.
package screencapture

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Prevent starts protecting every screen, until Allow.
func Prevent(ctx context.Context) error {
	_, err := plugin.Call[any](ctx, "screencapture", "set", map[string]bool{"prevent": true})
	return err
}

// Allow stops protecting them.
func Allow(ctx context.Context) error {
	_, err := plugin.Call[any](ctx, "screencapture", "set", map[string]bool{"prevent": false})
	return err
}

// Watch calls f each time a screenshot is taken of your app, until ctx ends.
// It blocks, so run it from a goroutine in an effect. Android needs version
// 14 or later, and fails with plugin.Unavailable before that.
//
//nimbus:param f called after each screenshot
//nimbus:returns why it stopped, which is ctx.Err() when ctx ends
func Watch(ctx context.Context, f func()) error {
	if _, err := plugin.CallJSON(ctx, "screencapture", "start", nil); err != nil {
		return err
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = plugin.CallJSON(c, "screencapture", "stop", nil)
	}()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	seen := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
		r, err := plugin.Call[struct {
			Count int `json:"count"`
		}](ctx, "screencapture", "read", nil)
		if err != nil {
			return err
		}
		for ; seen < r.Count; seen++ {
			f()
		}
	}
}
