package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Team shows a picture when there is one, and initials when not.
type Team struct{}

func (Team) Render(c *ui.Ctx) ui.Component {
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Direction: ui.Row, Gap: 12},
		ui.Avatar{Name: "Asha Rao"},
		ui.Avatar{Name: "Dev Mehta", Size: 56},
		ui.Avatar{Name: "Sana Khan", Size: 56, Image: "https://avatars.githubusercontent.com/u/9919?s=200"})
}
