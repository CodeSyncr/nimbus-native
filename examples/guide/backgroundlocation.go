package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugins/backgroundlocation"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Trip follows a delivery, even with the app in the background.
type Trip struct{}

func (Trip) Render(c *ui.Ctx) ui.Component {
	count, setCount := ui.State(c, 0)
	seen := ui.Ref(c, 0)
	on, setOn := ui.State(c, false)
	status, setStatus := ui.State(c, "")
	ctx := context.Background()
	ui.Effect(c, func(ctx context.Context) func() {
		go backgroundlocation.Watch(ctx, func(p []backgroundlocation.Point) {
			*seen += len(p)
			setCount(*seen)
		})
		return nil
	}, ui.Once)
	toggle := func() {
		go func() {
			if on {
				backgroundlocation.Stop(ctx)
				setOn(false)
				return
			}
			if ok, err := backgroundlocation.Request(ctx); err != nil || !ok {
				setStatus("Location is off")
				return
			}
			err := backgroundlocation.Start(ctx, backgroundlocation.Options{Title: "Delivery running", Text: "Following your route", MinDistance: 25})
			if err != nil {
				setStatus(err.Error())
				return
			}
			setOn(true)
		}()
	}
	label := "Start the trip"
	if on {
		label = "End the trip"
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Text(fmt.Sprintf("%d points", count), ui.Style{FontSize: 28}),
		ui.Button{Label: label, OnPress: toggle},
		ui.Text(status, ui.Style{}))
}
