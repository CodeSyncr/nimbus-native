package ui

import (
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

func TestLinkPath(t *testing.T) {
	for in, want := range map[string]string{
		"myapp://orders/42":             "/orders/42",
		"myapp://orders/42?tab=items":   "/orders/42?tab=items",
		"myapp://":                      "/",
		"https://shop.example/orders/7": "/orders/7",
		"myapp:///about":                "/about",
	} {
		if got := LinkPath(in); got != want {
			t.Errorf("LinkPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// A link pushes its page on the visible stack, or shows it as a sheet.
func TestOpenLink(t *testing.T) {
	RestoreRoute = func(path string) (string, Component, bool) {
		return "Order", detailScreen{id: 7}, path == "/orders/7"
	}
	defer func() { RestoreRoute = nil }()

	h := start(t, twoTabs{})
	h.send(&render.Selected{Tag: h.tagOf("tabs"), Index: 1})
	h.send(&render.Open{URL: "myapp://orders/7"})
	if h.tagOf("detail") == 0 {
		t.Fatal("the link's page was not pushed on the Orders tab's stack")
	}

	h2 := start(t, homeScreen{}) // no stack: a sheet
	h2.send(&render.Open{URL: "myapp://orders/7"})
	if len(h2.rt.modals) != 1 {
		t.Fatal("the link's page was not presented")
	}
	h2.send(&render.Open{URL: "myapp://nowhere"})
	if len(h2.rt.modals) != 1 {
		t.Fatal("an unknown link opened something")
	}
}
