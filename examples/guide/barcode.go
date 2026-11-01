package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/plugins/barcode"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// ScanCode reads the code on a table.
type ScanCode struct{}

func scanTable(ctx context.Context, _ struct{}) (string, error) {
	code, err := barcode.Scan(ctx)
	if plugin.IsCode(err, plugin.Cancelled) {
		return "", nil
	}
	return code.Value, err
}

func (ScanCode) Render(c *ui.Ctx) ui.Component {
	scan := ui.UseAction(c, scanTable)
	status := scan.Result
	if scan.Err != nil {
		status = scan.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Scan", OnPress: func() { scan.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
