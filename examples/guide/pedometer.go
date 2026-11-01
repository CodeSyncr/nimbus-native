package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugins/pedometer"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Steps counts a walk, from the moment they tap.
type Steps struct{}

func (Steps) Render(c *ui.Ctx) ui.Component {
	steps, setSteps := ui.State(c, 0)
	status, setStatus := ui.State(c, "")
	walk := func() {
		go func() {
			ctx := context.Background()
			ok, err := pedometer.Request(ctx)
			if err != nil {
				setStatus(err.Error()) // no sensor, for one
				return
			}
			if !ok {
				setStatus("Steps are off")
				return
			}
			setStatus("Walking")
			if err := pedometer.Watch(ctx, setSteps); err != nil {
				setStatus(err.Error())
			}
		}()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Text(fmt.Sprintf("%d steps", steps), ui.Style{FontSize: 28}),
		ui.Button{Label: "Start a walk", OnPress: walk},
		ui.Text(status, ui.Style{}))
}
