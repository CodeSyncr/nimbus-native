package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/push"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Notifications asks, gets a token for your server, and shows what arrives.
type Notifications struct{}

func (Notifications) Render(c *ui.Ctx) ui.Component {
	status, setStatus := ui.State(c, "Tap to turn on")
	last, setLast := ui.State(c, "")
	ui.Effect(c, func(ctx context.Context) func() {
		go push.Watch(ctx, func(m push.Message) { setLast(m.Title + ": " + m.Body) })
		return nil
	}, ui.Once)
	turnOn := func() {
		go func() {
			ctx := context.Background()
			if ok, err := push.Request(ctx); err != nil || !ok {
				setStatus("Notifications are off")
				return
			}
			t, err := push.GetToken(ctx)
			if err != nil {
				setStatus(err.Error())
				return
			}
			// Send t.Value and t.Service to your server here.
			setStatus("On, through " + t.Service)
		}()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Turn on", OnPress: turnOn},
		ui.Text(status, ui.Style{}),
		ui.Text(last, ui.Style{}))
}
