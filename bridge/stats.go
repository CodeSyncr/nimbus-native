package bridge

import (
	"runtime"
	"sync/atomic"
)

type counters struct {
	sent, wakes, drains     atomic.Int64 // Go → native
	received, calls, panics atomic.Int64 // native → Go
}

// Stats is a snapshot of bridge traffic and Go memory, for soak tests and
// (later) the DevTools inspector.
type Stats struct {
	Sent     int64 // messages queued by Send
	Wakes    int64 // times the host was woken
	Drains   int64 // non-empty batches the host took
	Received int64 // messages from nn_send
	Calls    int64 // nn_call invocations
	Panics   int64 // panics recovered at the boundary

	Goroutines int
	HeapAlloc  uint64 // bytes of live Go heap
	Sys        uint64 // bytes the Go runtime holds from the OS
}

// ReadStats returns current counters. With gc, it collects first so
// HeapAlloc reflects live memory only.
func ReadStats(gc bool) Stats {
	if gc {
		runtime.GC()
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return Stats{
		Sent: stats.sent.Load(), Wakes: stats.wakes.Load(), Drains: stats.drains.Load(),
		Received: stats.received.Load(), Calls: stats.calls.Load(), Panics: stats.panics.Load(),
		Goroutines: runtime.NumGoroutine(), HeapAlloc: m.HeapAlloc, Sys: m.Sys,
	}
}
