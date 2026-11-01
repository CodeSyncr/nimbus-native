// Package contacts reads and changes the phone's contacts.
//
//	nimbus native plugin add contacts
//
//	c, err := contacts.Pick(ctx)                // no permission: they choose one
//	saved, err := contacts.Add(ctx, cafe)       // no permission: they see it first
//
//	ok, err := contacts.Request(ctx)
//	all, err := contacts.List(ctx)
//	id, err := contacts.Create(ctx, cafe)
package contacts

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Contact is a person.
type Contact struct {
	ID     string   `json:"id"`     // for Update and Delete; set by List
	Name   string   `json:"name"`   // their full name
	Phones []string `json:"phones"` // their phone numbers
	Emails []string `json:"emails"` // their email addresses
}

// Pick opens the system's contact picker. It needs no permission, because
// the app only gets the one chosen. It fails with plugin.Cancelled if they
// back out.
func Pick(ctx context.Context) (Contact, error) {
	return plugin.Call[Contact](ctx, "contacts", "pick", nil)
}

// Request asks to read and change the contacts, once.
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

// Add opens the system's new contact screen, filled in. It needs no
// permission, because they see it and save it themselves. It's true if they
// saved it; Android can't always tell, so there it's true once the screen
// closes.
//
//nimbus:param c what to fill in
//nimbus:returns whether they saved it
func Add(ctx context.Context, c Contact) (bool, error) {
	r, err := plugin.Call[struct {
		Saved bool `json:"saved"`
	}](ctx, "contacts", "add", c)
	return r.Saved, err
}

// Create saves a new contact with no screen. Call Request first, and tell
// them: it's their address book.
//
//nimbus:param c the person
//nimbus:returns its ID
func Create(ctx context.Context, c Contact) (string, error) {
	r, err := plugin.Call[struct {
		ID string `json:"id"`
	}](ctx, "contacts", "create", c)
	return r.ID, err
}

// Update replaces a contact's name, phones and emails. Anything else about
// them is kept.
//
//nimbus:param c the person, with their ID
func Update(ctx context.Context, c Contact) error {
	_, err := plugin.Call[any](ctx, "contacts", "update", c)
	return err
}

// Delete removes a contact.
//
//nimbus:param id the Contact's ID
func Delete(ctx context.Context, id string) error {
	_, err := plugin.Call[any](ctx, "contacts", "delete", map[string]string{"id": id})
	return err
}
