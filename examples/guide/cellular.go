package guide

import (
	"github.com/CodeSyncr/nimbus-native/plugins/cellular"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// PhoneNetwork shows what the phone will say about its network.
type PhoneNetwork struct{}

func (PhoneNetwork) Render(c *ui.Ctx) ui.Component {
	q := ui.UseQuery(c, "cellular", cellular.Get)
	if q.Err != nil {
		return ui.SafeArea(ui.Style{Padding: ui.All(24)}, ui.Text(q.Err.Error(), ui.Style{}))
	}
	n := q.Data
	carrier := n.Carrier
	if carrier == "" {
		carrier = "Not shared"
	}
	sim := "No SIM"
	if n.HasSIM {
		sim = "SIM ready"
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 8},
		ui.Text(carrier, ui.Style{FontSize: 20}),
		ui.Text(sim, ui.Style{}),
		ui.Text("Country: "+n.Country, ui.Style{}),
		ui.Text("Network code: "+n.MCC+" "+n.MNC, ui.Style{}))
}
