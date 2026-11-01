// Package camera takes a photo, or picks one from the library, using the
// system's own screens. The photo comes back as JPEG bytes, so it works
// under `nimbus native dev` too, where your Go isn't on the phone.
//
//	nimbus native plugin add camera
//
//	p, err := camera.Take(ctx, camera.Options{MaxSize: 1600})
//	if plugin.IsCode(err, plugin.Cancelled) { return } // they backed out
//	filesystem.Write(ctx, filesystem.Data, "receipt.jpg", p.Data)
package camera

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Options sizes the photo.
type Options struct {
	MaxSize int     `json:"maxSize"` // its longer side, in pixels; 0 means 2000
	Quality float32 `json:"quality"` // JPEG, 0..1; 0 means 0.85
}

// Photo is a JPEG, turned the right way up.
type Photo struct {
	Data   []byte `json:"data"`   // the image, as a JPEG
	Width  int    `json:"width"`  // in pixels
	Height int    `json:"height"` // in pixels
}

// Take opens the camera. It fails with plugin.Cancelled if they back out, and
// plugin.Unavailable where there's no camera, as on the iOS simulator.
//
//nimbus:param o how big, and how good
func Take(ctx context.Context, o Options) (Photo, error) {
	return plugin.Call[Photo](ctx, "camera", "take", o)
}

// Pick opens the photo library. It asks for no permission: the system shows
// the photos and the app only gets the one chosen.
//
//nimbus:param o how big, and how good
func Pick(ctx context.Context, o Options) (Photo, error) {
	return plugin.Call[Photo](ctx, "camera", "pick", o)
}
