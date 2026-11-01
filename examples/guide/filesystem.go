package guide

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugins/filesystem"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Notes saves a file on each tap and lists what's there.
type Notes struct{}

func saveNote(ctx context.Context, _ struct{}) (string, error) {
	name := fmt.Sprintf("notes/%d.txt", time.Now().UnixMilli())
	if err := filesystem.Write(ctx, filesystem.Data, name, []byte("No sugar")); err != nil {
		return "", err
	}
	names, err := filesystem.List(ctx, filesystem.Data, "notes")
	return strings.Join(names, "\n"), err
}

func (Notes) Render(c *ui.Ctx) ui.Component {
	save := ui.UseAction(c, saveNote)
	status := save.Result
	if save.Err != nil {
		status = save.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Save a note", OnPress: func() { save.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
