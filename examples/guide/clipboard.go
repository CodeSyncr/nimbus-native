package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/clipboard"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// GiftCode copies a code, and pastes one back.
type GiftCode struct{}

func copyCode(ctx context.Context, code string) (string, error) {
	return "Copied " + code, clipboard.Set(ctx, code)
}

func pasteCode(ctx context.Context, _ struct{}) (string, error) {
	text, err := clipboard.Get(ctx)
	return "Pasted " + text, err
}

func (GiftCode) Render(c *ui.Ctx) ui.Component {
	cp, paste := ui.UseAction(c, copyCode), ui.UseAction(c, pasteCode)
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Copy code", OnPress: func() { cp.Run("MONSOON-25") }},
		ui.Button{Label: "Paste a code", Variant: ui.Tonal, OnPress: func() { paste.Run(struct{}{}) }},
		ui.Text(cp.Result, ui.Style{}),
		ui.Text(paste.Result, ui.Style{}))
}
