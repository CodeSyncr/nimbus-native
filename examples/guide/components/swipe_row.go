package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Inbox reveals an action when a row is swiped left.
type Inbox struct{}

func (Inbox) Render(c *ui.Ctx) ui.Component {
	done, setDone := ui.State(c, false)
	if done {
		return ui.SafeArea(ui.Style{Padding: ui.All(24)}, ui.Text("Deleted", ui.Style{}))
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(16)},
		ui.SwipeRow{
			Content: ui.ListRow{Title: "Your flat white is ready", Icon: "bag"},
			Actions: []ui.SwipeAction{{Label: "Delete", Icon: "trash", Tone: ui.Bad, OnPress: func() { setDone(true) }}},
		})
}
