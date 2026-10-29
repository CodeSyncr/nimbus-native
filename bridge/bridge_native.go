//go:build !nimbusdev

package bridge

/*
#cgo CFLAGS: -I${SRCDIR}/include
#define NN_NO_PROTOTYPES
#include <stdlib.h>
#include "nn.h"

static void nn_wake(const nn_host *h) {
	h->wake(h->ctx);
}

static int32_t nn_host_call(const nn_host *h, const uint8_t *req, int32_t len, uint8_t **out, int32_t *out_len) {
	if (h->call == NULL) return NN_ERR_NO_HANDLER;
	return h->call(h->ctx, req, len, out, out_len);
}
*/
import "C"

import (
	"encoding/binary"
	"sync"
	"unsafe"

	"github.com/CodeSyncr/nimbus-native/wire"
)

// ABIVersion is the nn.h version this runtime speaks.
const ABIVersion = C.NN_ABI_VERSION

// An outbox that grew past this is dropped after a drain instead of kept for
// reuse, so one burst doesn't pin its memory forever.
const keepOutbox = 1 << 20

var (
	host *C.nn_host // C-allocated copy of what the host gave nn_start

	outMu  sync.Mutex
	outbox []byte // frames waiting for nn_drain: [u32 LE length][bytes]...
)

// Main is for the app's main function. On a device the host owns the
// process and main never runs; in dev mode (bridge_dev.go) Main runs the app.
func Main() {}

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

// CallHost asks the host something only it can answer, synchronously — text
// measurement during layout, later native modules. Safe from any goroutine.
func CallHost(req []byte) ([]byte, error) {
	if !started.Load() {
		return nil, ErrNotStarted
	}
	var p *C.uint8_t
	if len(req) > 0 {
		p = (*C.uint8_t)(unsafe.Pointer(&req[0]))
	}
	var out *C.uint8_t
	var outLen C.int32_t
	rc := C.nn_host_call(host, p, C.int32_t(len(req)), &out, &outLen)
	stats.hostCalls.Add(1)
	var reply []byte
	if out != nil {
		reply = C.GoBytes(unsafe.Pointer(out), C.int(outLen))
		C.free(unsafe.Pointer(out))
	}
	if rc != C.NN_OK {
		return reply, &HostError{Code: int(rc)}
	}
	return reply, nil
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
		Logf("schema mismatch: host %#x, runtime %#x — regenerate with nimbus native gen", uint64(h.schema), wire.Combined())
		return C.NN_ERR_SCHEMA
	}
	if !started.CompareAndSwap(false, true) {
		return C.NN_ERR_STATE
	}
	host = (*C.nn_host)(C.malloc(C.size_t(unsafe.Sizeof(*h))))
	*host = *h
	begin()
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
func nn_call(data *C.uint8_t, n C.int32_t, out **C.uint8_t, outLen *C.int32_t) C.int32_t {
	if !started.Load() {
		return C.NN_ERR_STATE
	}
	reply, code := handleCall(copyIn(data, n))
	if code == codeNoHandler {
		return C.NN_ERR_NO_HANDLER
	}
	copyOut(reply, out, outLen)
	return C.int32_t(code)
}

//export nn_free
func nn_free(p unsafe.Pointer) { C.free(p) }

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
