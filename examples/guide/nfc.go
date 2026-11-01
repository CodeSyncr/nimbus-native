package guide

import (
	"context"
	"strings"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/plugins/nfc"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// ReadTag collects a loyalty stamp from a tag on the counter.
type ReadTag struct{}

func readTag(ctx context.Context, _ struct{}) (string, error) {
	if !nfc.Available(ctx) {
		return "This phone can't read tags", nil
	}
	t, err := nfc.Read(ctx, "Hold your phone to the tag")
	if plugin.IsCode(err, plugin.Cancelled) {
		return "", nil
	}
	return t.ID + " " + strings.Join(t.Records, ", "), err
}

func (ReadTag) Render(c *ui.Ctx) ui.Component {
	read := ui.UseAction(c, readTag)
	status := read.Result
	if read.Err != nil {
		status = read.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Scan a tag", OnPress: func() { read.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
