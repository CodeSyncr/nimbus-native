// Package dialog shows the platform's own dialogs: alerts, confirmations,
// action sheets, short toasts, and date and time pickers. Each call waits
// for the user, so call them from an action or effect, not in Render:
//
//	place := ui.UseAction(c, func(ctx context.Context, o Order) (bool, error) {
//		return dialog.Confirm(ctx, "Cancel order?", "This can't be undone.", "Cancel order", true)
//	})
package dialog

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

const name = "dialog"

// Button is one choice in a dialog.
type Button struct {
	Label string `json:"label"`
	// Style is "default", "cancel" (placed apart, chosen by tapping outside
	// or Back) or "destructive" (red on iOS).
	Style string `json:"style,omitempty"`
}

// Options describe a dialog for Show.
type Options struct {
	Title   string   `json:"title"`
	Message string   `json:"message,omitempty"`
	Buttons []Button `json:"buttons"`
	// Sheet shows the buttons as an action sheet (from the bottom on iOS, a
	// list on Android) instead of an alert.
	Sheet bool `json:"sheet,omitempty"`
}

// Show shows a dialog and returns the index of the button the user chose,
// or -1 when they dismissed it without choosing.
func Show(ctx context.Context, o Options) (int, error) {
	if len(o.Buttons) == 0 {
		o.Buttons = []Button{{Label: "OK"}}
	}
	i, err := plugin.Call[int](ctx, name, "show", o)
	if plugin.IsCode(err, plugin.Cancelled) {
		return -1, nil
	}
	return i, err
}

// Alert tells the user something, with one OK button.
func Alert(ctx context.Context, title, message string) error {
	_, err := Show(ctx, Options{Title: title, Message: message})
	return err
}

// Confirm asks yes or no: it is true when the user chose confirm.
// destructive marks confirm as dangerous (red on iOS).
func Confirm(ctx context.Context, title, message, confirm string, destructive bool) (bool, error) {
	style := "default"
	if destructive {
		style = "destructive"
	}
	i, err := Show(ctx, Options{Title: title, Message: message,
		Buttons: []Button{{Label: "Cancel", Style: "cancel"}, {Label: confirm, Style: style}}})
	return i == 1, err
}

// Choose shows options as an action sheet with a Cancel button, and returns
// the index of the one chosen, or -1.
func Choose(ctx context.Context, title string, options ...string) (int, error) {
	buttons := make([]Button, 0, len(options)+1)
	for _, o := range options {
		buttons = append(buttons, Button{Label: o})
	}
	buttons = append(buttons, Button{Label: "Cancel", Style: "cancel"})
	i, err := Show(ctx, Options{Title: title, Buttons: buttons, Sheet: true})
	if i == len(options) {
		i = -1
	}
	return i, err
}

// Toast shows a short message that goes away by itself.
func Toast(ctx context.Context, message string) error {
	_, err := plugin.Call[any](ctx, name, "toast", map[string]string{"message": message})
	return err
}

// Mode is what PickDate asks for.
type Mode string

const (
	Date     Mode = "date"
	Time     Mode = "time"
	DateTime Mode = "datetime"
)

// PickDate shows the platform's date or time picker starting at initial,
// in the device's time zone. ok is false when the user cancelled.
func PickDate(ctx context.Context, mode Mode, initial time.Time) (t time.Time, ok bool, err error) {
	if initial.IsZero() {
		initial = time.Now()
	}
	ms, err := plugin.Call[int64](ctx, name, "date", map[string]any{"mode": mode, "initial": initial.UnixMilli()})
	if plugin.IsCode(err, plugin.Cancelled) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return time.UnixMilli(ms), true, nil
}
