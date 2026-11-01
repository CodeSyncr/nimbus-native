// Package purchases sells products and subscriptions through the App Store
// and Google Play.
//
//	nimbus native plugin add purchases
//
//	items, err := purchases.Products(ctx, "pro_monthly", "coins_100")
//	r, err := purchases.Buy(ctx, "pro_monthly")
//	// Check r on your server, unlock what they paid for, then:
//	err = purchases.Finish(ctx, r)
//
// A purchase that isn't finished comes back from Owned, and the stores refund
// what's never finished, so nothing is lost if the app dies halfway. To let
// Cashier do the checking, use a Cashier.
package purchases

import (
	"context"
	"errors"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// ErrPending means the purchase is waiting on someone: a parent's approval or
// a cash payment. It turns up in Owned once it goes through.
var ErrPending = errors.New("purchases: waiting for approval")

// Product is something on sale, priced for this person's store.
type Product struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Price       string  `json:"price"` // ready to show: "₹499.00"
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	// Period is how often a subscription renews, such as "P1M" or "P1Y".
	// It's empty for anything else.
	Period string `json:"period"`
}

// Receipt is the store's signed proof of a purchase. Send it to your server,
// which asks the store whether it's real: never trust it on the phone.
type Receipt struct {
	Platform    string `json:"platform"` // "apple" or "google"
	ProductID   string `json:"productId"`
	ID          string `json:"id"`
	Token       string `json:"token"`
	RenewalInfo string `json:"renewalInfo,omitempty"` // Apple's, for a subscription
	Finished    bool   `json:"finished"`
}

// Products looks up what's on sale. Any ID the store doesn't know is left out.
func Products(ctx context.Context, ids ...string) ([]Product, error) {
	return plugin.Call[[]Product](ctx, "purchases", "products", map[string][]string{"ids": ids})
}

// Buy opens the store's payment sheet. If they back out the error has code
// plugin.Cancelled; if someone must approve it, the error is ErrPending.
func Buy(ctx context.Context, id string) (Receipt, error) {
	r, err := plugin.Call[struct {
		Receipt
		Pending bool `json:"pending"`
	}](ctx, "purchases", "buy", map[string]string{"id": id})
	if err == nil && r.Pending {
		return Receipt{}, ErrPending
	}
	return r.Receipt, err
}

// Finish tells the store they've got what they paid for. Call it after your
// server has accepted the receipt, not before.
func Finish(ctx context.Context, r Receipt) error {
	_, err := plugin.Call[any](ctx, "purchases", "finish", map[string]string{"id": r.ID})
	return err
}

// Consume is Finish for something used up, like coins, so that they can buy
// it again.
func Consume(ctx context.Context, r Receipt) error {
	_, err := plugin.Call[any](ctx, "purchases", "consume", map[string]string{"id": r.ID})
	return err
}

// Owned lists what they have now, and anything bought but not finished. Look
// at it when the app opens: that's how a renewal, a purchase made on another
// device, or one interrupted by a crash reaches you.
func Owned(ctx context.Context) ([]Receipt, error) {
	return plugin.Call[[]Receipt](ctx, "purchases", "owned", nil)
}

// Restore is Owned behind a "Restore purchases" button. Apple requires one,
// and asks them to sign in to the App Store.
func Restore(ctx context.Context) ([]Receipt, error) {
	return plugin.Call[[]Receipt](ctx, "purchases", "restore", nil)
}
