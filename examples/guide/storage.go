package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugins/storage"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Counter remembers a number between launches.
type Counter struct{}

func bump(ctx context.Context, _ struct{}) (int, error) {
	var visits int
	if _, err := storage.Get(ctx, "visits", &visits); err != nil {
		return 0, err
	}
	visits++
	return visits, storage.Set(ctx, "visits", visits)
}

func (Counter) Render(c *ui.Ctx) ui.Component {
	count := ui.UseAction(c, bump)
	status := fmt.Sprintf("Visits: %d", count.Result)
	if count.Err != nil {
		status = count.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Another visit", OnPress: func() { count.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
