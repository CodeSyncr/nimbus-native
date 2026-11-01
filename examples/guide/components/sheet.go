package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// FlatWhite opens a panel with its own component inside.
type FlatWhite struct{}

type sheetBody struct{}

func (sheetBody) Render(c *ui.Ctx) ui.Component {
	ov := ui.Overlays(c)
	return ui.View(ui.Style{Padding: ui.All(16), Gap: 12},
		ui.Text("Regular, ₹220", ui.Style{}),
		ui.Button{Label: "Add to order", Full: true, OnPress: ov.Dismiss})
}

func (FlatWhite) Render(c *ui.Ctx) ui.Component {
	ov := ui.Overlays(c)
	open := func() {
		ov.Sheet(ui.Sheet{Title: "Flat white", Height: 0.4, Content: sheetBody{}})
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24)},
		ui.Button{Label: "Choose a size", OnPress: open})
}
