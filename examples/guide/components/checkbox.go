package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Extras are choices that stack up.
type Extras struct{}

func (Extras) Render(c *ui.Ctx) ui.Component {
	shot, setShot := ui.State(c, true)
	oat, setOat := ui.State(c, false)
	return ui.SafeArea(ui.Style{Padding: ui.All(24)},
		ui.Checkbox{Label: "Extra shot", Checked: shot, OnChange: setShot},
		ui.Checkbox{Label: "Oat milk", Checked: oat, OnChange: setOat},
		ui.Checkbox{Label: "Whipped cream (sold out)", Disabled: true})
}
