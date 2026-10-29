// Package bridge is the Go side of the Nimbus Native C ABI (include/nn.h).
//
// An app registers its handlers in init() — before the host starts the
// runtime — and then talks to the host with Send and CallHost. Everything
// that crosses is a byte slice; the wire format gives the bytes meaning.
//
// On a device the bridge is the C ABI (bridge_native.go). In dev mode
// (`nimbus native dev`, build tag nimbusdev) the same API runs on the laptop
// over a socket to the device (bridge_dev.go).
package bridge

import (
	"errors"
	"fmt"
	"runtime/debug"
	"sync/atomic"
)

var (
	onStart   []func()
	onMessage func([]byte)
	onCall    func([]byte) []byte

	started atomic.Bool
	inbox   = make(chan []byte, 1024)

	stats counters
)

// ErrNotStarted is returned by Send before the host has started the runtime.
var ErrNotStarted = errors.New("bridge: host has not started the runtime")

// OnStart registers fn to run once the host has started the runtime.
// Register in init().
func OnStart(fn func()) { onStart = append(onStart, fn) }

// OnMessage sets the handler for asynchronous messages from the host.
// Messages arrive in order, one at a time, on a runtime goroutine.
func OnMessage(fn func(msg []byte)) { onMessage = fn }

// OnCall sets the handler for synchronous calls from the host. It runs on the
// host's calling thread — usually the UI thread — so keep it fast.
func OnCall(fn func(req []byte) []byte) { onCall = fn }

// HostError is a failed CallHost: Code is an NN_ERR_* value from nn.h.
type HostError struct{ Code int }

func (e *HostError) Error() string { return fmt.Sprintf("bridge: host call failed (code %d)", e.Code) }

// Result codes shared with nn.h.
const (
	codeOK        = 0
	codeState     = 2
	codePanic     = 3
	codeNoHandler = 4
)

// begin marks the runtime started and runs the start hooks.
func begin() {
	go dispatch()
	for _, fn := range onStart {
		go guard("start", fn)
	}
}

// dispatch delivers host messages to the app one at a time, in order.
func dispatch() {
	for msg := range inbox {
		if onMessage != nil {
			guard("message", func() { onMessage(msg) })
		}
	}
}

// handleCall runs the app's call handler, turning a panic into codePanic
// with the panic text and Go stack.
func handleCall(req []byte) (reply []byte, rc int) {
	if onCall == nil {
		return nil, codeNoHandler
	}
	stats.calls.Add(1)
	defer func() {
		if r := recover(); r != nil {
			stats.panics.Add(1)
			msg := fmt.Sprintf("panic: %v\n\n%s", r, debug.Stack())
			Logf("%s", msg)
			reply, rc = []byte(msg), codePanic
		}
	}()
	return onCall(req), codeOK
}

// guard runs fn, logging instead of crashing the host if it panics.
func guard(what string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			stats.panics.Add(1)
			Logf("panic in %s handler: %v\n\n%s", what, r, debug.Stack())
		}
	}()
	fn()
}
