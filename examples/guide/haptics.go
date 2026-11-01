package guide

import (
	"github.com/CodeSyncr/nimbus-native/plugins/haptics"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Feedback plays each kind. Haptics return at once, so handlers call them
// directly.
type Feedback struct{}

func (Feedback) Render(c *ui.Ctx) ui.Component {
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "A tap", OnPress: func() { haptics.Impact(haptics.Medium, 1) }},
		ui.Button{Label: "It worked", OnPress: func() { haptics.Notify(haptics.Success) }},
		ui.Button{Label: "A choice changed", OnPress: haptics.Selection})
}
