package ui

import (
	"fmt"
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

var testCart = NewStore(func() []string { return nil })

type cartBadge struct{}

func (cartBadge) Render(c *Ctx) Component {
	return Text(fmt.Sprintf("%d in cart", len(testCart.Use(c))), Style{}).TestID("badge")
}

type addButton struct{}

func (addButton) Render(c *Ctx) Component {
	return Pressable(func() {
		testCart.Update(c, func(items []string) []string { return append(append([]string(nil), items...), "tea") })
	}, Style{Height: Pt(44)}).TestID("add")
}

type shopScreen struct{}

func (shopScreen) Render(c *Ctx) Component { return View(Style{}, C(cartBadge{}), C(addButton{})) }

// A store re-renders the components using it, and each runtime (each
// browser tab on the web) has its own value.
func TestStore(t *testing.T) {
	h := start(t, shopScreen{})
	h.send(&render.Press{Tag: h.tagOf("add")})
	h.send(&render.Press{Tag: h.tagOf("add")})
	if got := h.all(func(o render.Op) bool { x, ok := o.(*render.Text); return ok && x.Text == "2 in cart" }); len(got) == 0 {
		t.Fatal("the badge did not follow the store")
	}
	h2 := start(t, shopScreen{})
	if got := h2.all(func(o render.Op) bool { x, ok := o.(*render.Text); return ok && x.Text == "0 in cart" }); len(got) == 0 {
		t.Fatal("a second runtime shared the first one's store")
	}
}
