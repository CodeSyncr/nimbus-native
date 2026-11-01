package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// ServiceCharge explains a term when it's tapped.
type ServiceCharge struct{}

func (ServiceCharge) Render(c *ui.Ctx) ui.Component {
	th := ui.UseTheme(c)
	return ui.SafeArea(ui.Style{Padding: ui.All(24)},
		ui.View(ui.Style{Height: ui.Pt(48)}), // room for the note above
		ui.View(ui.Style{Direction: ui.Row, Gap: 8, AlignItems: ui.AlignCenter},
			ui.Text("Service charge", ui.Style{}),
			ui.Tooltip{Text: "Shared between today's team.", Child: ui.Icon("info", 20, th.TextMuted)}))
}
