// The orders server: a Nimbus app serving the shared endpoints. It owns the
// validation rules; the app shows their errors on its form.
//
//	PORT=3400 go run ./server
package main

import (
	"log"
	"sync"

	"github.com/CodeSyncr/nimbus"
	"github.com/CodeSyncr/nimbus-native/examples/fullstack/shared"
	nhttp "github.com/CodeSyncr/nimbus/http"
	"github.com/CodeSyncr/nimbus/validation"
)

// newOrderRequest is shared.NewOrder with the server's rules attached.
type newOrderRequest shared.NewOrder

func (newOrderRequest) Rules() validation.Schema {
	return validation.Schema{
		"customer": validation.String().Required().Min(2),
		"phone":    validation.String().Required().Regex(`^[0-9 ]{10,14}$`),
	}
}

func main() {
	var mu sync.Mutex
	orders := []shared.Order{
		{ID: 1, Customer: "Asha Rao", Phone: "98765 43210", Total: 1249.5, Status: "Paid"},
		{ID: 2, Customer: "Vikram Shah", Phone: "91234 56780", Total: 399, Status: "Shipped"},
		{ID: 3, Customer: "Meera Iyer", Phone: "99887 76655", Total: 2150, Status: "Delivered"},
	}

	app := nimbus.New()
	app.Router.Get(shared.ListOrders.Path, func(c *nhttp.Context) error {
		mu.Lock()
		defer mu.Unlock()
		return c.JSON(200, orders)
	})
	app.Router.Post(shared.CreateOrder.Path, func(c *nhttp.Context) error {
		var req newOrderRequest
		errs, err := validation.BindAndValidateSchema(c, &req)
		if err != nil {
			return err
		}
		if len(errs) > 0 {
			return c.ValidationErrors(errs)
		}
		mu.Lock()
		o := shared.Order{ID: len(orders) + 1, Customer: req.Customer, Phone: req.Phone, Status: "Paid"}
		orders = append([]shared.Order{o}, orders...)
		mu.Unlock()
		return c.JSON(201, o)
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
