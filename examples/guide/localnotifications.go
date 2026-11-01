package guide

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugins/localnotifications"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Remind sets a reminder for ten seconds from now.
type Remind struct{}

func remind(ctx context.Context, _ struct{}) (string, error) {
	ok, err := localnotifications.Request(ctx)
	if err != nil || !ok {
		return "Notifications are off", err
	}
	return "Leave the app and wait", localnotifications.Schedule(ctx, localnotifications.Notification{
		ID: "pickup", Title: "Your order", Body: "It's ready at the counter.", After: 10 * time.Second,
	})
}

func (Remind) Render(c *ui.Ctx) ui.Component {
	set := ui.UseAction(c, remind)
	status := set.Result
	if set.Err != nil {
		status = set.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Remind me", OnPress: func() { set.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
