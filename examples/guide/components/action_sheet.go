package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// OrderActions asks what to do next.
type OrderActions struct{}

func (OrderActions) Render(c *ui.Ctx) ui.Component {
	ov := ui.Overlays(c)
	done, setDone := ui.State(c, "")
	open := func() {
		ov.ActionSheet("Order 1042",
			ui.MenuAction{Label: "Mark as ready", Icon: "check", OnPress: func() { setDone("Ready") }},
			ui.MenuAction{Label: "Cancel order", Icon: "trash", Destructive: true, OnPress: func() { setDone("Cancelled") }})
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Options", OnPress: open},
		ui.Text(done, ui.Style{}))
}
