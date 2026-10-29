package bridge

import "sync"

// Handle is how a host refers to a Go value — a callback, a view's event
// handler — across the boundary. Hosts never see Go pointers, so the
// garbage collector never has to reason about native code.
type Handle uint32

// Handles maps handles to Go values. The zero value is ready to use; it is
// safe from any goroutine. Handle 0 is never issued, so hosts can use it
// for "none".
type Handles[T any] struct {
	mu   sync.Mutex
	next Handle
	m    map[Handle]T
	free []Handle // released handles, reused before minting new ones
}

// Put stores v and returns its handle.
func (h *Handles[T]) Put(v T) Handle {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.m == nil {
		h.m = make(map[Handle]T)
	}
	var id Handle
	if n := len(h.free); n > 0 {
		id, h.free = h.free[n-1], h.free[:n-1]
	} else {
		h.next++
		id = h.next
	}
	h.m[id] = v
	return id
}

// Get returns the value for id, or false if it was never issued or has been
// released — a host acting on a stale handle gets a miss, not a crash.
func (h *Handles[T]) Get(id Handle) (T, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	v, ok := h.m[id]
	return v, ok
}

// Release forgets id; releasing twice is harmless.
func (h *Handles[T]) Release(id Handle) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.m[id]; ok {
		delete(h.m, id)
		h.free = append(h.free, id)
	}
}

// Len is the number of live handles — a leak shows up as a Len that only
// grows.
func (h *Handles[T]) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.m)
}
