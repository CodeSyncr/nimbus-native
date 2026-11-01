package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Categories lays its tiles out in equal columns.
type Categories struct{}

func (Categories) Render(c *ui.Ctx) ui.Component {
	th := ui.UseTheme(c)
	tiles := []ui.Component{}
	for _, name := range []string{"Coffee", "Tea", "Bakes", "Beans"} {
		tiles = append(tiles, ui.View(ui.Style{Height: ui.Pt(96), Radius: 16, Background: th.Surface, AlignItems: ui.AlignCenter, Justify: ui.JustifyCenter},
			ui.Text(name, ui.Style{})))
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(16)}, ui.Grid{Columns: 2, Gap: 12, Items: tiles})
}
