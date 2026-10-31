// Package contacts reads the phone's contacts.
//
//	nimbus native plugin add contacts
//
//	c, err := contacts.Pick(ctx) // no permission: they choose one
//
//	ok, err := contacts.Request(ctx)
//	all, err := contacts.List(ctx)
package contacts

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Contact is a person.
type Contact struct {
	Name   string   `json:"name"`
	Phones []string `json:"phones"`
	Emails []string `json:"emails"`
}

// Pick opens the system's contact picker. It needs no permission, because
// the app only gets the one chosen. It fails with plugin.Cancelled if they
// back out.
func Pick(ctx context.Context) (Contact, error) {
	return plugin.Call[Contact](ctx, "contacts", "pick", nil)
}

// Request asks to read all the contacts, once.
func Request(ctx context.Context) (bool, error) {
	r, err := plugin.Call[struct {
		Granted bool `json:"granted"`
	}](ctx, "contacts", "request", nil)
	return r.Granted, err
}

// List returns every contact. Call Request first.
func List(ctx context.Context) ([]Contact, error) {
	r, err := plugin.Call[struct {
		Contacts []Contact `json:"contacts"`
	}](ctx, "contacts", "list", nil)
	return r.Contacts, err
}
