// Package filepicker lets someone choose a file with the system's picker.
// The file comes back as bytes, so it works under `nimbus native dev` too.
//
//	nimbus native plugin add file-picker
//
//	f, err := filepicker.Pick(ctx, filepicker.Options{Types: []string{"application/pdf"}})
//	if plugin.IsCode(err, plugin.Cancelled) { return }
package filepicker

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Options narrows what can be chosen.
type Options struct {
	Types    []string `json:"types"`    // MIME types, such as "application/pdf"; empty for any
	MaxBytes int64    `json:"maxBytes"` // larger files fail; 0 means 20 MB
}

// File is what was chosen.
type File struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Data []byte `json:"data"`
}

// Pick opens the picker. It fails with plugin.Cancelled if they back out.
func Pick(ctx context.Context, o Options) (File, error) {
	return plugin.Call[File](ctx, "filepicker", "pick", o)
}
