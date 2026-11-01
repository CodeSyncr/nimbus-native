package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugins/brightness"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// BoardingPass turns the screen up while its code is showing, so a scanner
// can read it, and puts it back on leaving.
type BoardingPass struct{}

func raise(ctx context.Context, _ struct{}) (string, error) {
	if err := brightness.Set(ctx, 1); err != nil {
		return "", err
	}
	level, err := brightness.Get(ctx)
	return fmt.Sprintf("Brightness %.0f%%", level*100), err
}

func (BoardingPass) Render(c *ui.Ctx) ui.Component {
	up := ui.UseAction(c, raise)
	ui.Effect(c, func(ctx context.Context) func() {
		return func() { go brightness.Reset(context.Background()) }
	}, ui.Once)
	status := up.Result
	if up.Err != nil {
		status = up.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Show my code", OnPress: func() { up.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
