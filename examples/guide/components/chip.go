package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Filters toggles a choice, and a tag can be removed.
type Filters struct{}

func (Filters) Render(c *ui.Ctx) ui.Component {
	veg, setVeg := ui.State(c, true)
	tag, setTag := ui.State(c, true)
	rows := []ui.Component{ui.Chip{Label: "Vegetarian", Icon: "check", Selected: veg, OnPress: func() { setVeg(!veg) }}}
	if tag {
		rows = append(rows, ui.Chip{Label: "Koregaon Park", OnRemove: func() { setTag(false) }})
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Direction: ui.Row, Gap: 8}, rows...)
}
