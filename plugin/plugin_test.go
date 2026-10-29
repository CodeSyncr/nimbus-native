package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// fakeHost answers PluginCalls the way the kits do: an ack now, the result
// later on another goroutine.
func fakeHost(t *testing.T, handle func(c *render.PluginCall) *render.PluginResult) {
	t.Helper()
	old := callHost
	t.Cleanup(func() { callHost = old })
	callHost = func(req []byte) ([]byte, error) {
		m, err := render.Decode(req)
		if err != nil {
			t.Fatal(err)
		}
		c := m.(*render.PluginCall)
		if c.Plugin == "missing" {
			return render.Encode(&render.PluginAck{Err: "no plugin named missing"}), nil
		}
		go func() {
			if r := handle(c); r != nil {
				r.ID = c.ID
				Deliver(r)
			}
		}()
		return render.Encode(&render.PluginAck{}), nil
	}
}

func TestCall(t *testing.T) {
	fakeHost(t, func(c *render.PluginCall) *render.PluginResult {
		switch c.Method {
		case "echo":
			var args map[string]string
			json.Unmarshal([]byte(c.Args), &args)
			b, _ := json.Marshal(map[string]string{"value": args["key"] + "!"})
			return &render.PluginResult{Value: string(b)}
		case "deny":
			return &render.PluginResult{Code: Denied, Err: "no access"}
		}
		return nil // never answers
	})
	ctx := context.Background()

	type answer struct{ Value string }
	got, err := Call[answer](ctx, "test", "echo", map[string]string{"key": "hi"})
	if err != nil || got.Value != "hi!" {
		t.Fatalf("echo: %+v, %v", got, err)
	}

	_, err = Call[any](ctx, "test", "deny", nil)
	if !IsCode(err, Denied) {
		t.Fatalf("deny: %v", err)
	}

	_, err = Call[any](ctx, "missing", "x", nil)
	if !IsCode(err, Unavailable) {
		t.Fatalf("missing plugin: %v", err)
	}

	ctx2, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := Call[any](ctx2, "test", "hang", nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("hang: %v", err)
	}
	mu.Lock()
	n := len(pending)
	mu.Unlock()
	if n != 0 {
		t.Fatalf("%d calls left pending", n)
	}
}
