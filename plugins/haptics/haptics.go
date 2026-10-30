// Package haptics plays the phone's taps and buzzes: the system's own
// feedback (Impact, Notify, Selection) and patterns of your own (Play) —
// Core Haptics on iOS, the vibrator's effects on Android. Every call returns
// at once (the feedback plays on its own), so they are safe anywhere,
// handlers included. On the web, and on phones without a haptic engine,
// they do nothing.
//
//	haptics.Impact(haptics.Medium, 1)          // a tap
//	haptics.Notify(haptics.Success)            // something worked
//	haptics.Play([]haptics.Event{              // a thud, then a hum
//		{Intensity: 1, Sharpness: 0.35},
//		{At: 50 * time.Millisecond, Duration: time.Second, Intensity: 0.9, Sharpness: 0.3},
//	})
package haptics

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

const name = "haptics"

// Style is how an Impact feels.
type Style string

const (
	Light  Style = "light"
	Medium Style = "medium"
	Heavy  Style = "heavy"
	Soft   Style = "soft"
	Rigid  Style = "rigid"
)

// Kind is what a Notify says.
type Kind string

const (
	Success Kind = "success"
	Warning Kind = "warning"
	Error   Kind = "error"
)

// Event is one moment of a pattern: a tap (Duration 0) or a buzz that
// lasts Duration, At its offset from the start. Intensity and Sharpness are
// 0…1: how strong, and how crisp rather than round.
type Event struct {
	At, Duration         time.Duration
	Intensity, Sharpness float32
}

// Curve changes a pattern's intensity or sharpness over time while it
// plays (a hum that swells). Intensity values multiply (0…1); sharpness
// values add (-1…1).
type Curve struct {
	Sharpness bool // false: the curve is for intensity
	Points    []Point
}

// Point is a Curve's value At a moment.
type Point struct {
	At    time.Duration
	Value float32
}

// Impact is a single tap of a style, at intensity (0…1).
func Impact(style Style, intensity float32) {
	send("impact", map[string]any{"style": style, "intensity": intensity})
}

// Notify is the system's feedback for a success, warning or error.
func Notify(kind Kind) { send("notify", map[string]any{"kind": kind}) }

// Selection is the light tick of a choice changing.
func Selection() { send("selection", nil) }

// Play plays a pattern. Android plays its taps and buzzes with their
// strength; it can't vary sharpness, and its curves are sampled.
func Play(events []Event, curves ...Curve) {
	type ev struct {
		T, D, I, S float64
	}
	type cv struct {
		Sharp  bool        `json:"sharp"`
		Points [][2]float64 `json:"points"`
	}
	var es []ev
	for _, e := range events {
		es = append(es, ev{e.At.Seconds(), e.Duration.Seconds(), float64(e.Intensity), float64(e.Sharpness)})
	}
	var cs []cv
	for _, c := range curves {
		v := cv{Sharp: c.Sharpness}
		for _, p := range c.Points {
			v.Points = append(v.Points, [2]float64{p.At.Seconds(), float64(p.Value)})
		}
		cs = append(cs, v)
	}
	send("play", map[string]any{"events": es, "curves": cs})
}

// send calls the host off the caller's goroutine: plugin calls wait for an
// answer, and feedback has nothing to wait for.
func send(method string, args any) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = plugin.CallJSON(ctx, name, method, args)
	}()
}
