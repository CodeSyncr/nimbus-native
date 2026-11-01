package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/keyboard"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Note puts the keyboard away when they're done.
type Note struct{}

func (Note) Render(c *ui.Ctx) ui.Component {
	text, setText := ui.State(c, "")
	done := func() { go keyboard.Dismiss(context.Background()) }
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.TextInput(text, setText, ui.Style{Padding: ui.All(12), BorderWidth: 1, BorderColor: ui.Hex("#cccccc")}),
		ui.Button{Label: "Done", OnPress: done})
}
