// nn.h — the Nimbus Native C ABI.
//
// This is the only boundary between the Go runtime and a native host
// (iOS, Android, later web). Everything that crosses it is a byte buffer;
// what the bytes mean is the wire format's job, not this header's.
//
// Ownership:
//   - Buffers passed INTO a function are borrowed for the duration of the
//     call. The callee copies what it keeps.
//   - Buffers RETURNED through an out parameter belong to the caller and are
//     released with nn_free.
//
// Threads:
//   - The host has one UI thread. nn_drain and nn_call are meant for it;
//     nn_send may be called from any thread.
//   - The runtime calls wake() from its own threads, never the UI thread's
//     stack. wake must not call back into the runtime; it only schedules
//     nn_drain on the UI thread.
//   - Go code never blocks the UI thread except inside nn_call, which the
//     app keeps short.
#ifndef NIMBUS_NATIVE_H
#define NIMBUS_NATIVE_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Bumped on any incompatible change to this header. nn_start refuses a host
// built against a different version.
#define NN_ABI_VERSION 2

// Result codes.
#define NN_OK 0
#define NN_ERR_ABI 1        // host and runtime disagree on NN_ABI_VERSION
#define NN_ERR_STATE 2      // nn_start not called, or called twice
#define NN_ERR_PANIC 3      // Go code panicked; the panic was recovered
#define NN_ERR_NO_HANDLER 4 // the app registered no handler for this call
#define NN_ERR_ARG 5        // a required argument was missing or invalid

// Go → native: messages are waiting. Called once each time the outbox goes
// from empty to non-empty, from any runtime thread. The host answers by
// calling nn_drain on its UI thread.
typedef void (*nn_wake_fn)(void *ctx);

// What the host gives the runtime at start.
typedef struct {
    uint32_t abi;     // must be NN_ABI_VERSION
    void *ctx;        // passed back untouched to wake
    nn_wake_fn wake;  // required
} nn_host;

#ifndef NN_NO_PROTOTYPES
// (The Go runtime defines NN_NO_PROTOTYPES: cgo declares these itself.)

// Boots the app's Go side. Call once, before anything else.
int32_t nn_start(const nn_host *host);

// Native → Go, asynchronous. Messages reach the app in the order sent.
int32_t nn_send(const uint8_t *data, int32_t len);

// Go → native: takes every message Go has sent since the last drain, in
// order, as one buffer of frames: [uint32 little-endian length][bytes]...
// *out is NULL and *out_len 0 when nothing is waiting.
int32_t nn_drain(uint8_t **out, int32_t *out_len);

// Native → Go, synchronous. On NN_OK, *out holds the reply; on NN_ERR_PANIC
// it holds the panic text and Go stack. Either way, free it with nn_free.
int32_t nn_call(const uint8_t *data, int32_t len, uint8_t **out, int32_t *out_len);

// Releases a buffer the runtime handed out.
void nn_free(void *p);

#endif // NN_NO_PROTOTYPES

#ifdef __cplusplus
}
#endif

#endif
