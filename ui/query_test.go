package ui

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// settle runs queued work until background goroutines have reported back.
func (h *host) settle() {
	for i := 0; i < 100; i++ {
		time.Sleep(5 * time.Millisecond)
		h.rt.mu.Lock()
		n := len(h.rt.queue)
		h.rt.mu.Unlock()
		if n > 0 {
			h.rt.drain()
		}
	}
}

var fetches atomic.Int32
var failNext atomic.Bool

func fetchOrders(ctx context.Context) ([]string, error) {
	n := fetches.Add(1)
	if failNext.Load() {
		return nil, errors.New("offline")
	}
	return []string{fmt.Sprint("order-", n)}, nil
}

type orderList struct{ id string }

func (o orderList) Render(c *Ctx) *Element {
	q := UseQuery(c, "orders", fetchOrders)
	label := "loading"
	switch {
	case q.Err != nil:
		label = "error: " + q.Err.Error()
	case q.Loaded:
		label = q.Data[0]
	}
	return Text(label, Style{}).TestID(o.id)
}

type twoLists struct{}

func (twoLists) Render(c *Ctx) *Element {
	return View(Style{}, C(orderList{"a"}), C(orderList{"b"}),
		Pressable(func() { RefetchQuery(c, "orders") }, Style{Height: Pt(44)}).TestID("refresh"))
}

func TestQuery(t *testing.T) {
	fetches.Store(0)
	failNext.Store(false)
	h := start(t, twoLists{})
	if x := h.lastText(h.tagOf("a")); x.Text != "loading" {
		t.Fatalf("first render: %q", x.Text)
	}
	h.settle()
	if fetches.Load() != 1 {
		t.Fatalf("two components with one key made %d requests", fetches.Load())
	}
	if h.lastText(h.tagOf("a")).Text != "order-1" || h.lastText(h.tagOf("b")).Text != "order-1" {
		t.Fatal("data not shown in both components")
	}

	h.send(&render.Press{Tag: h.tagOf("refresh")})
	h.settle()
	if fetches.Load() != 2 || h.lastText(h.tagOf("b")).Text != "order-2" {
		t.Fatalf("refetch: %d fetches, b = %q", fetches.Load(), h.lastText(h.tagOf("b")).Text)
	}

	failNext.Store(true)
	h.send(&render.Press{Tag: h.tagOf("refresh")})
	h.settle()
	if got := h.lastText(h.tagOf("a")).Text; got != "error: offline" {
		t.Fatalf("error not shown: %q", got)
	}
}

// queryThenState uses a hook after UseQuery: it must keep its place on
// every render (UseQuery used to add its Effect only on the first).
type queryThenState struct{}

func (queryThenState) Render(c *Ctx) *Element {
	q := UseQuery(c, "orders", fetchOrders)
	on, setOn := State(c, false)
	label := fmt.Sprint(q.Loaded, on)
	return View(Style{}, Text(label, Style{}).TestID("label"),
		Pressable(func() { setOn(!on) }, Style{Height: Pt(44)}).TestID("toggle"))
}

func TestQueryKeepsHookOrder(t *testing.T) {
	failNext.Store(false)
	h := start(t, queryThenState{})
	h.settle() // the data arrives: a second render
	h.send(&render.Press{Tag: h.tagOf("toggle")})
	h.settle()
	if got := h.lastText(h.tagOf("label")).Text; got != "true true" {
		t.Fatalf("after the data and a toggle: %q", got)
	}
}

var placed []string

type placeOrder struct{}

func (placeOrder) Render(c *Ctx) *Element {
	act := UseAction(c, func(ctx context.Context, name string) (string, error) {
		if name == "" {
			return "", errors.New("name required")
		}
		return "placed " + name, nil
	}, func(r string) { placed = append(placed, r) })
	status := "idle"
	switch {
	case act.Running:
		status = "running"
	case act.Err != nil:
		status = act.Err.Error()
	case act.Result != "":
		status = act.Result
	}
	return View(Style{},
		Text(status, Style{}).TestID("status"),
		Pressable(func() { act.Run("") }, Style{Height: Pt(44)}).TestID("bad"),
		Pressable(func() { act.Run("Asha") }, Style{Height: Pt(44)}).TestID("good"),
	)
}

func TestAction(t *testing.T) {
	placed = nil
	h := start(t, placeOrder{})
	h.send(&render.Press{Tag: h.tagOf("bad")})
	h.settle()
	if got := h.lastText(h.tagOf("status")).Text; got != "name required" || len(placed) != 0 {
		t.Fatalf("failed action: %q, onDone ran %d times", got, len(placed))
	}
	h.send(&render.Press{Tag: h.tagOf("good")})
	h.settle()
	if got := h.lastText(h.tagOf("status")).Text; got != "placed Asha" || len(placed) != 1 {
		t.Fatalf("action: %q, placed %v", got, placed)
	}
}
