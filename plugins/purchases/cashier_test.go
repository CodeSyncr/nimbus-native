package purchases

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func cashierServer(t *testing.T, seen *map[string]any, status int, answer string) Cashier {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer pub" {
			t.Errorf("no key on %s", r.URL.Path)
		}
		(*seen)["path"] = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(seen)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(s.Close)
	return Cashier{Key: "pub", User: "u 1", URL: s.URL}
}

func TestDeliver(t *testing.T) {
	seen := map[string]any{}
	c := cashierServer(t, &seen, 200, `{"subscriber":{"subject":"u 1","active_entitlement_ids":["pro"],"active_product_ids":["m"],"latest_expires_at":"2026-12-01T00:00:00Z"}}`)
	who, err := c.Deliver(context.Background(), Receipt{Platform: "apple", ProductID: "m", ID: "1", Token: "jws"})
	if err != nil {
		t.Fatal(err)
	}
	if !who.Entitled("pro") || who.Entitled("max") || who.Expires.IsZero() {
		t.Fatalf("customer = %+v", who)
	}
	if seen["path"] != "/api/cashier/v1/receipts" || seen["app_user_id"] != "u 1" || seen["token"] != "jws" || seen["product_id"] != "m" {
		t.Fatalf("sent %v", seen)
	}
	if v, ok := seen["renewal_info"]; !ok || v != nil {
		t.Fatalf("renewal_info = %v", v)
	}
}

func TestDeliverRefused(t *testing.T) {
	seen := map[string]any{}
	c := cashierServer(t, &seen, 422, `{"error":"invalid_receipt","message":"no"}`)
	_, err := c.Deliver(context.Background(), Receipt{Platform: "google"})
	var refused *CashierError
	if !errors.As(err, &refused) || refused.Code != "invalid_receipt" || refused.Status != 422 {
		t.Fatalf("err = %v", err)
	}
}

func TestSyncSendsReceipts(t *testing.T) {
	seen := map[string]any{}
	c := cashierServer(t, &seen, 200, `{"subscriber":{"subject":"u"}}`)
	if _, err := c.Sync(context.Background(), Receipt{Platform: "google", Token: "t", Finished: true}); err != nil {
		t.Fatal(err)
	}
	if seen["path"] != "/api/cashier/v1/subscribers/u%201/restore" && seen["path"] != "/api/cashier/v1/subscribers/u 1/restore" {
		t.Fatalf("path = %v", seen["path"])
	}
	if l, _ := seen["receipts"].([]any); len(l) != 1 {
		t.Fatalf("receipts = %v", seen["receipts"])
	}
}

func TestCashierNeedsKeyAndUser(t *testing.T) {
	if _, err := (Cashier{}).Customer(context.Background()); err == nil {
		t.Fatal("no error")
	}
}
