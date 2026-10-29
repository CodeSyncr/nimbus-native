//go:build nimbusdev

package bridge

import (
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CodeSyncr/nimbus-native/dev/frame"
	"github.com/CodeSyncr/nimbus-native/wire"
)

// Dev mode: the app runs on the laptop and reaches the device through the
// relay in `nimbus native dev` (NIMBUS_DEV_ADDR). The API is the same as on
// a device; only the transport differs.

var (
	conn    net.Conn
	writeMu sync.Mutex
	nextID  atomic.Uint32
	waitMu  sync.Mutex
	waiting = map[uint32]chan hostReply{}
)

type hostReply struct {
	rc    byte
	reply []byte
}

// Main runs the app: it connects to the relay, starts the runtime and
// serves the device until the relay goes away.
func Main() {
	addr := os.Getenv("NIMBUS_DEV_ADDR")
	if addr == "" {
		fmt.Fprintln(os.Stderr, "nimbus native dev: NIMBUS_DEV_ADDR is not set (run the app with `nimbus native dev`)")
		os.Exit(2)
	}
	var err error
	for i := 0; i < 50; i++ {
		if conn, err = net.Dial("tcp", addr); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "nimbus native dev: cannot reach the relay:", err)
		os.Exit(1)
	}
	if err := write(frame.Hello, frame.HelloPayload(frame.RoleApp, wire.Combined())); err != nil {
		os.Exit(1)
	}
	started.Store(true)
	begin()

	for {
		t, p, err := frame.Read(conn)
		if err != nil {
			os.Exit(0) // the relay closed: a newer build is taking over
		}
		switch t {
		case frame.ToGo:
			stats.received.Add(1)
			inbox <- p
		case frame.Call:
			id, req, ok := frame.SplitID(p)
			if !ok {
				continue
			}
			go func() {
				reply, rc := handleCall(req)
				write(frame.CallReply, frame.Reply(id, byte(rc), reply))
			}()
		case frame.HostReply:
			id, rc, reply, ok := frame.SplitReply(p)
			if !ok {
				continue
			}
			waitMu.Lock()
			ch := waiting[id]
			delete(waiting, id)
			waitMu.Unlock()
			if ch != nil {
				ch <- hostReply{rc, reply}
			}
		}
	}
}

// Send delivers msg to the host, in order. Safe from any goroutine.
func Send(msg []byte) error {
	if !started.Load() {
		return ErrNotStarted
	}
	stats.sent.Add(1)
	return write(frame.FromGo, msg)
}

// CallHost asks the device something only it can answer and waits for the
// reply, which crosses the network both ways.
func CallHost(req []byte) ([]byte, error) {
	if !started.Load() {
		return nil, ErrNotStarted
	}
	stats.hostCalls.Add(1)
	id := nextID.Add(1)
	ch := make(chan hostReply, 1)
	waitMu.Lock()
	waiting[id] = ch
	waitMu.Unlock()
	if err := write(frame.HostCall, frame.WithID(id, req)); err != nil {
		return nil, err
	}
	select {
	case r := <-ch:
		if r.rc != codeOK {
			return r.reply, &HostError{Code: int(r.rc)}
		}
		return r.reply, nil
	case <-time.After(10 * time.Second):
		waitMu.Lock()
		delete(waiting, id)
		waitMu.Unlock()
		return nil, &HostError{Code: codeState}
	}
}

func write(t frame.Type, p []byte) error {
	writeMu.Lock()
	defer writeMu.Unlock()
	return frame.Write(conn, t, p)
}
