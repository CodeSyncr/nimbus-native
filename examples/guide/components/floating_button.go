package components

import (
	"fmt"

	"github.com/CodeSyncr/nimbus-native/ui"
)

// Orders has one main action, floating over the list.
type Orders struct{}

func (Orders) Render(c *ui.Ctx) ui.Component {
	count, setCount := ui.State(c, 2)
	rows := []ui.Component{}
	for i := 0; i < count; i++ {
		rows = append(rows, ui.ListRow{Title: fmt.Sprintf("Order %d", 1040+i), Icon: "cart"})
	}
	return ui.SafeArea(ui.Style{Grow: 1, Padding: ui.All(16)},
		ui.View(ui.Style{Grow: 1, Gap: 4}, rows...),
		ui.FloatingButton{Icon: "plus", Label: "New order", Extended: true, OnPress: func() { setCount(count + 1) }})
}
