package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/clipboard"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// GiftCode copies a code, and pastes one back.
type GiftCode struct{}

func pasteCode(ctx context.Context, _ struct{}) (string, error) {
	text, err := clipboard.Get(ctx)
	return "Pasted " + text, err
}

func (GiftCode) Render(c *ui.Ctx) ui.Component {
	copied, setCopied := ui.State(c, "")
	paste := ui.UseAction(c, pasteCode)
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		// Copies copies inside the tap, which browsers insist on.
		ui.Button{Label: "Copy code", Copies: "MONSOON-25", OnPress: func() { setCopied("Copied MONSOON-25") }},
		ui.Button{Label: "Paste a code", Variant: ui.Tonal, OnPress: func() { paste.Run(struct{}{}) }},
		ui.Text(copied, ui.Style{}),
		ui.Text(paste.Result, ui.Style{}))
}
