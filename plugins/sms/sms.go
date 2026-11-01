// Package sms opens the system's screen for writing a text message, filled
// in, for them to check and send. Nothing is sent without them.
//
//	nimbus native plugin add sms
//
//	status, err := sms.Compose(ctx, sms.Options{To: []string{"+912012345678"}, Body: "Table for two at 8?"})
package sms

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Options is what to fill in.
type Options struct {
	To   []string `json:"to,omitempty"`   // the phone numbers
	Body string   `json:"body,omitempty"` // the message
}

// Status is what they did.
type Status string

const (
	Sent      Status = "sent"      // it was sent
	Cancelled Status = "cancelled" // they backed out
	// Unknown is every answer on Android, which can't tell.
	Unknown Status = "unknown"
)

// Available reports whether this phone can send texts.
func Available(ctx context.Context) bool {
	r, err := plugin.Call[struct {
		Available bool `json:"available"`
	}](ctx, "sms", "available", nil)
	return err == nil && r.Available
}

// Compose opens the screen and returns when it closes. With no messages app
// it fails with plugin.Unavailable.
//
//nimbus:param o what to fill in
//nimbus:returns what they did
func Compose(ctx context.Context, o Options) (Status, error) {
	r, err := plugin.Call[struct {
		Status Status `json:"status"`
	}](ctx, "sms", "compose", o)
	return r.Status, err
}
