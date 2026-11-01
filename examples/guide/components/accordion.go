package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// FAQ opens one answer at a time.
type FAQ struct{}

func (FAQ) Render(c *ui.Ctx) ui.Component {
	return ui.SafeArea(ui.Style{Padding: ui.All(16)},
		ui.Accordion{Single: true, Items: []ui.AccordionItem{
			{Title: "Do you deliver?", Content: ui.Text("Within 5 km of a branch.", ui.Style{})},
			{Title: "Can I cancel?", Content: ui.Text("Until we start making it.", ui.Style{})},
		}})
}
