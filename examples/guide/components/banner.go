package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Offline stays in the page until it's dealt with.
type Offline struct{}

func (Offline) Render(c *ui.Ctx) ui.Component {
	shown, setShown := ui.State(c, true)
	if !shown {
		return ui.SafeArea(ui.Style{Padding: ui.All(24)}, ui.Text("All clear", ui.Style{}))
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24)},
		ui.Banner{Title: "You're offline", Message: "2 orders are waiting to go out.", Tone: ui.Caution,
			Action: "Try again", OnAction: func() { setShown(false) }, OnClose: func() { setShown(false) }})
}
