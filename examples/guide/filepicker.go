package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/plugins/filepicker"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Upload takes a PDF.
type Upload struct{}

func choosePDF(ctx context.Context, _ struct{}) (string, error) {
	f, err := filepicker.Pick(ctx, filepicker.Options{Types: []string{"application/pdf"}})
	if plugin.IsCode(err, plugin.Cancelled) {
		return "", nil
	}
	return fmt.Sprintf("%s, %d KB", f.Name, len(f.Data)/1024), err
}

func (Upload) Render(c *ui.Ctx) ui.Component {
	pick := ui.UseAction(c, choosePDF)
	status := pick.Result
	if pick.Err != nil {
		status = pick.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Choose a file", OnPress: func() { pick.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
