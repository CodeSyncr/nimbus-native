// Package audio is the phone's microphone and speaker: one spoken turn at a
// time, for talking to an assistant. Listen opens the microphone with the
// phone's noise suppression and echo cancelling, and waits for someone
// close to it to say something and stop; Play plays a clip back. Both
// report how loud it is, 0…1, for something to move with the voice.
//
//	if ok, err := audio.RequestMicrophone(ctx); err != nil || !ok {
//		return // not allowed
//	}
//	l := &audio.Listener{OnLevel: setLevel}
//	wav, err := l.Listen(ctx) // a WAV, 16 kHz mono; nil if nothing was said
//	…
//	audio.Play(ctx, reply, setLevel)
//	audio.Release()           // other apps' audio comes back
//
// Add it to a project with `nimbus native plugin add audio`, which also
// asks for the microphone in the app's Info.plist and manifest. There is no
// audio on the web yet: the calls fail with plugin.Unavailable there.
package audio

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"sync/atomic"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

const name = "audio"

const (
	frame        = Rate / 50             // the detector's 20 ms
	micPoll      = 50 * time.Millisecond // the host keeps what arrives between reads
	speakerPoll  = time.Second / 30
	stopDeadline = 3 * time.Second
)

// RequestMicrophone asks for the microphone, once: later calls answer at
// once with what the person chose.
func RequestMicrophone(ctx context.Context) (bool, error) {
	r, err := plugin.Call[struct {
		Granted bool `json:"granted"`
	}](ctx, name, "requestMicrophone", nil)
	return r.Granted, err
}

// Listener listens for one spoken turn at a time. Its callbacks run on
// Listen's goroutine.
type Listener struct {
	OnLevel  func(level float32) // how loud, 0…1, a few dozen times a second
	OnSpeech func()              // someone started talking
	finish   atomic.Bool
}

// Listen opens the microphone, waits for one turn and closes it again. It
// returns the turn as a WAV, or nil if nothing was said; a cancelled ctx
// stops it with ctx's error.
func (l *Listener) Listen(ctx context.Context) ([]byte, error) {
	l.finish.Store(false)
	if _, err := plugin.CallJSON(ctx, name, "micStart", nil); err != nil {
		return nil, err
	}
	defer call("micStop", nil)

	d := NewTurnDetector()
	var rest []int16
	tick := time.NewTicker(micPoll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-tick.C:
		}
		r, err := plugin.Call[struct {
			PCM string `json:"pcm"`
		}](ctx, name, "micRead", nil)
		if err != nil {
			return nil, err
		}
		pcm, err := base64.StdEncoding.DecodeString(r.PCM)
		if err != nil {
			return nil, err
		}
		for i := 0; i+1 < len(pcm); i += 2 {
			rest = append(rest, int16(binary.LittleEndian.Uint16(pcm[i:])))
		}
		for len(rest) >= frame {
			turn, level, clip := d.Feed(rest[:frame])
			rest = rest[frame:]
			if l.OnLevel != nil {
				l.OnLevel(level)
			}
			switch turn {
			case SpeechStarted:
				if l.OnSpeech != nil {
					l.OnSpeech()
				}
			case TurnEnded:
				return clip, nil
			case NothingSaid:
				return nil, nil
			}
		}
		if l.finish.Load() {
			return d.Finish(), nil
		}
	}
}

// Finish ends the turn now, with what has been said so far. Safe from any
// goroutine.
func (l *Listener) Finish() { l.finish.Store(true) }

// Play plays a clip (WAV, MP3 or M4A) through the speaker and returns when
// it has finished. A cancelled ctx stops it, returning ctx's error. onLevel
// (may be nil) gets how loud it is, 0…1.
func Play(ctx context.Context, clip []byte, onLevel func(level float32)) error {
	args := map[string]string{"clip": base64.StdEncoding.EncodeToString(clip)}
	if _, err := plugin.CallJSON(ctx, name, "play", args); err != nil {
		return err
	}
	tick := time.NewTicker(speakerPoll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			call("stop", nil)
			return ctx.Err()
		case <-tick.C:
		}
		r, err := plugin.Call[struct {
			Playing bool    `json:"playing"`
			Level   float32 `json:"level"`
		}](ctx, name, "playing", nil)
		if err != nil {
			if ctx.Err() != nil {
				continue // answered by the select above
			}
			return err
		}
		if !r.Playing {
			return nil
		}
		if onLevel != nil {
			onLevel(r.Level)
		}
	}
}

// Release stops everything and lets go of the microphone and speaker, so
// other apps' audio comes back. Call it when the conversation ends.
func Release() { call("release", nil) }

// call is for stopping: it runs when the caller's ctx may be over.
func call(method string, args any) {
	ctx, cancel := context.WithTimeout(context.Background(), stopDeadline)
	defer cancel()
	_, _ = plugin.CallJSON(ctx, name, method, args)
}
