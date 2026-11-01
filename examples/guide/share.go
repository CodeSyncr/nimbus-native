package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/share"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// SplitBill sends a message, and a receipt as a file.
type SplitBill struct{}

func (SplitBill) Render(c *ui.Ctx) ui.Component {
	ctx := context.Background()
	text := func() { go share.Share(ctx, share.Options{Text: "You owe me ₹580 for lunch"}) }
	file := func() {
		receipt := share.File{Name: "receipt.txt", Type: "text/plain", Data: []byte("Table 7, ₹1,160")}
		go share.Share(ctx, share.Options{Files: []share.File{receipt}})
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Share with Asha", OnPress: text},
		ui.Button{Label: "Share the receipt", Variant: ui.Tonal, OnPress: file})
}
