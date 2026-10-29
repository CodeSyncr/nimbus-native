// The Go side of the ABI test: host.c drives it through nn.h exactly as an
// iOS or Android host would.
package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/CodeSyncr/nimbus-native/bridge"
)

func init() {
	bridge.OnCall(func(req []byte) []byte {
		cmd, arg, _ := strings.Cut(string(req), ":")
		switch cmd {
		case "echo":
			return []byte(arg)
		case "panic":
			panic("asked to panic")
		case "burst":
			// burst:<goroutines>:<each> — every goroutine sends "<g>:<i>" in order.
			gs, each, _ := strings.Cut(arg, ":")
			g, _ := strconv.Atoi(gs)
			n, _ := strconv.Atoi(each)
			var wg sync.WaitGroup
			for gi := 0; gi < g; gi++ {
				wg.Add(1)
				go func(gi int) {
					defer wg.Done()
					for i := 0; i < n; i++ {
						bridge.Send([]byte(fmt.Sprintf("%d:%d", gi, i)))
					}
				}(gi)
			}
			return []byte("ok")
		case "stats":
			s := bridge.ReadStats(arg == "gc")
			return []byte(fmt.Sprintf("%d %d %d %d %d %d %d %d",
				s.Sent, s.Wakes, s.Drains, s.Received, s.Calls, s.Panics, s.Goroutines, s.HeapAlloc))
		}
		return nil
	})

	bridge.OnMessage(func(msg []byte) {
		if n, ok := strings.CutPrefix(string(msg), "ping:"); ok {
			bridge.Send([]byte("pong:" + n))
		}
	})
}

func main() {}
