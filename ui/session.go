package ui

import (
	"time"

	"github.com/CodeSyncr/nimbus-native/layout"
	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// Session is a UI runtime that is not the device's: the web host runs one
// per browser tab. It speaks the same render protocol as the phone hosts:
// Receive takes host messages, and SessionOptions.Send gets commits.
type Session struct{ rt *runtime }

// SessionOptions configure a Session.
type SessionOptions struct {
	// Send gets each render.Commit, encoded.
	Send func(commit []byte)
	// Style, when set, leaves layout to the host (a browser's CSS
	// flexbox): no Frame ops are sent, Style gets each view's layout style
	// whenever it changes, and Lists render every item.
	Style func(tag uint32, s layout.Style)
	// Values are what SessionValue returns to components.
	Values map[any]any
}

// NewSession starts a runtime for root. It renders once the host sends a
// render.Viewport.
func NewSession(root Component, o SessionOptions) *Session {
	rt := newRuntime(root)
	rt.send = o.Send
	rt.measure = func(*render.MeasureText) render.TextSize { return render.TextSize{} }
	rt.hostLayout = o.Style
	rt.values = o.Values
	go rt.loop()
	return &Session{rt: rt}
}

// Receive handles an encoded message from the host.
func (s *Session) Receive(msg []byte) { s.rt.receive(msg) }

// Do runs f on the session's UI loop and renders what it changed.
func (s *Session) Do(f func()) {
	s.rt.do(func() {
		f()
		s.rt.dirty = true
	})
}

// Settle waits, up to timeout, until the UI stops changing: no queued work
// and no query still loading. Server rendering uses it to send a page with
// its data, not a spinner. It reports whether the session settled.
func (s *Session) Settle(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		done := make(chan bool, 1)
		s.rt.do(func() {
			busy := s.rt.dirty
			for _, q := range s.rt.queries {
				if q.loading {
					busy = true
				}
			}
			done <- !busy
		})
		select {
		case settled := <-done:
			if settled {
				return true
			}
		case <-time.After(time.Until(deadline)):
			return false
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Close stops the session: effects stop, and its loop ends.
func (s *Session) Close() {
	s.rt.do(func() {
		if s.rt.rootInst != nil {
			s.rt.unmount(s.rt.rootInst)
		}
		s.rt.mu.Lock()
		s.rt.closed = true
		s.rt.mu.Unlock()
	})
}

// SessionValue is the value the session was started with under key (see
// SessionOptions.Values), or nil on a device.
func SessionValue(c *Ctx, key any) any { return c.rt.values[key] }
