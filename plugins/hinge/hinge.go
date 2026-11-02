// Package hinge tells a screen about a foldable phone's fold: where it
// runs, and whether the phone is half open, like a laptop or a tent, so a
// layout can keep things off the crease. Phones without a fold, and every
// iPhone and iPad, report none.
//
//	nimbus native plugin add hinge
//
//	hinge.Aware{Style: ui.Style{Grow: 1}, Content: func(f hinge.Fold) ui.Component {
//		if f.HalfOpen && !f.Vertical {
//			return tabletop(f) // video above the fold, controls below
//		}
//		return normal()
//	}}
//
// splitview uses it to split at a vertical fold.
package hinge

import (
	"encoding/json"

	"github.com/CodeSyncr/nimbus-native/ui"
)

// Fold is a fold, or none.
type Fold struct {
	Present   bool `json:"present"`   // there is one on this screen
	HalfOpen  bool `json:"halfOpen"`  // folded partway, not flat
	Vertical  bool `json:"vertical"`  // it runs top to bottom, splitting left from right
	Separates bool `json:"separates"` // it divides the screen in two: half open, or two screens with a gap
	Occludes  bool `json:"occludes"`  // it hides what's under it, like a hinge between two screens
	// X, Y, W and H are where it is, in points from the window's top left.
	// A fold that hides nothing is a line, so W or H is 0.
	X float32 `json:"x"`
	Y float32 `json:"y"` // its top
	W float32 `json:"w"` // its width: 0 for a vertical crease that hides nothing
	H float32 `json:"h"` // its height: 0 for a horizontal crease that hides nothing
}

// Aware renders Content with the fold, again whenever it changes. Put it at
// the root of a screen that fills the window, since the fold's place is the
// window's.
type Aware struct {
	Content func(Fold) ui.Component // the screen, for the fold as it is now
	Style   ui.Style                // its size and place; usually Grow: 1
}

// Render watches the fold and renders Content.
func (a Aware) Render(c *ui.Ctx) ui.Component {
	fold, setFold := ui.State(c, Fold{})
	probe := ui.Native("hinge", struct{}{}, ui.Style{Absolute: true, Width: ui.Pt(0), Height: ui.Pt(0)}).OnEvent(func(name, data string) {
		var f Fold
		if name == "fold" && json.Unmarshal([]byte(data), &f) == nil {
			setFold(f)
		}
	})
	var content ui.Component = ui.View(ui.Style{})
	if a.Content != nil {
		content = a.Content(fold)
	}
	return ui.View(a.Style, probe, content)
}
