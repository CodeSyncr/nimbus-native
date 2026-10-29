// hello is the bridge check app: the host calls Go synchronously, sends it
// messages, and Go pushes messages back on its own goroutines. The hosts in
// ios/ and android/ run the same checks and print "NN-0B RESULT: PASS".
package main

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/CodeSyncr/nimbus-native/bridge"
)

func init() {
	// Synchronous: the host blocks until Go answers.
	bridge.OnCall(func(req []byte) []byte {
		cmd, arg, _ := strings.Cut(string(req), ":")
		switch cmd {
		case "hello":
			return []byte(fmt.Sprintf("Hello, %s, from Go %s on %s/%s", arg, runtime.Version(), runtime.GOOS, runtime.GOARCH))
		case "echo":
			return []byte(arg)
		case "panic":
			panic("asked to panic")
		case "burst":
			// Go sends <n> messages from a goroutine; the host counts them.
			n, _ := strconv.Atoi(arg)
			go func() {
				for i := 0; i < n; i++ {
					bridge.Send([]byte("b:" + strconv.Itoa(i)))
				}
			}()
			return []byte("ok")
		case "stats":
			s := bridge.ReadStats(arg == "gc")
			return []byte(fmt.Sprintf("%d %d %d %d %d", s.HeapAlloc, s.Goroutines, s.Sent, s.Wakes, s.Drains))
		}
		return nil
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
			time.Sleep(300 * time.Millisecond)
			bridge.Send([]byte(fmt.Sprintf("tick:%d", i)))
		}
	})
}

// main is unused: the app is built as a library and the host owns the process.
func main() {}
