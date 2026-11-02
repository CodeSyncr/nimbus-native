// Package livephoto picks Live Photos and plays them inside your screen, on
// iOS. Android has no Live Photos: there Pick fails with plugin.Unavailable
// and View shows its Fallback.
//
//	nimbus native plugin add live-photo
//
//	p, err := livephoto.Pick(ctx)
//	if plugin.IsCode(err, plugin.Cancelled) { return }
//
//	livephoto.View{ID: p.ID, Style: ui.Style{Width: ui.Pt(320), Height: ui.Pt(320 / p.Aspect())}}
//
// The picker hands the photo over itself, so it needs no photo library
// permission. A picked photo lasts until the app closes; Files gives its
// still and its video, to keep or upload.
package livephoto

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Photo is a Live Photo the app holds.
type Photo struct {
	ID     string `json:"id"`     // for View and Files
	Width  int    `json:"width"`  // in pixels
	Height int    `json:"height"` // in pixels
}

// Aspect is its width over its height, for sizing a View; 1 when unknown.
func (p Photo) Aspect() float32 {
	if p.Width <= 0 || p.Height <= 0 {
		return 1
	}
	return float32(p.Width) / float32(p.Height)
}

// Pick shows the system's photo picker, with only Live Photos. Backing out
// fails with plugin.Cancelled.
//
//nimbus:returns the photo, for View and Files
func Pick(ctx context.Context) (Photo, error) {
	return plugin.Call[Photo](ctx, "livephoto", "pick", nil)
}

// Media is a Live Photo's two halves.
type Media struct {
	Photo     []byte `json:"photo"`     // the still, usually HEIC
	PhotoType string `json:"photoType"` // its media type
	Video     []byte `json:"video"`     // the moment around it, a QuickTime movie
	VideoType string `json:"videoType"` // its media type
}

// Files gives a picked photo's still and video, to keep or upload.
//
//nimbus:param id Photo.ID from Pick
func Files(ctx context.Context, id string) (Media, error) {
	return plugin.Call[Media](ctx, "livephoto", "files", map[string]string{"id": id})
}

// Forget lets go of a picked photo's memory before the app closes.
//
//nimbus:param id Photo.ID from Pick
func Forget(ctx context.Context, id string) error {
	_, err := plugin.Call[any](ctx, "livephoto", "forget", map[string]string{"id": id})
	return err
}

// View shows a picked Live Photo. Touching and holding it plays it, as in
// Photos.
type View struct {
	ID    string // from Pick
	Play  int    // raise it to play the photo once more, sound and all
	Muted bool   // plays without sound
	// Fill crops it to fill the frame; otherwise it fits inside.
	Fill  bool
	Style ui.Style // its size and place on the screen
	// Fallback is shown instead on Android. Nil leaves an empty space.
	Fallback ui.Component
}

// Render shows the photo, or Fallback on Android.
func (v View) Render(c *ui.Ctx) ui.Component {
	if ui.Viewport(c).Platform == "android" {
		if v.Fallback == nil {
			return ui.View(v.Style)
		}
		return ui.View(v.Style, v.Fallback)
	}
	return ui.Native("livephoto", map[string]any{"id": v.ID, "play": v.Play, "muted": v.Muted, "fill": v.Fill}, v.Style)
}
