// Package cameraview puts a live camera inside your screen, with your own
// buttons over it. For a photo with no design of your own, the camera plugin
// is simpler: it opens the system's camera and needs no permission.
//
//	nimbus native plugin add camera-view
//
//	if ok, _ := cameraview.Request(ctx); ok {
//		setShow(true)
//	}
//
//	cameraview.View{Style: ui.Style{Grow: 1}, OnReady: ready}
//
//	photo, err := cameraview.Take(ctx, cameraview.Options{MaxSize: 1600})
package cameraview

import (
	"context"
	"encoding/json"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// View is the live picture. Show one at a time.
type View struct {
	Front   bool              // the camera on the screen's side
	Torch   bool              // the light; back camera only
	OnReady func()            // the picture has started
	OnError func(code string) // plugin.Denied or plugin.Unavailable: no permission, or no camera
	Style   ui.Style          // its size and place on the screen
}

// Render shows the camera.
func (v View) Render(c *ui.Ctx) ui.Component {
	p := map[string]any{"front": v.Front, "torch": v.Torch}
	return ui.Native("cameraview", p, v.Style).OnEvent(func(name, data string) {
		switch {
		case name == "ready" && v.OnReady != nil:
			v.OnReady()
		case name == "error" && v.OnError != nil:
			var e struct {
				Code string `json:"code"`
			}
			if json.Unmarshal([]byte(data), &e) == nil {
				v.OnError(e.Code)
			}
		}
	})
}

// Options say how to take a photo.
type Options struct {
	MaxSize int     `json:"maxSize"` // its longer side, in pixels; 0 keeps it full size
	Quality float32 `json:"quality"` // JPEG, 0 to 1; 0 means 0.85
}

// Photo is a JPEG, turned the right way up.
type Photo struct {
	Data   []byte `json:"data"`   // the JPEG
	Width  int    `json:"width"`  // in pixels
	Height int    `json:"height"` // in pixels
}

// Request asks to use the camera, once.
func Request(ctx context.Context) (bool, error) {
	r, err := plugin.Call[struct {
		Granted bool `json:"granted"`
	}](ctx, "cameraview", "request", nil)
	return r.Granted, err
}

// Take takes a photo with the View on screen. With no View, or before
// OnReady, it fails with plugin.Unavailable.
//
//nimbus:param o how big, and how good
//nimbus:returns the photo
func Take(ctx context.Context, o Options) (Photo, error) {
	return plugin.Call[Photo](ctx, "cameraview", "take", o)
}
