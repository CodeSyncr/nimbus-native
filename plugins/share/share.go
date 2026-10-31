// Package share opens the system's share sheet.
//
//	nimbus native plugin add share
//
//	share.Share(ctx, share.Options{Text: "Table for two at 8", URL: "https://example.com/b/42"})
//	share.Share(ctx, share.Options{Files: []share.File{{Name: "receipt.pdf", Type: "application/pdf", Data: pdf}}})
package share

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Options is what to share; set at least one.
type Options struct {
	Title string `json:"title,omitempty"` // Android's chooser title, and a subject in mail
	Text  string `json:"text,omitempty"`
	URL   string `json:"url,omitempty"`
	Files []File `json:"files,omitempty"`
}

// File is a file to share, as bytes.
type File struct {
	Name string `json:"name"`
	Type string `json:"type"` // a MIME type, such as "image/jpeg"
	Data []byte `json:"data"`
}

// Share opens the sheet and returns once it's up.
func Share(ctx context.Context, o Options) error {
	_, err := plugin.CallJSON(ctx, "share", "share", o)
	return err
}
