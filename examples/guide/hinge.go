package guide

import (
	"github.com/CodeSyncr/nimbus-native/plugins/hinge"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Recipe goes tabletop on a half-open foldable: the video above the fold,
// the steps below it.
type Recipe struct{}

func (Recipe) Render(c *ui.Ctx) ui.Component {
	video := ui.View(ui.Style{Grow: 1, Background: ui.Hex("#111111"), AlignItems: ui.AlignCenter, Justify: ui.JustifyCenter},
		ui.Text("How to pour latte art", ui.Style{Color: ui.Hex("#ffffff")}))
	steps := ui.Text("1. Steam the milk to 60°C\n2. Pour from high, then low", ui.Style{Padding: ui.All(24)})
	return hinge.Aware{Style: ui.Style{Grow: 1}, Content: func(f hinge.Fold) ui.Component {
		if f.HalfOpen && !f.Vertical {
			return ui.View(ui.Style{Grow: 1},
				ui.View(ui.Style{Height: ui.Pt(f.Y)}, video),
				ui.View(ui.Style{Height: ui.Pt(f.H)}), // the crease
				ui.View(ui.Style{Grow: 1}, steps))
		}
		note := "Fold the phone halfway to stand it up."
		if !f.Present {
			note = "This phone doesn't fold."
		}
		return ui.SafeArea(ui.Style{Grow: 1}, ui.View(ui.Style{Height: ui.Pt(240)}, video), steps,
			ui.Text(note, ui.Style{Padding: ui.All(24)}))
	}}
}
