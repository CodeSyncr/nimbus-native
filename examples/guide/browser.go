package guide

import (
	"github.com/CodeSyncr/nimbus-native/plugins/browser"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// OurStory opens a page inside the app.
type OurStory struct{}

func (OurStory) Render(c *ui.Ctx) ui.Component {
	open := func() { go browser.Open(c.Context(), "https://nimbusgo.space") }
	return ui.SafeArea(ui.Style{Padding: ui.All(24)},
		ui.Button{Label: "Read our story", OnPress: open})
}
