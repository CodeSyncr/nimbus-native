package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Receipt separates its lines.
type Receipt struct{}

func (Receipt) Render(c *ui.Ctx) ui.Component {
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Text("Flat white", ui.Style{}),
		ui.Divider{},
		ui.Text("Banana bread", ui.Style{}),
		ui.Divider{Inset: 16},
		ui.Text("Total ₹400", ui.Style{FontWeight: 600}))
}
