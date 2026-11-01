package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/plugins/printing"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// PrintReceipt prints a page written in HTML.
type PrintReceipt struct{}

const receipt = `<h1>Monsoon Coffee</h1><p>Order 1042, table 7</p><p><b>Total ₹1,160</b></p>`

func printReceipt(ctx context.Context, _ struct{}) (string, error) {
	err := printing.Print(ctx, printing.Options{Name: "Receipt 1042", HTML: receipt})
	if plugin.IsCode(err, plugin.Cancelled) {
		return "", nil
	}
	return "Sent to the print dialog", err
}

func (PrintReceipt) Render(c *ui.Ctx) ui.Component {
	send := ui.UseAction(c, printReceipt)
	status := send.Result
	if send.Err != nil {
		status = send.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Print the receipt", OnPress: func() { send.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
