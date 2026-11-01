package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugins/device"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// AboutPhone is what you'd attach to a support ticket.
type AboutPhone struct{}

type phone struct {
	info    device.Info
	battery device.Battery
}

func loadPhone(ctx context.Context) (phone, error) {
	info, err := device.Get(ctx)
	if err != nil {
		return phone{}, err
	}
	battery, err := device.GetBattery(ctx)
	return phone{info, battery}, err
}

func (AboutPhone) Render(c *ui.Ctx) ui.Component {
	q := ui.UseQuery(c, "phone", loadPhone)
	live, setLive := ui.State(c, device.Battery{})
	ui.Effect(c, func(ctx context.Context) func() {
		go device.WatchBattery(ctx, setLive)
		return nil
	}, ui.Once)
	if q.Err != nil {
		return ui.SafeArea(ui.Style{Padding: ui.All(24)}, ui.Text(q.Err.Error(), ui.Style{}))
	}
	p := q.Data
	if live == (device.Battery{}) {
		live = p.battery
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 8},
		ui.Text(p.info.Manufacturer+" "+p.info.Model, ui.Style{FontSize: 20}),
		ui.Text(p.info.Platform+" "+p.info.OSVersion, ui.Style{}),
		ui.Text(fmt.Sprintf("%d GB of memory", p.info.Memory>>30), ui.Style{}),
		ui.Text(fmt.Sprintf("Battery %.0f%%, charging: %v, low power: %v", live.Level*100, live.Charging, live.LowPower), ui.Style{}))
}
