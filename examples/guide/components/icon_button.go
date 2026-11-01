package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Toolbar is a row of actions that are only icons.
type Toolbar struct{}

func (Toolbar) Render(c *ui.Ctx) ui.Component {
	saved, setSaved := ui.State(c, false)
	heart := "heart"
	if saved {
		heart = "check"
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Direction: ui.Row, Gap: 8},
		ui.IconButton{Icon: "share", Label: "Share", OnPress: func() {}},
		ui.IconButton{Icon: heart, Label: "Save", Variant: ui.Tonal, OnPress: func() { setSaved(!saved) }},
		ui.IconButton{Icon: "plus", Label: "Add", Filled: true, OnPress: func() {}})
}
