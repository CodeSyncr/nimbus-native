// Package cellular describes the phone's mobile network. Phones say less
// about this every year, so treat all of it as a hint.
//
//	nimbus native plugin add cellular
//
//	info, err := cellular.Get(ctx)
package cellular

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Info is what the phone will say. Anything it won't is empty.
type Info struct {
	HasSIM bool `json:"hasSim"` // whether a SIM is ready
	// Carrier is the network's name. iOS 16 stopped giving it to apps.
	Carrier string `json:"carrier"`
	Country string `json:"country"` // the network's country, such as "in"
	MCC     string `json:"mcc"`     // the mobile country code
	MNC     string `json:"mnc"`     // the mobile network code
	// Generation is "2g", "3g", "4g" or "5g". It's iOS only: Android asks
	// for a permission that isn't worth having for this.
	Generation string `json:"generation"`
	Roaming    bool   `json:"roaming"` // on another network; Android only
}

// Get reads it.
func Get(ctx context.Context) (Info, error) {
	return plugin.Call[Info](ctx, "cellular", "get", nil)
}
