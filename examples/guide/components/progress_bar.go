package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Upload shows how far it has got, or just that it's busy.
type Upload struct{}

func (Upload) Render(c *ui.Ctx) ui.Component {
	done, setDone := ui.State(c, 0.25)
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 16},
		ui.ProgressBar{Value: done},
		ui.Button{Label: "Faster", OnPress: func() { setDone(min(done+0.25, 1)) }},
		ui.ProgressBar{Busy: true, Tone: ui.Good})
}
