// hello is the Phase 0a round-trip app: the host calls Go synchronously,
// sends it messages, and Go pushes messages back on its own goroutines.
package main

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/CodeSyncr/nimbus-native/bridge"
)

func init() {
	// Synchronous: the host blocks until Go answers.
	bridge.OnCall(func(req []byte) []byte {
		name := string(req)
		if name == "panic" {
			panic("asked to panic")
		}
		return []byte(fmt.Sprintf("Hello, %s, from Go %s on %s/%s", name, runtime.Version(), runtime.GOOS, runtime.GOARCH))
	})

	// Asynchronous: "ping:<n>" comes in, "pong:<n>" goes out.
	bridge.OnMessage(func(msg []byte) {
		if n, ok := strings.CutPrefix(string(msg), "ping:"); ok {
			bridge.Send([]byte("pong:" + n))
		}
	})

	// Go speaks first, from goroutines the host never created.
	bridge.OnStart(func() {
		bridge.Send([]byte("ready"))
		for i := 1; i <= 3; i++ {
			time.Sleep(500 * time.Millisecond)
			bridge.Send([]byte(fmt.Sprintf("tick:%d", i)))
		}
	})
}

// main is unused: the app is built as a library and the host owns the process.
func main() {}
