package ui

import (
	"context"
	"time"
)

// StaleAfter is how old cached query data may be before a component that
// mounts with it fetches again in the background.
var StaleAfter = 30 * time.Second

// Query is a UseQuery's state.
type Query[T any] struct {
	Data    T
	Loading bool  // a fetch is running (Data may still hold the last result)
	Loaded  bool  // Data holds a result
	Err     error // the last fetch's error, if it failed
	Refetch func()
}

// UseQuery loads data off the UI goroutine and re-renders when it arrives.
// Results are cached by key and shared: components using the same key make
// one request and see the same data. Cached data shows at once and is
// fetched again in the background when older than StaleAfter.
//
//	q := ui.UseQuery(c, "orders", func(ctx context.Context) ([]Order, error) {
//		return api.Call(ctx, client, shared.ListOrders, shared.ListOrdersReq{})
//	})
//	if q.Loading && !q.Loaded { return ui.Spinner(ui.Style{}) }
func UseQuery[T any](c *Ctx, key string, fetch func(ctx context.Context) (T, error)) Query[T] {
	rt, in := c.rt, c.inst
	e := rt.queries[key]
	if e == nil {
		e = &queryEntry{subs: map[*instance]bool{}}
		rt.queries[key] = e
	}
	e.fetch = func(ctx context.Context) (any, error) { return fetch(ctx) }
	e.subs[in] = true
	// Leave the cache's subscribers when the component goes. Called on every
	// render, so the hooks after this one keep their places.
	Effect(c, func(context.Context) func() {
		return func() { rt.do(func() { delete(e.subs, in) }) }
	}, Once)
	if !e.loading && (!e.started || (e.loaded && time.Since(e.at) > StaleAfter)) {
		rt.startQuery(e)
	}
	q := Query[T]{Loading: e.loading, Loaded: e.loaded, Err: e.err, Refetch: func() { rt.do(func() { rt.startQuery(e) }) }}
	if e.loaded {
		q.Data, _ = e.data.(T)
	}
	return q
}

// RefetchQuery fetches the query with this key again (after a change, say),
// re-rendering every component that shows it.
func RefetchQuery(c *Ctx, key string) {
	rt := c.rt
	rt.do(func() {
		if e := rt.queries[key]; e != nil && e.fetch != nil {
			rt.startQuery(e)
		}
	})
}

type queryEntry struct {
	fetch   func(context.Context) (any, error)
	data    any
	err     error
	at      time.Time
	started bool
	loading bool
	loaded  bool
	subs    map[*instance]bool
	cancel  context.CancelFunc
}

// startQuery runs a fetch in its own goroutine and hands the result back to
// the UI loop. Runs on the UI loop.
func (rt *runtime) startQuery(e *queryEntry) {
	if e.cancel != nil {
		e.cancel() // a newer fetch wins
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel, e.started, e.loading = cancel, true, true
	rt.touchQuery(e)
	fetch := e.fetch
	go func() {
		data, err := fetch(ctx)
		rt.do(func() {
			if ctx.Err() != nil {
				return // superseded
			}
			cancel()
			e.cancel, e.loading, e.err = nil, false, err
			if err == nil {
				e.data, e.loaded, e.at = data, true, time.Now()
			}
			rt.touchQuery(e)
		})
	}()
}

// touchQuery re-renders every component showing the query.
func (rt *runtime) touchQuery(e *queryEntry) {
	for in := range e.subs {
		if !in.unmounted {
			in.dirty = true
			rt.dirty = true
		}
	}
}

// Action is a UseAction's state.
type Action[A, R any] struct {
	Run     func(A) // starts the action (ignored while it runs)
	Running bool
	Err     error // the last run's error
	Result  R     // the last successful run's result
}

// UseAction runs a write — placing an order, saving a profile — off the UI
// goroutine and re-renders with its progress. onDone, if given, runs on the
// UI goroutine after a success (e.g. to RefetchQuery or close a modal).
//
//	place := ui.UseAction(c, func(ctx context.Context, o NewOrder) (Order, error) {
//		return api.Call(ctx, client, shared.CreateOrder, o)
//	}, func(Order) { ui.RefetchQuery(c, "orders"); modals.Dismiss() })
func UseAction[A, R any](c *Ctx, do func(ctx context.Context, arg A) (R, error), onDone ...func(R)) Action[A, R] {
	type state struct {
		running bool
		err     error
		result  R
	}
	st, update := Reducer(c, state{})
	rt := c.rt
	return Action[A, R]{
		Running: st.running, Err: st.err, Result: st.result,
		Run: func(arg A) {
			if st.running {
				return
			}
			update(func(s state) state { s.running, s.err = true, nil; return s })
			go func() {
				res, err := do(context.Background(), arg)
				update(func(s state) state {
					s.running, s.err = false, err
					if err == nil {
						s.result = res
					}
					return s
				})
				if err == nil {
					rt.do(func() {
						for _, f := range onDone {
							f(res)
						}
					})
				}
			}()
		},
	}
}
