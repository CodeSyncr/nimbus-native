package frame

import (
	"bytes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	var b bytes.Buffer
	Write(&b, Hello, HelloPayload(RoleApp, 0xfeed))
	Write(&b, HostCall, WithID(7, []byte("measure")))
	Write(&b, HostReply, Reply(7, 3, []byte("panic")))
	Write(&b, Reset, nil)

	ty, p, _ := Read(&b)
	if role, schema, ok := ParseHello(p); ty != Hello || !ok || role != RoleApp || schema != 0xfeed {
		t.Fatalf("hello: %v %v %v %x", ty, ok, role, schema)
	}
	ty, p, _ = Read(&b)
	if id, req, ok := SplitID(p); ty != HostCall || !ok || id != 7 || string(req) != "measure" {
		t.Fatalf("host call: %v %v %d %q", ty, ok, id, req)
	}
	ty, p, _ = Read(&b)
	if id, rc, reply, ok := SplitReply(p); ty != HostReply || !ok || id != 7 || rc != 3 || string(reply) != "panic" {
		t.Fatalf("reply: %v %v %d %d %q", ty, ok, id, rc, reply)
	}
	if ty, p, err := Read(&b); ty != Reset || len(p) != 0 || err != nil {
		t.Fatalf("reset: %v %v %v", ty, p, err)
	}
	if _, _, err := Read(&b); err == nil {
		t.Fatal("read past the end")
	}
}

func TestOversizedFrameRejected(t *testing.T) {
	b := bytes.NewReader([]byte{byte(FromGo), 0xff, 0xff, 0xff, 0xff})
	if _, _, err := Read(b); err == nil {
		t.Fatal("a 4 GB frame header was accepted")
	}
}
