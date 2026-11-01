// Package trackingconsent asks before the app tracks them across other apps
// and websites, as the App Store requires.
//
//	nimbus native plugin add tracking-consent
//
//	if s, _ := trackingconsent.Request(ctx); s.Allowed() {
//		ads.Personalize()
//	}
package trackingconsent

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Status is what they've said.
type Status string

const (
	// NotAsked means Request will show the prompt.
	NotAsked Status = "not-asked"
	Granted  Status = "granted"
	Denied   Status = "denied"
	// Restricted means the phone doesn't let them choose, such as on a
	// child's account. Treat it as Denied.
	Restricted Status = "restricted"
	// NotNeeded means this platform has no such prompt: Android.
	NotNeeded Status = "not-needed"
)

// Allowed reports whether tracking may go ahead.
func (s Status) Allowed() bool { return s == Granted || s == NotNeeded }

func call(ctx context.Context, method string) (Status, error) {
	r, err := plugin.Call[struct {
		Status Status `json:"status"`
	}](ctx, "trackingconsent", method, nil)
	return r.Status, err
}

// Current says what they've said, without asking.
func Current(ctx context.Context) (Status, error) { return call(ctx, "status") }

// Request shows the prompt, once ever: after that it answers with what they
// said. Call it from a screen that's on show, or iOS skips the prompt.
func Request(ctx context.Context) (Status, error) { return call(ctx, "request") }
