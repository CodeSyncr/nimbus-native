package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/sms"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// TextFriend fills in a text for them to check and send.
type TextFriend struct{}

func textTable(ctx context.Context, _ struct{}) (string, error) {
	status, err := sms.Compose(ctx, sms.Options{To: []string{"+912012345678"}, Body: "Table for two at 8?"})
	return string(status), err
}

func (TextFriend) Render(c *ui.Ctx) ui.Component {
	send := ui.UseAction(c, textTable)
	status := send.Result
	if send.Err != nil {
		status = send.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Text a friend", OnPress: func() { send.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
