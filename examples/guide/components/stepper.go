package components

import (
	"fmt"

	"github.com/CodeSyncr/nimbus-native/ui"
)

// Basket changes a quantity, and the price follows.
type Basket struct{}

func (Basket) Render(c *ui.Ctx) ui.Component {
	qty, setQty := ui.State(c, 1)
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Stepper{Value: qty, Min: 1, Max: 9, OnChange: setQty},
		ui.Text(fmt.Sprintf("₹%d", qty*220), ui.Style{FontSize: 20}))
}
