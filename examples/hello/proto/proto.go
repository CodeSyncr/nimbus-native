// Package proto is the hello app's wire schema: every message between its
// Go code and its iOS / Android hosts. `nimbus-native gen` (run by every
// build) generates wire_gen.go here and the Swift / Java types.
package proto

// Kind is what a Row shows.
//
//wire:enum
type Kind uint8

const (
	KindText Kind = iota
	KindImage
	KindButton
)

// Hello asks Go to greet someone (sync call).
//
//wire:message
type Hello struct{ Name string }

// Greeting answers Hello.
//
//wire:message
type Greeting struct{ Text, GoVersion, Platform string }

// Echo comes back unchanged (sync call; used for call benchmarks).
//
//wire:message
type Echo struct{ Text string }

// Crash makes the Go handler panic, to check panic reporting.
//
//wire:message
type Crash struct{ Reason string }

// Ready is the first message Go sends after start.
//
//wire:message
type Ready struct{ GoVersion string }

// Tick is sent from a goroutine the host never created.
//
//wire:message
type Tick struct{ N uint32 }

//wire:message
type Ping struct{ Seq uint32 }

//wire:message
type Pong struct{ Seq uint32 }

// StartBurst makes Go send Count Items as fast as it can.
//
//wire:message
type StartBurst struct{ Count uint32 }

//wire:message
type Item struct{ Index uint32 }

//wire:message
type Ack struct{ OK bool }

//wire:message
type StatsRequest struct{ GC bool }

// Stats is bridge traffic and Go memory (bridge.ReadStats).
//
//wire:message
type Stats struct {
	Heap                uint64
	Goroutines          uint32
	Sent, Wakes, Drains uint64
}

// SnapshotRequest asks for a list shaped like real screen data.
//
//wire:message
type SnapshotRequest struct{ Rows uint32 }

//wire:message
type Snapshot struct{ Rows []Row }

// Row is one list item: the kind of data a UI batch carries.
type Row struct {
	ID       uint32
	Title    string
	Subtitle *string
	Kind     Kind
	Score    float64
	Tags     []string
	Attrs    map[string]string
}
