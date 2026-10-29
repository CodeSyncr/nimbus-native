package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type Order struct {
	ID       int    `json:"id"`
	Customer string `json:"customer"`
}

type ListReq struct {
	Status string `json:"status"`
	Page   int    `json:"page"`
}

type OrderRef struct {
	ID int `path:"id" json:"-"`
}

type NewOrder struct {
	Customer string `json:"customer"`
}

var (
	list   = GET[ListReq, []Order]("/api/orders")
	get    = GET[OrderRef, Order]("/api/orders/{id}")
	create = POST[NewOrder, Order]("/api/orders")
	remove = DELETE[OrderRef, None]("/api/orders/{id}")
)

// server answers the way a Nimbus app does, errors included.
func server(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/orders", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("status") != "paid" || r.URL.Query().Get("page") != "2" {
			t.Errorf("query %q", r.URL.RawQuery)
		}
		json.NewEncoder(w).Encode([]Order{{1, "Asha"}, {2, "Vikram"}})
	})
	mux.HandleFunc("GET /api/orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "7" || r.URL.RawQuery != "" {
			t.Errorf("path %s query %q", r.URL.Path, r.URL.RawQuery)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		json.NewEncoder(w).Encode(Order{7, "Meera"})
	})
	mux.HandleFunc("POST /api/orders", func(w http.ResponseWriter, r *http.Request) {
		var in NewOrder
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &in)
		if in.Customer == "" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]any{
				"message": "The given data was invalid.",
				"errors":  map[string][]string{"customer": {"customer is required"}},
			})
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Order{9, in.Customer})
	})
	mux.HandleFunc("DELETE /api/orders/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

func TestCalls(t *testing.T) {
	s := server(t)
	c := New(s.URL)
	ctx := context.Background()

	orders, err := Call(ctx, c, list, ListReq{Status: "paid", Page: 2})
	if err != nil || len(orders) != 2 || orders[1].Customer != "Vikram" {
		t.Fatalf("list: %v %v", orders, err)
	}

	_, err = Call(ctx, c, get, OrderRef{ID: 7})
	var e *Error
	if !errors.As(err, &e) || e.Status != 401 || e.Message != "unauthorized" {
		t.Fatalf("no token: %v", err)
	}
	c.Token = func() string { return "tok" }
	if o, err := Call(ctx, c, get, OrderRef{ID: 7}); err != nil || o.Customer != "Meera" {
		t.Fatalf("get: %v %v", o, err)
	}

	if o, err := Call(ctx, c, create, NewOrder{Customer: "Rohan"}); err != nil || o.ID != 9 {
		t.Fatalf("create: %v %v", o, err)
	}
	if _, err := Call(ctx, c, remove, OrderRef{ID: 7}); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

// A Nimbus 422 becomes field errors a form can show.
func TestValidationErrors(t *testing.T) {
	_, err := Call(context.Background(), New(server(t).URL), create, NewOrder{})
	var withMap interface{ ToMap() map[string][]string }
	if !errors.As(err, &withMap) || withMap.ToMap()["customer"][0] != "customer is required" {
		t.Fatalf("422 not read as field errors: %v", err)
	}
	if err.Error() != "api: 422 The given data was invalid." {
		t.Fatalf("message: %q", err.Error())
	}
}

func TestPathNeedsTaggedField(t *testing.T) {
	bad := GET[ListReq, Order]("/api/orders/{id}")
	if _, err := Call(context.Background(), New("http://unused"), bad, ListReq{}); err == nil {
		t.Fatal("a missing path field should be an error before any request")
	}
}
