package ui

import (
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui/render"
)

// A box with a gradient is sent again only when the gradient changes.
func TestBoxEqualGradients(t *testing.T) {
	a := render.Box{Tag: 1, GradientColors: []uint32{1, 2}, GradientStops: []float32{0, 1}, Blur: render.BlurThin}
	b := a
	b.GradientColors = []uint32{1, 2} // equal, not the same slice
	if !boxEqual(a, b) {
		t.Fatal("equal gradients compared unequal")
	}
	b.GradientColors = []uint32{1, 3}
	if boxEqual(a, b) {
		t.Fatal("a changed gradient compared equal")
	}
	c := a
	c.Corners = render.CornerTopLeft
	if boxEqual(a, c) {
		t.Fatal("changed corners compared equal")
	}
}

// A scroll view told to grow fills its space instead of growing to its
// content (which would leave nothing to scroll).
func TestScrollGrowFills(t *testing.T) {
	s := Scroll(Style{Grow: 1}).style
	if s.Basis != Pt(0) || s.Shrink != 1 {
		t.Fatalf("Grow: basis %v, shrink %v", s.Basis, s.Shrink)
	}
	if s := Scroll(Style{Grow: 1, Shrink: 2}).style; s.Basis != (Value{}) {
		t.Fatal("an explicit Shrink was overridden")
	}
	if s := Scroll(Style{}).style; s.Basis != (Value{}) || s.Shrink != 0 {
		t.Fatal("a scroll view that doesn't grow changed")
	}
}
