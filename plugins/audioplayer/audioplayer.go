// Package audioplayer plays audio from a URL or from bytes, with pause, seek,
// volume, speed and looping. It needs no permission. For a spoken turn with
// the microphone, see the audio plugin.
//
//	nimbus native plugin add audio-player
//
//	p, err := audioplayer.Open(ctx, audioplayer.Source{URL: "https://example.com/show.mp3"})
//	defer p.Close(ctx)
//	p.Play(ctx)
//	go p.Watch(ctx, setStatus)
package audioplayer

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Source is what to play: a URL, or Data with its Type.
type Source struct {
	URL  string `json:"url,omitempty"`  // an https address
	Data []byte `json:"data,omitempty"` // or the file itself
	Type string `json:"type,omitempty"` // Data's MIME type, such as "audio/mp4"
}

// Status is where it's got to.
type Status struct {
	Position float64 `json:"position"` // seconds in
	Duration float64 `json:"duration"` // seconds long; 0 for a live stream
	Playing  bool    `json:"playing"`  // it's playing now
	Ended    bool    `json:"ended"`    // it reached the end
}

// Player is one sound. Close it when you're done.
type Player struct{ id string }

func (p *Player) call(ctx context.Context, method string, args map[string]any) error {
	if args == nil {
		args = map[string]any{}
	}
	args["id"] = p.id
	_, err := plugin.Call[any](ctx, "audioplayer", method, args)
	return err
}

// Open loads it, ready to play, and returns once it's ready.
//
//nimbus:param s what to play
//nimbus:returns the player
func Open(ctx context.Context, s Source) (*Player, error) {
	r, err := plugin.Call[struct {
		ID string `json:"id"`
	}](ctx, "audioplayer", "open", s)
	if err != nil {
		return nil, err
	}
	return &Player{r.ID}, nil
}

// Play starts it, or carries on after Pause. After the end, it starts again.
func (p *Player) Play(ctx context.Context) error { return p.call(ctx, "play", nil) }

// Pause stops it where it is.
func (p *Player) Pause(ctx context.Context) error { return p.call(ctx, "pause", nil) }

// Seek jumps to a place.
//
//nimbus:param seconds how far in
func (p *Player) Seek(ctx context.Context, seconds float64) error {
	return p.call(ctx, "seek", map[string]any{"seconds": seconds})
}

// SetVolume sets how loud it is, within the phone's own volume.
//
//nimbus:param volume 0 to 1
func (p *Player) SetVolume(ctx context.Context, volume float64) error {
	return p.call(ctx, "volume", map[string]any{"volume": volume})
}

// SetRate sets its speed, keeping the pitch.
//
//nimbus:param rate 1 is normal; try 0.5 to 2
func (p *Player) SetRate(ctx context.Context, rate float64) error {
	return p.call(ctx, "rate", map[string]any{"rate": rate})
}

// SetLoop makes it start again at the end.
//
//nimbus:param loop whether to
func (p *Player) SetLoop(ctx context.Context, loop bool) error {
	return p.call(ctx, "loop", map[string]any{"loop": loop})
}

// Status says where it's got to.
func (p *Player) Status(ctx context.Context) (Status, error) {
	return plugin.Call[Status](ctx, "audioplayer", "status", map[string]string{"id": p.id})
}

// Watch calls f about four times a second until ctx ends. It blocks, so run
// it from a goroutine in an effect.
//
//nimbus:param f called with where it's got to, on the goroutine that called Watch
//nimbus:returns why it stopped, which is ctx.Err() when ctx ends
func (p *Player) Watch(ctx context.Context, f func(Status)) error {
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
		s, err := p.Status(ctx)
		if err != nil {
			return err
		}
		f(s)
	}
}

// Close stops it and lets go of it.
func (p *Player) Close(ctx context.Context) error { return p.call(ctx, "close", nil) }
