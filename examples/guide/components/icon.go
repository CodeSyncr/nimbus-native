package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Icons shows the built-in set, in the theme's colors.
type Icons struct{}

func (Icons) Render(c *ui.Ctx) ui.Component {
	th := ui.UseTheme(c)
	tiles := []ui.Component{}
	for _, name := range ui.IconNames() {
		tiles = append(tiles, ui.View(ui.Style{Height: ui.Pt(48), AlignItems: ui.AlignCenter, Justify: ui.JustifyCenter},
			ui.Icon(name, 24, th.Primary)))
	}
	return ui.SafeArea(ui.Style{Grow: 1},
		ui.Scroll(ui.Style{Grow: 1, Padding: ui.All(16)}, ui.Grid{Columns: 6, Items: tiles}))
}
