// Package geolocation says where the phone is.
//
//	nimbus native plugin add geolocation
//
//	ok, err := geolocation.Request(ctx)
//	at, err := geolocation.Current(ctx)
//	go geolocation.Watch(ctx, func(p geolocation.Position) { setHere(p) })
package geolocation

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Position is a place on Earth.
type Position struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Accuracy  float64 `json:"accuracy"` // metres
	Altitude  float64 `json:"altitude"` // metres
	Speed     float64 `json:"speed"`    // metres a second; below 0 when unknown
	Heading   float64 `json:"heading"`  // degrees from north; below 0 when unknown
}

// Request asks to use the location while the app is open, once.
func Request(ctx context.Context) (bool, error) {
	r, err := plugin.Call[struct {
		Granted bool `json:"granted"`
	}](ctx, "geolocation", "request", nil)
	return r.Granted, err
}

// Current returns a fresh position. Call Request first: without permission
// it fails with plugin.Denied.
func Current(ctx context.Context) (Position, error) {
	return plugin.Call[Position](ctx, "geolocation", "current", nil)
}

// Watch calls f as the phone moves, until ctx ends. It blocks, so run it from
// a goroutine in an effect.
func Watch(ctx context.Context, f func(Position)) error {
	if _, err := plugin.CallJSON(ctx, "geolocation", "start", nil); err != nil {
		return err
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = plugin.CallJSON(c, "geolocation", "stop", nil)
	}()
	var last Position
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
		r, err := plugin.Call[struct {
			Position
			Known bool `json:"known"`
		}](ctx, "geolocation", "latest", nil)
		if err != nil {
			return err
		}
		if r.Known && r.Position != last {
			last = r.Position
			f(last)
		}
	}
}
