// Package bridge is the Go side of the Nimbus Native C ABI (include/nn.h).
//
// An app registers its handlers in init() — before the host calls nn_start —
// and then talks to the host with Send. Everything that crosses is a byte
// slice; the wire format gives the bytes meaning.
package bridge

/*
#cgo CFLAGS: -I${SRCDIR}/include
#define NN_NO_PROTOTYPES
#include <stdlib.h>
#include "nn.h"

static void nn_wake(const nn_host *h) {
	h->wake(h->ctx);
}
*/
import "C"

import (
	"encoding/binary"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/CodeSyncr/nimbus-native/wire"
)

// ABIVersion is the nn.h version this runtime speaks.
const ABIVersion = C.NN_ABI_VERSION

// An outbox that grew past this is dropped after a drain instead of kept for
// reuse, so one burst doesn't pin its memory forever.
const keepOutbox = 1 << 20

var (
	onStart   []func()
	onMessage func([]byte)
	onCall    func([]byte) []byte

	host    *C.nn_host // C-allocated copy of what the host gave nn_start
	started atomic.Bool
	inbox   = make(chan []byte, 1024)

	outMu  sync.Mutex
	outbox []byte // frames waiting for nn_drain: [u32 LE length][bytes]...

	stats counters
)

// ErrNotStarted is returned by Send before the host has called nn_start.
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

// Send queues msg for the host. It is safe from any goroutine and never
// blocks on the host: messages are batched and the host drains them on its
// UI thread, in the order Send returned. msg is copied.
func Send(msg []byte) error {
	if !started.Load() {
		return ErrNotStarted
	}
	outMu.Lock()
	wasEmpty := len(outbox) == 0
	outbox = binary.LittleEndian.AppendUint32(outbox, uint32(len(msg)))
	outbox = append(outbox, msg...)
	outMu.Unlock()
	stats.sent.Add(1)

	// Only the empty → non-empty transition wakes the host; everything sent
	// before it drains rides along in the same batch.
	if wasEmpty {
		stats.wakes.Add(1)
		C.nn_wake(host)
	}
	return nil
}

//export nn_start
func nn_start(h *C.nn_host) C.int32_t {
	if h == nil || h.wake == nil {
		return C.NN_ERR_ARG
	}
	if h.abi != C.NN_ABI_VERSION {
		return C.NN_ERR_ABI
	}
	if uint64(h.schema) != wire.Combined() {
		Logf("schema mismatch: host %#x, runtime %#x — regenerate with nimbus-native gen", uint64(h.schema), wire.Combined())
		return C.NN_ERR_SCHEMA
	}
	if !started.CompareAndSwap(false, true) {
		return C.NN_ERR_STATE
	}
	host = (*C.nn_host)(C.malloc(C.size_t(unsafe.Sizeof(*h))))
	*host = *h

	go dispatch()
	for _, fn := range onStart {
		go guard("start", fn)
	}
	return C.NN_OK
}

//export nn_send
func nn_send(data *C.uint8_t, n C.int32_t) C.int32_t {
	if !started.Load() {
		return C.NN_ERR_STATE
	}
	stats.received.Add(1)
	inbox <- copyIn(data, n)
	return C.NN_OK
}

//export nn_drain
func nn_drain(out **C.uint8_t, outLen *C.int32_t) C.int32_t {
	if out == nil || outLen == nil {
		return C.NN_ERR_ARG
	}
	if !started.Load() {
		return C.NN_ERR_STATE
	}
	outMu.Lock()
	if len(outbox) > 0 {
		stats.drains.Add(1)
	}
	copyOut(outbox, out, outLen) // one memcpy into C memory
	if cap(outbox) > keepOutbox {
		outbox = nil
	} else {
		outbox = outbox[:0]
	}
	outMu.Unlock()
	return C.NN_OK
}

//export nn_call
func nn_call(data *C.uint8_t, n C.int32_t, out **C.uint8_t, outLen *C.int32_t) (rc C.int32_t) {
	if !started.Load() {
		return C.NN_ERR_STATE
	}
	if onCall == nil {
		return C.NN_ERR_NO_HANDLER
	}
	stats.calls.Add(1)
	defer func() {
		if r := recover(); r != nil {
			stats.panics.Add(1)
			msg := fmt.Sprintf("panic: %v\n\n%s", r, debug.Stack())
			Logf("%s", msg)
			copyOut([]byte(msg), out, outLen)
			rc = C.NN_ERR_PANIC
		}
	}()
	copyOut(onCall(copyIn(data, n)), out, outLen)
	return C.NN_OK
}

//export nn_free
func nn_free(p unsafe.Pointer) { C.free(p) }

// dispatch delivers host messages to the app one at a time, in order.
func dispatch() {
	for msg := range inbox {
		if onMessage != nil {
			guard("message", func() { onMessage(msg) })
		}
	}
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

func copyIn(data *C.uint8_t, n C.int32_t) []byte {
	if data == nil || n <= 0 {
		return []byte{}
	}
	return C.GoBytes(unsafe.Pointer(data), C.int(n))
}

func copyOut(b []byte, out **C.uint8_t, outLen *C.int32_t) {
	if out == nil || outLen == nil {
		return
	}
	*outLen = C.int32_t(len(b))
	if len(b) == 0 {
		*out = nil
		return
	}
	*out = (*C.uint8_t)(C.CBytes(b))
}
