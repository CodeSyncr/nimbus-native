// Package nfc reads NFC tags.
//
//	nimbus native plugin add nfc
//
//	if nfc.Available(ctx) {
//		tag, err := nfc.Read(ctx, "Hold your card near the phone")
//	}
//
// iPhones also need the NFC capability turned on for the app in your Apple
// developer account. Written against the platform APIs, but not yet run:
// simulators and emulators have no NFC.
package nfc

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Tag is what was read.
type Tag struct {
	ID      string   `json:"id"`      // hex
	Records []string `json:"records"` // NDEF text and links, when it has any
}

// Available reports whether this phone can read tags.
func Available(ctx context.Context) bool {
	r, err := plugin.Call[struct {
		Available bool `json:"available"`
	}](ctx, "nfc", "available", nil)
	return err == nil && r.Available
}

// Read waits for a tag. prompt shows on iPhone's scanning sheet. It fails
// with plugin.Cancelled if they back out; a cancelled ctx stops it on Android.
//
//nimbus:param prompt what to do, shown while it waits on iOS
func Read(ctx context.Context, prompt string) (Tag, error) {
	return plugin.Call[Tag](ctx, "nfc", "read", map[string]string{"prompt": prompt})
}
