// Package shared is the contract between the orders server (a Nimbus app)
// and the orders app (a Nimbus Native app): the types and endpoints both
// sides compile against. It imports only the dependency-free api package,
// so the app stays free of server code.
package shared

import "github.com/CodeSyncr/nimbus-native/api"

type Order struct {
	ID       int     `json:"id"`
	Customer string  `json:"customer"`
	Phone    string  `json:"phone"`
	Total    float64 `json:"total"`
	Status   string  `json:"status"`
}

type ListOrdersReq struct {
	Status string `json:"status,omitempty"`
}

type NewOrder struct {
	Customer string `json:"customer"`
	Phone    string `json:"phone"`
}

var (
	ListOrders  = api.GET[ListOrdersReq, []Order]("/api/orders")
	CreateOrder = api.POST[NewOrder, Order]("/api/orders")
)
