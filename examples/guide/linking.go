package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/linking"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// CallUs opens the phone's dialer, or says it can't.
type CallUs struct{}

func call(ctx context.Context, url string) (string, error) {
	ok, err := linking.CanOpen(ctx, url)
	if err != nil || !ok {
		return "This phone can't make calls", err
	}
	return "", linking.Open(ctx, url)
}

func (CallUs) Render(c *ui.Ctx) ui.Component {
	dial := ui.UseAction(c, call)
	status := dial.Result
	if dial.Err != nil {
		status = dial.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Call the cafe", OnPress: func() { dial.Run("tel:+912012345678") }},
		ui.Text(status, ui.Style{}))
}
