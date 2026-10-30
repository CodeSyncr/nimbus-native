// Package plugin calls native code: a method of a plugin registered in the
// iOS or Android host, written in Swift or Kotlin. Arguments and results
// travel as JSON, so a plugin needs no generated code on either side.
//
//	type token struct{ Value string `json:"value"` }
//	t, err := plugin.Call[token](ctx, "secureStorage", "get", map[string]string{"key": "token"})
//
// Call blocks until the native side answers, so call it where blocking is
// fine: in ui.UseQuery, ui.UseAction, an effect or your own goroutine, not
// in Render. The host runs every plugin method on its main thread.
//
// Plugins are registered in the host:
//
//	Plugins.register("battery") { call in call.resolve(UIDevice.current.batteryLevel) }   // Swift
//	Plugins.register("battery") { call -> call.resolve(level) }                           // Kotlin
package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CodeSyncr/nimbus-native/bridge"
	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// Error is a failed plugin call.
type Error struct {
	Plugin, Method string
	// Code says what kind of failure: "unavailable" (no such plugin or
	// method here), "cancelled" (the user backed out), "denied" (no
	// permission), "not_found", or "failed".
	Code    string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("plugin %s.%s: %s: %s", e.Plugin, e.Method, e.Code, e.Message)
}

// Codes of Error.
const (
	Unavailable = "unavailable"
	Cancelled   = "cancelled"
	Denied      = "denied"
	NotFound    = "not_found"
	Failed      = "failed"
)

// IsCode reports whether err is a plugin Error with the given code.
func IsCode(err error, code string) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == code
}

var (
	// nextID starts somewhere random so call IDs never repeat across the
	// app restarts of dev mode.
	nextID  = func() *atomic.Uint64 { var n atomic.Uint64; n.Store(uint64(time.Now().UnixNano()) << 8); return &n }()
	mu      sync.Mutex
	pending = map[uint64]chan *render.PluginResult{}

	// callHost reaches the host; tests replace it.
	callHost = bridge.CallHost
)

// Call runs method on the named plugin with args (any JSON-encodable value,
// or nil) and decodes the answer into Resp. Use struct{} or any for a method
// with no useful answer.
func Call[Resp any](ctx context.Context, name, method string, args any) (Resp, error) {
	var zero Resp
	raw, err := CallJSON(ctx, name, method, args)
	if err != nil {
		return zero, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return zero, nil
	}
	var out Resp
	if err := json.Unmarshal(raw, &out); err != nil {
		return zero, &Error{Plugin: name, Method: method, Code: Failed, Message: fmt.Sprintf("decoding %T: %v", out, err)}
	}
	return out, nil
}

// CallJSON is Call with the raw JSON answer.
func CallJSON(ctx context.Context, name, method string, args any) (json.RawMessage, error) {
	b, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("plugin %s.%s: encoding args: %w", name, method, err)
	}
	id := nextID.Add(1)
	ch := make(chan *render.PluginResult, 1)
	mu.Lock()
	pending[id] = ch
	mu.Unlock()
	defer func() {
		mu.Lock()
		delete(pending, id)
		mu.Unlock()
	}()

	reply, err := callHost(render.Encode(&render.PluginCall{ID: id, Plugin: name, Method: method, Args: string(b)}))
	if err != nil {
		return nil, &Error{Plugin: name, Method: method, Code: Unavailable, Message: err.Error()}
	}
	if m, err := render.Decode(reply); err == nil {
		if ack, ok := m.(*render.PluginAck); ok && ack.Err != "" {
			return nil, &Error{Plugin: name, Method: method, Code: Unavailable,
				Message: ack.Err + " (if it comes with Nimbus Native, `nimbus native plugin add " + name + "` adds it)"}
		}
	}

	select {
	case r := <-ch:
		if r.Code != "" || r.Err != "" {
			code := r.Code
			if code == "" {
				code = Failed
			}
			return nil, &Error{Plugin: name, Method: method, Code: code, Message: r.Err}
		}
		return json.RawMessage(r.Value), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Deliver hands a plugin's outcome to the waiting Call. The UI runtime
// calls it for every PluginResult the host sends.
func Deliver(r *render.PluginResult) {
	mu.Lock()
	ch := pending[r.ID]
	mu.Unlock()
	if ch != nil {
		ch <- r
	}
}
