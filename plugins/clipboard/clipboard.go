// Package clipboard copies and pastes text.
//
//	nimbus native plugin add clipboard
//
//	clipboard.Set(ctx, "NOVA-4821")
//	text, err := clipboard.Get(ctx)
package clipboard

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Set copies text.
func Set(ctx context.Context, text string) error {
	_, err := plugin.CallJSON(ctx, "clipboard", "set", map[string]string{"text": text})
	return err
}

// Get returns the text on the clipboard, or "". iOS may show a paste notice,
// and Android only allows it while the app is in front.
func Get(ctx context.Context) (string, error) {
	r, err := plugin.Call[struct {
		Text string `json:"text"`
	}](ctx, "clipboard", "get", nil)
	return r.Text, err
}
