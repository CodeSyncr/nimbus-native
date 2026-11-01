package components

import (
	"fmt"

	"github.com/CodeSyncr/nimbus-native/ui"
)

// Details has a top bar without a navigation stack.
type Details struct{}

func (Details) Render(c *ui.Ctx) ui.Component {
	back, setBack := ui.State(c, 0)
	return ui.SafeArea(ui.Style{Grow: 1},
		ui.AppBar{Title: "Order 1042", Subtitle: "Table 7", OnBack: func() { setBack(back + 1) },
			Actions: []ui.Component{ui.IconButton{Icon: "share", Label: "Share", OnPress: func() {}}}},
		ui.Text(fmt.Sprintf("Back tapped %d times", back), ui.Style{Padding: ui.All(16)}))
}
