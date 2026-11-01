package purchases

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Cashier sends receipts to Nimbus Cashier, which checks them with the stores
// and keeps track of who's entitled to what, so you don't need a server of
// your own for it.
//
//	cashier := purchases.Cashier{Key: "cshr_pub_...", User: userID}
//	r, err := purchases.Buy(ctx, "pro_monthly")
//	who, err := cashier.Deliver(ctx, r)
//	if who.Entitled("pro") { ... }
type Cashier struct {
	Key  string // your app's public key, never the secret one
	User string // your own ID for this person
	URL  string // for a self-hosted Nimbus Cloud; empty is nimbusgo.space
}

// Customer is what Cashier knows about someone.
type Customer struct {
	User         string    `json:"subject"`
	Entitlements []string  `json:"active_entitlement_ids"`
	Products     []string  `json:"active_product_ids"`
	Expires      time.Time `json:"latest_expires_at"` // zero if nothing expires
}

// Entitled reports whether they have an entitlement now.
func (c Customer) Entitled(id string) bool {
	for _, e := range c.Entitlements {
		if e == id {
			return true
		}
	}
	return false
}

// CashierError is Cashier turning a request down. Code is its reason, such as
// "invalid_receipt".
type CashierError struct {
	Status  int
	Code    string `json:"error"`
	Message string `json:"message"`
}

func (e *CashierError) Error() string {
	return fmt.Sprintf("cashier %d %s: %s", e.Status, e.Code, e.Message)
}

// Deliver gives Cashier a receipt and finishes the purchase once it's
// accepted. If Cashier can't be reached the purchase stays unfinished, and
// Sync delivers it later.
func (c Cashier) Deliver(ctx context.Context, r Receipt) (Customer, error) {
	var a struct {
		Subscriber Customer `json:"subscriber"`
	}
	err := c.send(ctx, http.MethodPost, "receipts", map[string]any{
		"app_user_id": c.User, "platform": r.Platform, "token": r.Token,
		"renewal_info": optional(r.RenewalInfo), "product_id": r.ProductID,
	}, &a)
	var refused *CashierError
	switch {
	case err == nil:
		// If this fails the store offers it again, and Cashier takes it again.
		_ = Finish(ctx, r)
	case errors.As(err, &refused) && refused.Code == "invalid_receipt":
		// It will never be accepted, and the store would offer it forever.
		_ = Finish(ctx, r)
	}
	return a.Subscriber, err
}

// Sync tells Cashier everything this phone owns: call it when the app opens,
// and from a "Restore purchases" button with the receipts from Restore. With
// no receipts it uses Owned.
func (c Cashier) Sync(ctx context.Context, receipts ...Receipt) (Customer, error) {
	if len(receipts) == 0 {
		var err error
		if receipts, err = Owned(ctx); err != nil {
			return Customer{}, err
		}
	}
	list := make([]map[string]any, len(receipts))
	for i, r := range receipts {
		list[i] = map[string]any{"platform": r.Platform, "token": r.Token,
			"renewal_info": optional(r.RenewalInfo), "product_id": r.ProductID}
	}
	var a struct {
		Subscriber Customer `json:"subscriber"`
	}
	if err := c.send(ctx, http.MethodPost, "subscribers/"+url.PathEscape(c.User)+"/restore",
		map[string]any{"receipts": list}, &a); err != nil {
		return Customer{}, err
	}
	for _, r := range receipts {
		if !r.Finished {
			_ = Finish(ctx, r)
		}
	}
	return a.Subscriber, nil
}

// Customer asks Cashier what they're entitled to.
func (c Cashier) Customer(ctx context.Context) (Customer, error) {
	var who Customer
	err := c.send(ctx, http.MethodGet, "subscribers/"+url.PathEscape(c.User), nil, &who)
	return who, err
}

func optional(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (c Cashier) send(ctx context.Context, method, path string, body, out any) error {
	if c.Key == "" || c.User == "" {
		return errors.New("purchases: a Cashier needs a Key and a User")
	}
	base := c.URL
	if base == "" {
		base = "https://nimbusgo.space"
	}
	var in io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		in = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+"/api/cashier/v1/"+path, in)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode/100 != 2 {
		e := &CashierError{Status: res.StatusCode}
		_ = json.Unmarshal(b, e)
		return e
	}
	return json.Unmarshal(b, out)
}
