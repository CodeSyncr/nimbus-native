package guide

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugins/bluetooth"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Nearby lists what's around for ten seconds.
type Nearby struct{}

func (Nearby) Render(c *ui.Ctx) ui.Component {
	found, setFound := ui.Reducer(c, []bluetooth.Device(nil))
	status, setStatus := ui.State(c, "")
	scan := func() {
		go func() {
			ctx, stop := context.WithTimeout(c.Context(), 10*time.Second)
			defer stop()
			if ok, err := bluetooth.Request(ctx); err != nil || !ok {
				setStatus("Bluetooth is off")
				return
			}
			setStatus("Looking…")
			bluetooth.Scan(ctx, func(d bluetooth.Device) {
				setFound(func(l []bluetooth.Device) []bluetooth.Device { return append(l, d) })
			})
			setStatus("Done")
		}()
	}
	rows := []ui.Component{ui.Button{Label: "Scan", OnPress: scan}, ui.Text(status, ui.Style{})}
	for _, d := range found {
		rows = append(rows, ui.Text(d.Name, ui.Style{}))
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12}, rows...)
}
