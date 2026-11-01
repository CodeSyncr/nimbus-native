package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/network"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Online shows a banner while the phone has no connection.
type Online struct{}

func (Online) Render(c *ui.Ctx) ui.Component {
	s, setStatus := ui.State(c, network.Status{Connected: true})
	ui.Effect(c, func(ctx context.Context) func() {
		go network.Watch(ctx, setStatus)
		return nil
	}, ui.Once)
	line := "Online over " + s.Type
	if !s.Connected {
		line = "You're offline"
	}
	if s.Airplane {
		line += ", in airplane mode"
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 8},
		ui.Text(line, ui.Style{FontSize: 20}),
		ui.Text("Address: "+s.Address, ui.Style{}))
}
