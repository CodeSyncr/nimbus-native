//go:build !nimbusdev && !cgo

package bridge

// Without cgo there is no native host to start the runtime: this is a
// server build (a Nimbus server serving the app's pages, built with
// CGO_ENABLED=0). The web host renders through ui.Session and never comes
// here; anything that does gets ErrNotStarted, and native plugins answer
// unavailable.

// Main is for the app's main function; there is no host to run here.
func Main() {}

// Send has no host to deliver to.
func Send(msg []byte) error { return ErrNotStarted }

// CallHost has no host to ask.
func CallHost(req []byte) ([]byte, error) { return nil, ErrNotStarted }
