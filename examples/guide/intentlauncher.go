package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/plugins/intentlauncher"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// FixWifi opens the phone's Wi-Fi settings, and carries on when they return.
type FixWifi struct{}

func openWifi(ctx context.Context, _ struct{}) (string, error) {
	r, err := intentlauncher.Start(ctx, intentlauncher.Intent{Action: intentlauncher.WifiSettings})
	if plugin.IsCode(err, plugin.Unavailable) {
		return "Open Settings on your phone", nil // iOS
	}
	return fmt.Sprintf("Back, with code %d", r.Code), err
}

func (FixWifi) Render(c *ui.Ctx) ui.Component {
	open := ui.UseAction(c, openWifi)
	status := open.Result
	if open.Err != nil {
		status = open.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Wi-Fi settings", OnPress: func() { open.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
