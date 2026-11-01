package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Loading holds the shape of a row until its data arrives.
type Loading struct{}

func (Loading) Render(c *ui.Ctx) ui.Component {
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Direction: ui.Row, Gap: 12},
		ui.Skeleton{Height: 48, Circle: true},
		ui.View(ui.Style{Grow: 1, Gap: 8},
			ui.Skeleton{Width: ui.Pt(180)},
			ui.Skeleton{Height: 12}))
}
