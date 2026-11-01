package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/biometrics"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Figures unlocks today's numbers for the manager.
type Figures struct{}

func unlock(ctx context.Context, _ struct{}) (string, error) {
	if biometrics.Available(ctx) == biometrics.None {
		return "Set up Face ID or a fingerprint first", nil
	}
	ok, err := biometrics.Confirm(ctx, "Show today's figures")
	if err != nil || !ok {
		return "Locked", err
	}
	return "Today: ₹48,200", nil
}

func (Figures) Render(c *ui.Ctx) ui.Component {
	open := ui.UseAction(c, unlock)
	status := open.Result
	if open.Err != nil {
		status = open.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Unlock", OnPress: func() { open.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
