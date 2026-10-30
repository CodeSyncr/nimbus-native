// Package network says whether the phone is online, and how.
//
//	nimbus native plugin add network
//
//	s, err := network.Get(ctx)
//	network.Watch(ctx, func(s network.Status) { setOnline(s.Connected) })
package network

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Status is the phone's connection.
type Status struct {
	Connected bool   `json:"connected"`
	Type      string `json:"type"`      // "wifi", "cellular", "wired", "other" or "none"
	Expensive bool   `json:"expensive"` // metered, such as mobile data
}

// Get returns the connection now.
func Get(ctx context.Context) (Status, error) {
	return plugin.Call[Status](ctx, "network", "status", nil)
}

// Watch calls f with the connection now and whenever it changes, until ctx
// ends. It blocks, so run it from a goroutine in an effect.
func Watch(ctx context.Context, f func(Status)) error {
	var last Status
	first := true
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		s, err := Get(ctx)
		if err != nil {
			return err
		}
		if first || s != last {
			first, last = false, s
			f(s)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}
