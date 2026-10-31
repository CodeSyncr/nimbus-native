// Package browser opens a web page inside the app, with the system browser's
// cookies and sign-ins, and a button back. For a page that's part of your
// screen, use webview; for leaving the app, use linking.
//
//	nimbus native plugin add browser
//
//	browser.Open(ctx, "https://example.com/terms")
package browser

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Open shows the page and returns once it's up.
func Open(ctx context.Context, url string) error {
	_, err := plugin.CallJSON(ctx, "browser", "open", map[string]string{"url": url})
	return err
}
