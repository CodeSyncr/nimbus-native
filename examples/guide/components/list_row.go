package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Settings is the usual kinds of row.
type Settings struct{}

func (Settings) Render(c *ui.Ctx) ui.Component {
	alerts, setAlerts := ui.State(c, true)
	return ui.SafeArea(ui.Style{Padding: ui.All(16)},
		ui.ListRow{Title: "Asha Rao", Subtitle: "Manager", Leading: ui.Avatar{Name: "Asha Rao"}},
		ui.ListRow{Title: "Order alerts", Icon: "bell", Trailing: ui.Switch(alerts, setAlerts, ui.Style{})},
		ui.ListRow{Title: "Help", Icon: "help", OnPress: func() {}})
}
