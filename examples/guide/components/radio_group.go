package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Payment picks one way to pay, with every option in view.
type Payment struct{}

func (Payment) Render(c *ui.Ctx) ui.Component {
	options := []string{"Card", "UPI", "Cash at the counter"}
	pay, setPay := ui.State(c, 0)
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.RadioGroup{Options: options, Selected: pay, OnChange: setPay},
		ui.Text("Paying by "+options[pay], ui.Style{}))
}
