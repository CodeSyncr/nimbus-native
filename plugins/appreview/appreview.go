// Package appreview asks for a rating with the system's own prompt.
//
//	nimbus native plugin add app-review
//
//	appreview.Request(ctx)
//
// The system decides whether it really shows, and limits how often, so call
// it after something good happens, never from a button. In development
// builds it may not show at all.
package appreview

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Request may show the prompt.
func Request(ctx context.Context) error {
	_, err := plugin.CallJSON(ctx, "appreview", "request", nil)
	return err
}
