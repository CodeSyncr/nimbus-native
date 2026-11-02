package guide

import (
	"github.com/CodeSyncr/nimbus-native/plugins/symbols"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// MenuIcons labels the menu with each platform's own icons.
type MenuIcons struct{}

func (MenuIcons) Render(c *ui.Ctx) ui.Component {
	row := func(ios, android, label string) ui.Component {
		return ui.View(ui.Style{Direction: ui.Row, AlignItems: ui.AlignCenter, Gap: 12},
			symbols.Symbol{IOS: ios, Android: android, Size: 28},
			ui.Text(label, ui.Style{FontSize: 17}))
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 16},
		row("cup.and.saucer.fill", "local_cafe", "Coffee"),
		row("birthday.cake", "cake", "Cakes"),
		row("leaf", "eco", "Tea"),
		row("bag", "shopping_bag", "Takeaway"))
}
