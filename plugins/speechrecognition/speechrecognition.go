// Package speechrecognition turns speech into text, with the phone's own
// recognizer. For a whole assistant turn sent to your server, use audio.
//
//	nimbus native plugin add speech-recognition
//
//	ok, err := speechrecognition.Request(ctx)
//	text, err := speechrecognition.Listen(ctx, speechrecognition.Options{
//		Language: "hi-IN", OnPartial: setDraft})
package speechrecognition

import (
	"context"
	"encoding/json"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Options shapes a listen.
type Options struct {
	Language  string            // BCP 47, such as "hi-IN"; empty for the phone's own
	OnPartial func(text string) // what it has so far, as they speak
}

// Request asks for the microphone and speech recognition, once.
func Request(ctx context.Context) (bool, error) {
	r, err := plugin.Call[struct {
		Granted bool `json:"granted"`
	}](ctx, "speechrecognition", "request", nil)
	return r.Granted, err
}

// Listen hears one phrase and returns its text. It ends when they stop
// speaking; a cancelled ctx stops it. Nothing heard is "", not an error.
//
//nimbus:param o the language, and a callback for words so far
func Listen(ctx context.Context, o Options) (string, error) {
	if _, err := plugin.CallJSON(ctx, "speechrecognition", "start", map[string]string{"language": o.Language}); err != nil {
		return "", err
	}
	tick := time.NewTicker(150 * time.Millisecond)
	defer tick.Stop()
	last := ""
	for {
		select {
		case <-ctx.Done():
			stop, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, _ = plugin.CallJSON(stop, "speechrecognition", "stop", nil)
			return last, ctx.Err()
		case <-tick.C:
		}
		raw, err := plugin.CallJSON(ctx, "speechrecognition", "read", nil)
		if err != nil {
			return last, err
		}
		var r struct {
			Text string `json:"text"`
			Done bool   `json:"done"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return last, err
		}
		if r.Text != last {
			last = r.Text
			if o.OnPartial != nil && !r.Done {
				o.OnPartial(last)
			}
		}
		if r.Done {
			return last, nil
		}
	}
}
