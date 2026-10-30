// Package client is the Go side of a dev shell: built into the app on the
// device instead of the app's own Go code, it forwards bridge traffic to
// the relay in `nimbus native dev`, where the app runs on the laptop. The
// native host is unchanged: it still talks to an in-process Go runtime.
package client

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CodeSyncr/nimbus-native/bridge"
	"github.com/CodeSyncr/nimbus-native/dev/frame"
	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// Set when the shell is built: the relay's address, and the combined schema
// hash of the app the shell was built for (the relay checks each app
// process against it).
var (
	Addr      string
	AppSchema uint64
	Session   uint64 // the dev session this shell belongs to
)

var (
	mu       sync.Mutex
	conn     net.Conn
	viewport []byte              // the host's last Viewport, replayed to each new app process
	live     = map[uint32]bool{} // native views the current app process has created

	nextID  atomic.Uint32
	waitMu  sync.Mutex
	waiting = map[uint32]chan []byte{}
)

func init() {
	bridge.OnStart(run)
	bridge.OnMessage(fromHost)
	bridge.OnCall(callFromHost)
}

// run keeps a connection to the relay, reconnecting when it drops.
func run() {
	logged := false
	for {
		c, err := net.Dial("tcp", Addr)
		if err != nil {
			if !logged {
				bridge.Logf("dev: waiting for nimbus native dev at %s", Addr)
				logged = true
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}
		logged = false
		mu.Lock()
		conn = c
		mu.Unlock()
		send(frame.Hello, frame.HelloPayload(frame.RoleDevice, AppSchema, Session))
		began := time.Now()
		serve(c)
		if time.Since(began) < time.Second {
			// Turned away (a newer session's relay): don't hammer it.
			time.Sleep(2 * time.Second)
		}
		mu.Lock()
		conn = nil
		mu.Unlock()
		c.Close()
	}
}

func serve(c net.Conn) {
	for {
		t, p, err := frame.Read(c)
		if err != nil {
			return
		}
		switch t {
		case frame.FromGo:
			track(p)
			bridge.Send(p)
		case frame.CallReply:
			id, _, reply, ok := frame.SplitReply(p)
			if ok {
				waitMu.Lock()
				ch := waiting[id]
				delete(waiting, id)
				waitMu.Unlock()
				if ch != nil {
					ch <- reply
				}
			}
		case frame.HostCall:
			id, req, ok := frame.SplitID(p)
			if !ok {
				continue
			}
			go func() {
				reply, err := bridge.CallHost(req)
				rc := byte(0)
				if he, ok := err.(*bridge.HostError); ok {
					rc = byte(he.Code)
				}
				send(frame.HostReply, frame.Reply(id, rc, reply))
			}()
		case frame.Reset:
			reset()
		}
	}
}

// track records which views exist, from the commits passing through.
func track(p []byte) {
	m, err := render.Decode(p)
	if err != nil {
		return
	}
	c, ok := m.(*render.Commit)
	if !ok {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	for _, op := range c.Ops {
		switch o := op.(type) {
		case *render.Create:
			live[o.Tag] = true
		case *render.Destroy:
			for _, t := range o.Tags {
				delete(live, t)
			}
		}
	}
}

// reset clears the screen for a new app process and gives it the viewport.
func reset() {
	mu.Lock()
	tags := make([]uint32, 0, len(live))
	for t := range live {
		tags = append(tags, t)
	}
	live = map[uint32]bool{}
	vp := viewport
	mu.Unlock()
	if len(tags) > 0 {
		bridge.Send(render.Encode(&render.Commit{Ops: []render.Op{&render.Destroy{Tags: tags}}}))
	}
	if vp != nil {
		send(frame.ToGo, vp)
	}
}

func fromHost(msg []byte) {
	if m, err := render.Decode(msg); err == nil {
		if _, ok := m.(*render.Viewport); ok {
			mu.Lock()
			viewport = msg
			mu.Unlock()
		}
	}
	send(frame.ToGo, msg)
}

func callFromHost(req []byte) []byte {
	id := nextID.Add(1)
	ch := make(chan []byte, 1)
	waitMu.Lock()
	waiting[id] = ch
	waitMu.Unlock()
	if !send(frame.Call, frame.WithID(id, req)) {
		return nil
	}
	select {
	case r := <-ch:
		return r
	case <-time.After(5 * time.Second):
		return nil
	}
}

func send(t frame.Type, p []byte) bool {
	mu.Lock()
	defer mu.Unlock()
	if conn == nil {
		return false
	}
	return frame.Write(conn, t, p) == nil
}
