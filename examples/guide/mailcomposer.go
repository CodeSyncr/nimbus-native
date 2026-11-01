package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/mailcomposer"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// SendReceipt fills in an email with a receipt attached.
type SendReceipt struct{}

func emailReceipt(ctx context.Context, _ struct{}) (string, error) {
	status, err := mailcomposer.Compose(ctx, mailcomposer.Options{
		To:      []string{"accounts@example.com"},
		Subject: "Receipt for order 1042",
		Body:    "Here's my receipt.",
		Files:   []mailcomposer.File{{Name: "receipt.txt", Type: "text/plain", Data: []byte("Table 7, ₹1,160")}},
	})
	return string(status), err
}

func askForHelp(ctx context.Context, _ struct{}) (string, error) {
	status, err := mailcomposer.Compose(ctx, mailcomposer.Options{To: []string{"help@example.com"}, Subject: "Help"})
	return string(status), err
}

func (SendReceipt) Render(c *ui.Ctx) ui.Component {
	send, help := ui.UseAction(c, emailReceipt), ui.UseAction(c, askForHelp)
	status := send.Result + help.Result
	if send.Err != nil {
		status = send.Err.Error()
	}
	if help.Err != nil {
		status = help.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Email my receipt", OnPress: func() { send.Run(struct{}{}) }},
		ui.Button{Label: "Ask for help", Variant: ui.Tonal, OnPress: func() { help.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
