package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// RowMenu opens its actions beside the dots.
type RowMenu struct{}

func (RowMenu) Render(c *ui.Ctx) ui.Component {
	done, setDone := ui.State(c, "")
	return ui.SafeArea(ui.Style{Padding: ui.All(16)},
		ui.ListRow{Title: "Order 1042", Subtitle: done, Trailing: ui.Menu{
			Trigger: ui.Icon("more", 26, 0),
			Items: []ui.MenuAction{
				{Label: "Edit", Icon: "edit", OnPress: func() { setDone("Edited") }},
				{Label: "Delete", Icon: "trash", Destructive: true, OnPress: func() { setDone("Deleted") }},
			},
		}})
}
