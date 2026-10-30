// Package localnotifications shows reminders the app schedules on the phone
// itself, with no server. For messages from a server, use push.
//
//	nimbus native plugin add local-notifications
//
//	ok, err := localnotifications.Request(ctx)
//	localnotifications.Schedule(ctx, localnotifications.Notification{
//		ID: "table-7", Title: "Table 7 is waiting", Body: "Order's ready", After: 10 * time.Minute})
package localnotifications

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Notification is a reminder.
type Notification struct {
	ID    string // a new one with the same ID replaces it
	Title string
	Body  string
	After time.Duration // how long from now; at least a second
}

// Request asks to show notifications, once.
func Request(ctx context.Context) (bool, error) {
	r, err := plugin.Call[struct {
		Granted bool `json:"granted"`
	}](ctx, "localnotifications", "request", nil)
	return r.Granted, err
}

// Schedule sets a reminder.
func Schedule(ctx context.Context, n Notification) error {
	_, err := plugin.CallJSON(ctx, "localnotifications", "schedule", map[string]any{
		"id": n.ID, "title": n.Title, "body": n.Body, "seconds": max(n.After.Seconds(), 1)})
	return err
}

// Cancel takes one back.
func Cancel(ctx context.Context, id string) error {
	_, err := plugin.CallJSON(ctx, "localnotifications", "cancel", map[string]string{"id": id})
	return err
}
