// Package push receives notifications from your server: APNs on iOS, and
// Firebase Cloud Messaging on Android.
//
//	nimbus native plugin add push
//
// Android needs your Firebase project's google-services.json, at
// native/android/app/google-services.json. On iOS, turn on Push Notifications
// for the app in your Apple developer account.
//
//	ok, err := push.Request(ctx)
//	t, err := push.GetToken(ctx)
//	// send t.Value and t.Service to your server
//	go push.Watch(ctx, func(m push.Message) { openOrder(m.Data["order"]) })
package push

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Token is where your server sends to reach this phone.
type Token struct {
	Value   string `json:"value"`
	Service string `json:"service"` // "apns" or "fcm"
}

// Message is a notification that arrived while the app was open, or one
// that was tapped.
type Message struct {
	Title  string            `json:"title"`
	Body   string            `json:"body"`
	Data   map[string]string `json:"data"`
	Tapped bool              `json:"tapped"`
}

// Request asks to show notifications, once.
func Request(ctx context.Context) (bool, error) {
	r, err := plugin.Call[struct {
		Granted bool `json:"granted"`
	}](ctx, "push", "request", nil)
	return r.Granted, err
}

// GetToken registers with APNs or Firebase and returns this phone's token.
// It can change: send it to your server each time the app starts.
func GetToken(ctx context.Context) (Token, error) {
	return plugin.Call[Token](ctx, "push", "token", nil)
}

// Messages returns what has arrived since the last call.
func Messages(ctx context.Context) ([]Message, error) {
	r, err := plugin.Call[struct {
		Messages []Message `json:"messages"`
	}](ctx, "push", "messages", nil)
	return r.Messages, err
}

// Watch calls f with each message until ctx ends, including the tap that
// opened the app. It blocks, so run it from a goroutine in an effect.
func Watch(ctx context.Context, f func(Message)) error {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		ms, err := Messages(ctx)
		if err != nil {
			return err
		}
		for _, m := range ms {
			f(m)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}
