package bridge

import (
	"sync"
	"testing"
)

func TestHandles(t *testing.T) {
	var h Handles[string]
	a, b := h.Put("a"), h.Put("b")
	if a == 0 || b == 0 || a == b {
		t.Fatalf("bad handles %d %d", a, b)
	}
	if v, ok := h.Get(a); !ok || v != "a" {
		t.Fatalf("Get(a) = %q, %v", v, ok)
	}
	h.Release(a)
	h.Release(a)
	if _, ok := h.Get(a); ok {
		t.Fatal("released handle still resolves")
	}
	if c := h.Put("c"); c != a {
		t.Fatalf("released handle not reused: got %d, want %d", c, a)
	}
	if _, ok := h.Get(0); ok {
		t.Fatal("handle 0 resolves")
	}
	if h.Len() != 2 {
		t.Fatalf("Len = %d, want 2", h.Len())
	}
}

func TestHandlesConcurrent(t *testing.T) {
	var h Handles[int]
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10_000; i++ {
				id := h.Put(i)
				if v, ok := h.Get(id); !ok || v != i {
					t.Errorf("Get(%d) = %d, %v; want %d", id, v, ok, i)
					return
				}
				h.Release(id)
			}
		}()
	}
	wg.Wait()
	if h.Len() != 0 {
		t.Fatalf("leaked %d handles", h.Len())
	}
}
