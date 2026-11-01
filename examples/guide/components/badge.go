package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// OrderStates labels what state each order is in.
type OrderStates struct{}

func (OrderStates) Render(c *ui.Ctx) ui.Component {
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Badge{Label: "New", Tone: ui.Accent},
		ui.Badge{Label: "Ready", Tone: ui.Good},
		ui.Badge{Label: "Running late", Tone: ui.Caution},
		ui.Badge{Label: "Cancelled", Tone: ui.Bad},
		ui.Badge{Label: "3", Tone: ui.Bad, Solid: true})
}
