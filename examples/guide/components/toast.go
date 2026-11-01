package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Placed confirms something without stopping them.
type Placed struct{}

func (Placed) Render(c *ui.Ctx) ui.Component {
	ov := ui.Overlays(c)
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Place order", OnPress: func() { ov.Toast("Order placed", ui.Good) }},
		ui.Button{Label: "Try a bad card", Variant: ui.Tonal, OnPress: func() { ov.Toast("Card declined", ui.Bad) }})
}
