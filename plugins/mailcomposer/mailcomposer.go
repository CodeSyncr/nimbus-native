// Package mailcomposer opens the system's screen for writing an email, filled
// in, for them to check and send. Nothing is sent without them.
//
//	nimbus native plugin add mail-composer
//
//	status, err := mailcomposer.Compose(ctx, mailcomposer.Options{
//		To: []string{"hello@example.com"}, Subject: "Order 1042", Body: "Hello",
//	})
package mailcomposer

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Options is what to fill in. They can change all of it.
type Options struct {
	To      []string `json:"to,omitempty"`      // who it goes to
	CC      []string `json:"cc,omitempty"`      // who's copied
	BCC     []string `json:"bcc,omitempty"`     // who's copied, unseen
	Subject string   `json:"subject,omitempty"` // its subject
	Body    string   `json:"body,omitempty"`    // its text
	// HTML treats Body as HTML. It's iOS only: Android sends it as plain text.
	HTML  bool   `json:"html,omitempty"`
	Files []File `json:"files,omitempty"` // attachments
}

// File is an attachment, as bytes.
type File struct {
	Name string `json:"name"` // its file name
	Type string `json:"type"` // a MIME type, such as "application/pdf"
	Data []byte `json:"data"` // its contents
}

// Status is what they did.
type Status string

const (
	Sent      Status = "sent"      // it went to the outbox
	Saved     Status = "saved"     // kept as a draft
	Cancelled Status = "cancelled" // they backed out
	// Unknown is every answer on Android, which can't tell.
	Unknown Status = "unknown"
)

// Available reports whether this phone can write an email. On iOS that needs
// an account in the Mail app. Android only finds out when Compose runs.
func Available(ctx context.Context) bool {
	r, err := plugin.Call[struct {
		Available bool `json:"available"`
	}](ctx, "mailcomposer", "available", nil)
	return err == nil && r.Available
}

// Compose opens the screen and returns when it closes. With no mail app it
// fails with plugin.Unavailable.
//
//nimbus:param o what to fill in
//nimbus:returns what they did
func Compose(ctx context.Context, o Options) (Status, error) {
	r, err := plugin.Call[struct {
		Status Status `json:"status"`
	}](ctx, "mailcomposer", "compose", o)
	return r.Status, err
}
