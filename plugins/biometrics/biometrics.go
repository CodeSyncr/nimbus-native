// Package biometrics confirms it's really them, with Face ID, Touch ID or a
// fingerprint.
//
//	nimbus native plugin add biometrics
//
//	if biometrics.Available(ctx) != biometrics.None {
//		ok, err := biometrics.Confirm(ctx, "Show today's figures")
//	}
package biometrics

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Kind is what the phone can do.
type Kind string

const (
	None        Kind = ""
	Face        Kind = "face"
	Fingerprint Kind = "fingerprint"
)

// Available says which is set up on this phone. Android can't always tell
// which: it says Fingerprint for any strong biometric.
func Available(ctx context.Context) Kind {
	r, err := plugin.Call[struct {
		Kind Kind `json:"kind"`
	}](ctx, "biometrics", "available", nil)
	if err != nil {
		return None
	}
	return r.Kind
}

// Confirm asks them to prove it's them, saying reason. It's false if they
// fail or back out.
func Confirm(ctx context.Context, reason string) (bool, error) {
	r, err := plugin.Call[struct {
		OK bool `json:"ok"`
	}](ctx, "biometrics", "confirm", map[string]string{"reason": reason})
	return r.OK, err
}
