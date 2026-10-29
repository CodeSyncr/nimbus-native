package ui

import (
	"context"
	"fmt"
	"reflect"
)

// Ctx is a component instance's handle for hooks. Hooks must be called in
// the same order on every render (never inside if or for).
type Ctx struct {
	rt   *runtime
	inst *instance
	hook int
}

func (c *Ctx) slot() *hookSlot {
	if c.hook == len(c.inst.hooks) {
		c.inst.hooks = append(c.inst.hooks, &hookSlot{})
	}
	s := c.inst.hooks[c.hook]
	c.hook++
	return s
}

type hookSlot struct {
	value any
	deps  []any
	set   bool

	// Effect
	pending bool
	run     func(ctx context.Context) func()
	cancel  context.CancelFunc
	cleanup func()
}

// State is a value that survives re-renders. Calling set — from any
// goroutine — stores the new value and schedules a render.
func State[T any](c *Ctx, initial T) (T, func(T)) {
	s := c.slot()
	if !s.set {
		s.value, s.set = initial, true
	}
	rt, inst := c.rt, c.inst
	return s.value.(T), func(v T) {
		rt.do(func() {
			if inst.unmounted {
				return
			}
			s.value = v
			inst.dirty, rt.dirty = true, true
		})
	}
}

// Reducer is State for values updated by a function of the old value:
// update(func(n int) int { return n + 1 }) never loses an increment, even
// when called many times before the next render.
func Reducer[T any](c *Ctx, initial T) (T, func(func(T) T)) {
	s := c.slot()
	if !s.set {
		s.value, s.set = initial, true
	}
	rt, inst := c.rt, c.inst
	return s.value.(T), func(f func(T) T) {
		rt.do(func() {
			if inst.unmounted {
				return
			}
			s.value = f(s.value.(T))
			inst.dirty, rt.dirty = true, true
		})
	}
}

// Effect runs fn after a render that changed deps: every render if no deps
// are given, only the first with ui.Once. fn's context is
// cancelled — and its returned cleanup called — before it runs again and
// when the component leaves the screen. Effects run on the UI goroutine;
// start your own goroutine for slow work and set State from it.
func Effect(c *Ctx, fn func(ctx context.Context) func(), deps ...any) {
	s := c.slot()
	if s.set && deps != nil && depsEqual(s.deps, deps) {
		return
	}
	s.set, s.deps, s.run, s.pending = true, deps, fn, true
	c.rt.effects = append(c.rt.effects, s)
}

// Once as an Effect dependency runs the effect only after the first render.
var Once any = once{}

type once struct{}

// Memo returns fn's result, computing it again only when deps change.
func Memo[T any](c *Ctx, fn func() T, deps ...any) T {
	s := c.slot()
	if !s.set || !depsEqual(s.deps, deps) {
		s.value, s.deps, s.set = fn(), deps, true
	}
	return s.value.(T)
}

// Ref is a value that survives re-renders without causing them.
func Ref[T any](c *Ctx, initial T) *T {
	s := c.slot()
	if !s.set {
		v := initial
		s.value, s.set = &v, true
	}
	return s.value.(*T)
}

// Viewport is the screen the root fills: size, safe-area insets, platform.
func Viewport(c *Ctx) ViewportInfo { return c.rt.viewport }

// ViewportInfo describes the screen.
type ViewportInfo struct {
	W, H                                     float32
	Scale                                    float32
	SafeTop, SafeBottom, SafeLeft, SafeRight float32
	Platform                                 string // "ios" | "android"
	Dark                                     bool
}

func depsEqual(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !comparable(a[i]) || !comparable(b[i]) {
			if !reflect.DeepEqual(a[i], b[i]) {
				return false
			}
			continue
		}
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func comparable(v any) bool { return v == nil || reflect.TypeOf(v).Comparable() }

func (s *hookSlot) stop() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if s.cleanup != nil {
		f := s.cleanup
		s.cleanup = nil
		defer func() {
			if r := recover(); r != nil {
				logf("panic in effect cleanup: %v", r)
			}
		}()
		f()
	}
}

func (s *hookSlot) start() {
	s.stop()
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	defer func() {
		if r := recover(); r != nil {
			logf("panic in effect: %v", r)
		}
	}()
	s.cleanup = s.run(ctx)
}

var logf = func(format string, args ...any) { fmt.Printf(format+"\n", args...) }
