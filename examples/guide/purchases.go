package guide

import (
	"context"
	"errors"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/plugins/purchases"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Membership sells one subscription, and hands the receipt to Cashier.
type Membership struct{}

// cashier uses your app's public key. User is your own ID for this person.
var cashier = purchases.Cashier{Key: "cshr_pub_your_key", User: "user-42"}

func loadPlan(ctx context.Context) (purchases.Product, error) {
	items, err := purchases.Products(ctx, "club_monthly")
	if err != nil || len(items) == 0 {
		return purchases.Product{}, err
	}
	return items[0], nil
}

func join(ctx context.Context, id string) (string, error) {
	r, err := purchases.Buy(ctx, id)
	switch {
	case plugin.IsCode(err, plugin.Cancelled):
		return "", nil
	case errors.Is(err, purchases.ErrPending):
		return "Waiting for approval", nil
	case err != nil:
		return "", err
	}
	who, err := cashier.Deliver(ctx, r)
	if err != nil {
		return "We'll finish this shortly", err
	}
	if who.Entitled("club") {
		return "Welcome to the club", nil
	}
	return "Not active yet", nil
}

func (Membership) Render(c *ui.Ctx) ui.Component {
	plan := ui.UseQuery(c, "plan", loadPlan)
	buy := ui.UseAction(c, join)
	status := buy.Result
	if plan.Err != nil {
		status = plan.Err.Error()
	}
	if buy.Err != nil {
		status = buy.Err.Error()
	}
	if plan.Data.ID == "" {
		return ui.SafeArea(ui.Style{Padding: ui.All(24)}, ui.Text(status, ui.Style{}))
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Text(plan.Data.Title, ui.Style{FontSize: 20}),
		ui.Button{Label: "Join for " + plan.Data.Price, Loading: buy.Running, OnPress: func() { buy.Run(plan.Data.ID) }},
		ui.Text(status, ui.Style{}))
}
