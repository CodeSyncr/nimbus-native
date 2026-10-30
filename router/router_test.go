package router

import (
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui"
)

type home struct{}

func (home) Render(*ui.Ctx) ui.Component { return nil }

type order struct{ ID int }

func (order) Render(*ui.Ctx) ui.Component { return nil }
func (o order) Title() string             { return "Order" }

type newOrder struct{}

func (newOrder) Render(*ui.Ctx) ui.Component { return nil }

type list struct{ Tab string }

func (list) Render(*ui.Ctx) ui.Component { return nil }

func table() Table {
	return Table{
		{"/orders/:id", func(p Params) (ui.Component, bool) {
			id, ok := p.Int("id")
			return order{id}, ok
		}},
		{"/", func(Params) (ui.Component, bool) { return home{}, true }},
		{"/orders", func(p Params) (ui.Component, bool) { return list{p.Get("tab")}, true }},
		{"/orders/new", func(Params) (ui.Component, bool) { return newOrder{}, true }},
	}
}

func TestMatch(t *testing.T) {
	Register(table())
	for path, want := range map[string]ui.Component{
		"/":                home{},
		"":                 home{},
		"/orders":          list{},
		"/orders/":         list{},
		"/orders?tab=paid": list{"paid"},
		"/orders/42":       order{42},
		"/orders/new":      newOrder{}, // static beats :id, whatever the table order
		"/orders//7":       order{7},
	} {
		got, _, ok := Match(path)
		if !ok || got != want {
			t.Errorf("Match(%q) = %#v, %v; want %#v", path, got, ok, want)
		}
	}
	for _, path := range []string{"/orders/abc", "/missing", "/orders/1/items"} {
		if c, _, ok := Match(path); ok {
			t.Errorf("Match(%q) = %#v, want no match", path, c)
		}
	}
}

func TestTitleOf(t *testing.T) {
	cases := []struct {
		comp       ui.Component
		path, want string
	}{
		{order{1}, "/orders/1", "Order"},
		{list{}, "/orders?tab=x", "Orders"},
		{newOrder{}, "/orders/new", "New"},
		{list{}, "/order-history/7", "Order history"},
		{home{}, "/", ""},
	}
	for _, c := range cases {
		if got := TitleOf(c.comp, c.path); got != c.want {
			t.Errorf("TitleOf(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func BenchmarkMatch(b *testing.B) {
	Register(table())
	for i := 0; i < b.N; i++ {
		Match("/orders/42")
	}
}
