// Package linking opens URLs in other apps: https in the browser, tel:,
// mailto:, maps and app links in their apps.
package linking

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

type urlArgs struct {
	URL string `json:"url"`
}

// Open opens url; it fails with plugin code "unavailable" when no app can.
//
//nimbus:param url the address, in any scheme the phone can handle
func Open(ctx context.Context, url string) error {
	_, err := plugin.Call[bool](ctx, "linking", "open", urlArgs{url})
	return err
}

// CanOpen reports whether some app on the device can open url. New apps can
// ask about https, tel, sms and mailto. For any other scheme, list it in
// LSApplicationQueriesSchemes on iOS and in <queries> in the Android manifest,
// or the answer is false.
//
//nimbus:param url the address to check
func CanOpen(ctx context.Context, url string) (bool, error) {
	return plugin.Call[bool](ctx, "linking", "canOpen", urlArgs{url})
}
