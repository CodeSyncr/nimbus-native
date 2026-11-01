package guide

import (
	"github.com/CodeSyncr/nimbus-native/plugins/appinfo"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// AboutApp shows the version people quote when they ask for help.
type AboutApp struct{}

func (AboutApp) Render(c *ui.Ctx) ui.Component {
	info := ui.UseQuery(c, "appinfo", appinfo.Get)
	if info.Err != nil {
		return ui.SafeArea(ui.Style{Padding: ui.All(24)}, ui.Text(info.Err.Error(), ui.Style{}))
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 8},
		ui.Text("Version "+info.Data.Version+" ("+info.Data.Build+")", ui.Style{FontSize: 20}),
		ui.Text(info.Data.Platform+" "+info.Data.OSVersion, ui.Style{}),
		ui.Text("Language: "+info.Data.Locale, ui.Style{}))
}
