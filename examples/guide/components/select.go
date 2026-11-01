package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Branch opens the platform's own list of choices.
type Branch struct{}

func (Branch) Render(c *ui.Ctx) ui.Component {
	branches := []string{"Koregaon Park", "Baner", "Aundh"}
	at, setAt := ui.State(c, -1)
	line := "No branch yet"
	if at >= 0 {
		line = "Pick up at " + branches[at]
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Select{Label: "Branch", Placeholder: "Choose a branch", Options: branches, Selected: at, OnChange: setAt},
		ui.Text(line, ui.Style{}))
}
