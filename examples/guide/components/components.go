// Package components holds the complete example on each component's guide
// page, compiled here so that they can't go stale.
package components

import "github.com/CodeSyncr/nimbus-native/ui"

// Screens are the examples, by page.
var Screens = map[string]ui.Component{
	"button":            Checkout{},
	"icon-button":       Toolbar{},
	"floating-button":   Orders{},
	"checkbox":          Extras{},
	"radio-group":       Payment{},
	"segmented-control": Sales{},
	"stepper":           Basket{},
	"select":            Branch{},
	"date-field":        Reservation{},
	"search-bar":        Menu{},
	"otp-input":         Verify{},
	"icon":              Icons{},
	"image":             Photo{},
	"avatar":            Team{},
	"badge":             OrderStates{},
	"chip":              Filters{},
	"divider":           Receipt{},
	"list-row":          Settings{},
	"progress-bar":      Upload{},
	"skeleton":          Loading{},
	"empty-state":       NoOrders{},
	"sheet":             FlatWhite{},
	"action-sheet":      OrderActions{},
	"toast":             Placed{},
	"banner":            Offline{},
	"menu":              RowMenu{},
	"tooltip":           ServiceCharge{},
	"app-bar":           Details{},
	"grid":              Categories{},
	"accordion":         FAQ{},
	"carousel":          Offers{},
	"swipe-row":         Inbox{},
	"gestures":          Tile{},
}
