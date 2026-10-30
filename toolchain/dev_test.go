package toolchain

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CodeSyncr/nimbus-native/dev/frame"
	"github.com/CodeSyncr/nimbus-native/ui/render"
)

func measure(text string) []byte { return render.Encode(&render.MeasureText{Text: text}) }

type end struct {
	t *testing.T
	c net.Conn
}

func dial(t *testing.T, addr string, role byte, schema uint64) *end {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	frame.Write(c, frame.Hello, frame.HelloPayload(role, schema))
	return &end{t, c}
}

func (e *end) send(ty frame.Type, p []byte) { frame.Write(e.c, ty, p) }

func (e *end) expect(ty frame.Type) []byte {
	e.t.Helper()
	e.c.SetReadDeadline(time.Now().Add(2 * time.Second))
	got, p, err := frame.Read(e.c)
	if err != nil || got != ty {
		e.t.Fatalf("want frame %d, got %d (%v)", ty, got, err)
	}
	return p
}

func (e *end) expectNothing() {
	e.t.Helper()
	e.c.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	if ty, _, err := frame.Read(e.c); err == nil {
		e.t.Fatalf("unexpected frame %d", ty)
	}
}

func TestRelay(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	devices := make(chan struct{}, 4)
	r := &relay{con: newConsole(io.Discard), onDevice: func() { devices <- struct{}{} }}
	r.setExpected(42)
	go r.serve(ln)
	addr := ln.Addr().String()

	device := dial(t, addr, frame.RoleDevice, 42)
	<-devices
	app := dial(t, addr, frame.RoleApp, 42)
	device.expect(frame.Reset) // a new app process: clear the screen

	// Both directions are forwarded.
	app.send(frame.FromGo, []byte("commit"))
	if p := device.expect(frame.FromGo); string(p) != "commit" {
		t.Fatalf("device got %q", p)
	}
	device.send(frame.ToGo, []byte("tap"))
	if p := app.expect(frame.ToGo); string(p) != "tap" {
		t.Fatalf("app got %q", p)
	}

	// A host call goes to the device once; the same question is then
	// answered by the relay, even for the next app process.
	app.send(frame.HostCall, frame.WithID(1, measure("Hello")))
	id, req, _ := frame.SplitID(device.expect(frame.HostCall))
	device.send(frame.HostReply, frame.Reply(id, 0, []byte("40x20")))
	if _, _, reply, _ := frame.SplitReply(app.expect(frame.HostReply)); string(reply) != "40x20" || string(req) != string(measure("Hello")) {
		t.Fatalf("first answer %q for %q", reply, req)
	}

	app2 := dial(t, addr, frame.RoleApp, 42)
	device.expect(frame.Reset)
	app2.send(frame.HostCall, frame.WithID(9, measure("Hello")))
	if id, _, reply, _ := frame.SplitReply(app2.expect(frame.HostReply)); id != 9 || string(reply) != "40x20" {
		t.Fatalf("cached answer: id %d, %q", id, reply)
	}
	device.expectNothing() // the device was not asked again

	// A failed answer is not cached.
	app2.send(frame.HostCall, frame.WithID(10, measure("Bye")))
	id, _, _ = frame.SplitID(device.expect(frame.HostCall))
	device.send(frame.HostReply, frame.Reply(id, 3, nil))
	app2.expect(frame.HostReply)
	app2.send(frame.HostCall, frame.WithID(11, measure("Bye")))
	device.expect(frame.HostCall)

	// Plugin calls are never answered from the cache, even when identical.
	call := render.Encode(&render.PluginCall{ID: 1, Plugin: "secureStorage", Method: "get", Args: `{"key":"t"}`})
	for i := uint32(20); i < 22; i++ {
		app2.send(frame.HostCall, frame.WithID(i, call))
		id, _, _ := frame.SplitID(device.expect(frame.HostCall))
		device.send(frame.HostReply, frame.Reply(id, 0, render.Encode(&render.PluginAck{})))
		app2.expect(frame.HostReply)
	}

	// A new device starts a fresh cache and restarts the app.
	device2 := dial(t, addr, frame.RoleDevice, 42)
	<-devices
	app2.send(frame.HostCall, frame.WithID(12, measure("Hello")))
	device2.expect(frame.HostCall)
}

func TestSnapshotIgnoresGeneratedAndBuild(t *testing.T) {
	dir := t.TempDir()
	write := func(name, s string) {
		t.Helper()
		p := dir + "/" + name
		if err := mkdirFor(p); err != nil {
			t.Fatal(err)
		}
		if err := writeFile(p, s); err != nil {
			t.Fatal(err)
		}
	}
	write("main.go", "package main")
	before := snapshot(dir)
	write("proto/wire_gen.go", "generated")
	write("build/dev/x.go", "output")
	if snapshot(dir) != before {
		t.Fatal("generated files or build output count as changes")
	}
	time.Sleep(10 * time.Millisecond)
	write("main.go", "package main // edited")
	if snapshot(dir) == before {
		t.Fatal("an edit was not noticed")
	}
}

func mkdirFor(p string) error { return os.MkdirAll(filepath.Dir(p), 0o755) }

func writeFile(p, s string) error { return os.WriteFile(p, []byte(s), 0o644) }
