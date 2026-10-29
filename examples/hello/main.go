// hello is the bridge and wire check app. Its hosts in ios/ and android/
// talk to it only through the typed messages in proto/, and print
// "NN-0C RESULT: PASS" when every check holds.
package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/CodeSyncr/nimbus-native/bridge"
	"github.com/CodeSyncr/nimbus-native/examples/hello/proto"
)

func init() {
	// Synchronous: the host blocks until Go answers.
	bridge.OnCall(func(req []byte) []byte {
		m, err := proto.Decode(req)
		if err != nil {
			panic(err) // reaches the host as NN_ERR_PANIC with this text
		}
		switch m := m.(type) {
		case *proto.Hello:
			return proto.Encode(&proto.Greeting{
				Text:      "Hello, " + m.Name + ", from Go",
				GoVersion: runtime.Version(),
				Platform:  runtime.GOOS + "/" + runtime.GOARCH,
			})
		case *proto.Echo:
			return proto.Encode(m)
		case *proto.Crash:
			panic(m.Reason)
		case *proto.StartBurst:
			go func() {
				for i := uint32(0); i < m.Count; i++ {
					bridge.Send(proto.Encode(&proto.Item{Index: i}))
				}
			}()
			return proto.Encode(&proto.Ack{OK: true})
		case *proto.StatsRequest:
			s := bridge.ReadStats(m.GC)
			return proto.Encode(&proto.Stats{Heap: s.HeapAlloc, Goroutines: uint32(s.Goroutines),
				Sent: uint64(s.Sent), Wakes: uint64(s.Wakes), Drains: uint64(s.Drains)})
		case *proto.SnapshotRequest:
			return proto.Encode(snapshot(int(m.Rows)))
		}
		panic(fmt.Sprintf("hello: unexpected call %T", m))
	})

	// Asynchronous: Ping in, Pong out.
	bridge.OnMessage(func(msg []byte) {
		if m, err := proto.Decode(msg); err == nil {
			if p, ok := m.(*proto.Ping); ok {
				bridge.Send(proto.Encode(&proto.Pong{Seq: p.Seq}))
			}
		}
	})

	// Go speaks first, from goroutines the host never created.
	bridge.OnStart(func() {
		bridge.Send(proto.Encode(&proto.Ready{GoVersion: runtime.Version()}))
		for i := uint32(1); i <= 3; i++ {
			time.Sleep(300 * time.Millisecond)
			bridge.Send(proto.Encode(&proto.Tick{N: i}))
		}
	})
}

func snapshot(n int) *proto.Snapshot {
	s := &proto.Snapshot{Rows: make([]proto.Row, n)}
	for i := range s.Rows {
		r := &s.Rows[i]
		r.ID = uint32(i)
		r.Title = fmt.Sprintf("Order #%d", 10000+i)
		r.Kind = proto.Kind(i % 3)
		r.Score = float64(i) * 1.5
		r.Tags = []string{"open", "today"}
		r.Attrs = map[string]string{"city": "Pune", "status": "paid"}
		if i%2 == 0 {
			sub := "Asha Rao · 2 items"
			r.Subtitle = &sub
		}
	}
	return s
}

// main is unused: the app is built as a library and the host owns the process.
func main() {}
