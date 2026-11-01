package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Sales switches between views of the same figures.
type Sales struct{}

func (Sales) Render(c *ui.Ctx) ui.Component {
	periods := []string{"Day", "Week", "Month"}
	figures := []string{"₹48,200", "₹3,12,000", "₹13,40,000"}
	at, setAt := ui.State(c, 1)
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 16},
		ui.SegmentedControl{Options: periods, Selected: at, OnChange: setAt},
		ui.Text(figures[at], ui.Style{FontSize: 28}))
}
