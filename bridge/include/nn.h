// nn.h — the Nimbus Native C ABI.
//
// This is the only boundary between the Go runtime and a native host
// (iOS, Android, later web). Everything that crosses it is a byte buffer;
// what the bytes mean is the wire format's job, not this header's.
//
// Ownership rules:
//   - Buffers passed INTO a function are borrowed for the duration of the
//     call. The callee copies what it keeps.
//   - Buffers RETURNED through an out parameter are owned by the caller and
//     must be released with nn_free.
#ifndef NIMBUS_NATIVE_H
#define NIMBUS_NATIVE_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Bumped on any incompatible change to this header. nn_start refuses a host
// built against a different version.
#define NN_ABI_VERSION 1

// Result codes.
#define NN_OK 0
#define NN_ERR_ABI 1        // host and runtime disagree on NN_ABI_VERSION
#define NN_ERR_STATE 2      // nn_start not called, or called twice
#define NN_ERR_PANIC 3      // Go code panicked; the panic was recovered
#define NN_ERR_NO_HANDLER 4 // the app registered no handler for this call

// Go → native: a message for the host. `data` is borrowed for the call only.
// May be invoked from any thread; the host hops to its UI thread itself.
typedef void (*nn_receive_fn)(void *ctx, const uint8_t *data, int32_t len);

// What the host gives the runtime at start.
typedef struct {
    uint32_t abi;            // must be NN_ABI_VERSION
    void *ctx;               // passed back untouched to every callback
    nn_receive_fn receive;   // required
} nn_host;

#ifndef NN_NO_PROTOTYPES
// (The Go runtime defines NN_NO_PROTOTYPES: cgo declares these itself.)

// Boots the app's Go side. Call once, before anything else.
int32_t nn_start(const nn_host *host);

// Native → Go, asynchronous. Messages are delivered to the app in order.
int32_t nn_send(const uint8_t *data, int32_t len);

// Native → Go, synchronous. On NN_OK, *out holds the reply (free with
// nn_free); on NN_ERR_PANIC it holds the panic text.
int32_t nn_call(const uint8_t *data, int32_t len, uint8_t **out, int32_t *out_len);

// Releases a buffer the runtime handed out.
void nn_free(void *p);

#endif // NN_NO_PROTOTYPES

#ifdef __cplusplus
}
#endif

#endif
