// Package speech reads text aloud with the phone's own voices.
//
//	nimbus native plugin add speech
//
//	speech.Say(ctx, "Order ready for table seven", speech.Options{Language: "en-IN"})
package speech

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Options shapes the voice.
type Options struct {
	Language string  `json:"language"` // BCP 47, such as "hi-IN"; empty for the phone's own
	Rate     float32 `json:"rate"`     // 1 is normal; 0 means 1
}

// Say reads text and returns when it's done. A cancelled ctx stops it.
func Say(ctx context.Context, text string, o Options) error {
	args := map[string]any{"text": text, "language": o.Language, "rate": o.Rate}
	if _, err := plugin.CallJSON(ctx, "speech", "say", args); err != nil {
		return err
	}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			stop, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, _ = plugin.CallJSON(stop, "speech", "stop", nil)
			return ctx.Err()
		case <-tick.C:
		}
		r, err := plugin.Call[struct {
			Speaking bool `json:"speaking"`
		}](ctx, "speech", "speaking", nil)
		if err != nil || !r.Speaking {
			return err
		}
	}
}
