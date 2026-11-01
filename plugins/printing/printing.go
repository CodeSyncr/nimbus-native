// Package printing prints HTML or a PDF with the system's print dialog.
//
//	nimbus native plugin add printing
//
//	err := printing.Print(ctx, printing.Options{Name: "Receipt 1042", HTML: "<h1>Thank you</h1>"})
package printing

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Options is what to print: set HTML or PDF.
type Options struct {
	Name string `json:"name,omitempty"` // the job's name, shown in the dialog
	HTML string `json:"html,omitempty"` // a page to print
	PDF  []byte `json:"pdf,omitempty"`  // or a PDF to print
}

// Print opens the dialog. On iOS it returns when the dialog closes, and fails
// with plugin.Cancelled if they back out. Android returns as it opens, and
// can't tell.
//
//nimbus:param o what to print
func Print(ctx context.Context, o Options) error {
	_, err := plugin.Call[any](ctx, "printing", "print", o)
	return err
}
