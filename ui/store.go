package ui

import "sync"

// Store is state the whole app shares, such as a cart or the signed-in
// user: one value per app on a device, and one per browser tab on the web,
// where every visitor's UI runs in the same server. Declare it once, at
// package level:
//
//	var Cart = ui.NewStore(func() CartState { return CartState{} })
//
// Components read it with Use, which re-renders them when it changes, and
// change it from handlers, effects and actions with Update:
//
//	cart := Cart.Use(c)
//	ui.Pressable(func() { Cart.Update(c, func(s CartState) CartState { return s.Add(p) }) }, …)
//
// Keep T a value: Update gets the current one and returns the next.
type Store[T any] struct{ init func() T }

// NewStore declares a store; init makes its first value in each app or tab.
func NewStore[T any](init func() T) *Store[T] { return &Store[T]{init: init} }

type storeState struct {
	mu    sync.Mutex
	value any
	subs  map[*instance]bool // touched on the UI loop only
}

func (s *Store[T]) state(rt *runtime) *storeState {
	rt.storesMu.Lock()
	defer rt.storesMu.Unlock()
	st := rt.stores[s]
	if st == nil {
		st = &storeState{value: s.init(), subs: map[*instance]bool{}}
		rt.stores[s] = st
	}
	return st
}

// Use is the store's value, and re-renders the component when it changes.
// Call it while rendering, like any hook.
func (s *Store[T]) Use(c *Ctx) T {
	st := s.state(c.rt)
	st.subs[c.inst] = true
	return s.Get(c)
}

// Get is the store's value without subscribing: for handlers, effects and
// actions (safe from any goroutine).
func (s *Store[T]) Get(c *Ctx) T {
	st := s.state(c.rt)
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.value.(T)
}

// Update replaces the value with f(current) and re-renders the components
// using it. Safe from any goroutine.
func (s *Store[T]) Update(c *Ctx, f func(T) T) {
	rt := c.rt
	st := s.state(rt)
	st.mu.Lock()
	st.value = f(st.value.(T))
	st.mu.Unlock()
	rt.do(func() {
		for in := range st.subs {
			if in.unmounted {
				delete(st.subs, in)
				continue
			}
			in.dirty, rt.dirty = true, true
		}
	})
}
