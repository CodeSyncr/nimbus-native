package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugins/geolocation"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// WhereAmI finds the phone once.
type WhereAmI struct{}

func locate(ctx context.Context, _ struct{}) (string, error) {
	ok, err := geolocation.Request(ctx)
	if err != nil || !ok {
		return "Location is off", err
	}
	p, err := geolocation.Current(ctx)
	return fmt.Sprintf("%.4f, %.4f", p.Latitude, p.Longitude), err
}

func (WhereAmI) Render(c *ui.Ctx) ui.Component {
	find := ui.UseAction(c, locate)
	status := find.Result
	if find.Err != nil {
		status = find.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Find me", OnPress: func() { find.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
