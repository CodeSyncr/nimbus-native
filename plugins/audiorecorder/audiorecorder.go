// Package audiorecorder records from the microphone to an AAC file, for a
// voice memo, say. Pause and Resume work, and the file comes back as bytes
// from Stop. For one spoken turn that ends by itself, see the audio plugin.
//
//	nimbus native plugin add audio-recorder
//
//	if ok, _ := audiorecorder.Request(ctx); ok {
//		audiorecorder.Start(ctx)
//	}
//	r, err := audiorecorder.Stop(ctx)
package audiorecorder

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Recording is a finished recording.
type Recording struct {
	Data    []byte  `json:"data"`    // the file
	Type    string  `json:"type"`    // its MIME type, "audio/mp4"
	Seconds float64 `json:"seconds"` // how long it runs
}

// Status is where it's got to.
type Status struct {
	Recording bool    `json:"recording"` // it's recording now, not paused
	Seconds   float64 `json:"seconds"`   // how much so far
	Level     float64 `json:"level"`     // how loud it is now, 0 to 1
}

// Request asks to use the microphone, once.
func Request(ctx context.Context) (bool, error) {
	r, err := plugin.Call[struct {
		Granted bool `json:"granted"`
	}](ctx, "audiorecorder", "request", nil)
	return r.Granted, err
}

// Start begins recording, one at a time. Without permission it fails with
// plugin.Denied.
func Start(ctx context.Context) error {
	_, err := plugin.Call[any](ctx, "audiorecorder", "start", nil)
	return err
}

// Pause stops listening, keeping what's been recorded.
func Pause(ctx context.Context) error {
	_, err := plugin.Call[any](ctx, "audiorecorder", "pause", nil)
	return err
}

// Resume carries on after Pause.
func Resume(ctx context.Context) error {
	_, err := plugin.Call[any](ctx, "audiorecorder", "resume", nil)
	return err
}

// Stop ends it and gives back the file.
//
//nimbus:returns the recording
func Stop(ctx context.Context) (Recording, error) {
	return plugin.Call[Recording](ctx, "audiorecorder", "stop", nil)
}

// Cancel ends it and throws it away.
func Cancel(ctx context.Context) error {
	_, err := plugin.Call[any](ctx, "audiorecorder", "cancel", nil)
	return err
}

// Current says where it's got to.
func Current(ctx context.Context) (Status, error) {
	return plugin.Call[Status](ctx, "audiorecorder", "status", nil)
}

// Watch calls f about ten times a second until ctx ends. It blocks, so run it
// from a goroutine in an effect.
//
//nimbus:param f called with where it's got to, on the goroutine that called Watch
//nimbus:returns why it stopped, which is ctx.Err() when ctx ends
func Watch(ctx context.Context, f func(Status)) error {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
		s, err := Current(ctx)
		if err != nil {
			return err
		}
		f(s)
	}
}
