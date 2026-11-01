// Package localization says how the person likes things shown: their
// languages, region, currency, and date and number formats.
//
//	nimbus native plugin add localization
//
//	l, err := localization.Get(ctx)
//	fmt.Sprintf("%s%d", l.CurrencySymbol, price)
package localization

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Info is the phone's settings.
type Info struct {
	Languages         []string `json:"languages"`         // in their order of preference, such as "en-IN"
	Language          string   `json:"language"`          // the first, without its region, such as "en"
	Region            string   `json:"region"`            // such as "IN"
	Currency          string   `json:"currency"`          // such as "INR"; empty for a region with none
	CurrencySymbol    string   `json:"currencySymbol"`    // such as "₹"
	TimeZone          string   `json:"timeZone"`          // such as "Asia/Kolkata"
	Uses24Hour        bool     `json:"uses24Hour"`        // 15:00, not 3:00 pm
	FirstWeekday      int      `json:"firstWeekday"`      // 1 is Sunday, 2 is Monday
	Metric            bool     `json:"metric"`            // kilometres, not miles
	DecimalSeparator  string   `json:"decimalSeparator"`  // as in 1.5
	GroupingSeparator string   `json:"groupingSeparator"` // as in 1,000
	RightToLeft       bool     `json:"rightToLeft"`       // their language reads that way
}

// Get reads them. Ask again when the app comes back: people can change them
// while it's away.
func Get(ctx context.Context) (Info, error) {
	return plugin.Call[Info](ctx, "localization", "get", nil)
}
