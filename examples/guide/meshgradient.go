package guide

import (
	"github.com/CodeSyncr/nimbus-native/plugins/meshgradient"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Sunset is a welcome screen on a mesh gradient, its middle point bent
// toward the top right.
type Sunset struct{}

func (Sunset) Render(c *ui.Ctx) ui.Component {
	teal, sky, coral, sand := ui.Hex("#0d7377"), ui.Hex("#7ec8e3"), ui.Hex("#ff7f50"), ui.Hex("#f4e1c1")
	return ui.View(ui.Style{Grow: 1},
		meshgradient.View{
			Columns: 3, Rows: 3,
			Points: []ui.Point{
				{X: 0, Y: 0}, {X: 0.5, Y: 0}, {X: 1, Y: 0},
				{X: 0, Y: 0.5}, {X: 0.62, Y: 0.4}, {X: 1, Y: 0.5},
				{X: 0, Y: 1}, {X: 0.5, Y: 1}, {X: 1, Y: 1},
			},
			Colors: []ui.Color{teal, teal, sky, coral, sand, sky, coral, coral, sand},
			Style:  ui.Style{Absolute: true, Top: ui.Pt(0), Left: ui.Pt(0), Right: ui.Pt(0), Bottom: ui.Pt(0)},
		},
		ui.SafeArea(ui.Style{Padding: ui.All(24), Justify: ui.JustifyEnd, Grow: 1},
			ui.Text("Monsoon Coffee", ui.Style{FontSize: 34, FontWeight: 700, Color: ui.Hex("#ffffff")})))
}
