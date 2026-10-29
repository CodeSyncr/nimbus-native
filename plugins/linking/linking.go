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
func Open(ctx context.Context, url string) error {
	_, err := plugin.Call[bool](ctx, "linking", "open", urlArgs{url})
	return err
}

// CanOpen reports whether some app on the device can open url. iOS answers
// only for schemes listed in the app's LSApplicationQueriesSchemes (and
// always for https).
func CanOpen(ctx context.Context, url string) (bool, error) {
	return plugin.Call[bool](ctx, "linking", "canOpen", urlArgs{url})
}
