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
