// Package intentlauncher starts other screens on Android: its settings pages,
// and other apps' activities. iOS has nothing like it, so there Start fails
// with plugin.Unavailable.
//
//	nimbus native plugin add intent-launcher
//
//	r, err := intentlauncher.Start(ctx, intentlauncher.Intent{Action: intentlauncher.WifiSettings})
package intentlauncher

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Settings pages, for Intent.Action.
const (
	Settings             = "android.settings.SETTINGS"                     // the Settings app
	WifiSettings         = "android.settings.WIFI_SETTINGS"                // Wi-Fi
	BluetoothSettings    = "android.settings.BLUETOOTH_SETTINGS"           // Bluetooth
	LocationSettings     = "android.settings.LOCATION_SOURCE_SETTINGS"     // location
	NotificationSettings = "android.settings.APP_NOTIFICATION_SETTINGS"    // needs the extra android.provider.extra.APP_PACKAGE
	AppDetails           = "android.settings.APPLICATION_DETAILS_SETTINGS" // needs Data: "package:your.app.id"
)

// Intent is what to start.
type Intent struct {
	Action  string            `json:"action,omitempty"`  // what to do, such as WifiSettings
	Data    string            `json:"data,omitempty"`    // a URI for it to act on
	Type    string            `json:"type,omitempty"`    // a MIME type
	Package string            `json:"package,omitempty"` // keeps it to this app
	Class   string            `json:"class,omitempty"`   // this activity in Package
	Extras  map[string]string `json:"extras,omitempty"`  // string extras
}

// Result is how it ended.
type Result struct {
	Code int    `json:"code"` // -1 is OK, 0 is cancelled; other values are the activity's own
	Data string `json:"data"` // the URI it handed back, if any
}

// Start opens the screen and returns when it closes. If nothing on the phone
// handles it, it fails with plugin.NotFound.
//
//nimbus:param i what to start
//nimbus:returns how it ended
//nimbus:platforms Android
func Start(ctx context.Context, i Intent) (Result, error) {
	return plugin.Call[Result](ctx, "intentlauncher", "start", i)
}
