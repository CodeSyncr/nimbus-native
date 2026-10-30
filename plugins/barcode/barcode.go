// Package barcode scans QR codes and barcodes with the camera, on a screen
// of its own.
//
//	nimbus native plugin add barcode
//
//	code, err := barcode.Scan(ctx)
//	if plugin.IsCode(err, plugin.Cancelled) { return }
package barcode

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Code is what was scanned.
type Code struct {
	Value  string `json:"value"`
	Format string `json:"format"` // "qr", "ean13", "code128", ...
}

// Scan opens the scanner and returns the first code it reads. It fails with
// plugin.Cancelled if they back out, and plugin.Unavailable where there's no
// camera, as on the iOS simulator. Android uses Google Play services' scanner.
func Scan(ctx context.Context) (Code, error) {
	return plugin.Call[Code](ctx, "barcode", "scan", nil)
}
