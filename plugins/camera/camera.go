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

// Video is a video from the library, as it was recorded.
type Video struct {
	Data    []byte  `json:"data"`    // the file
	Type    string  `json:"type"`    // its MIME type, such as "video/mp4"
	Seconds float64 `json:"seconds"` // how long it runs
}

// VideoOptions limits a video.
type VideoOptions struct {
	MaxBytes int64 `json:"maxBytes"` // larger files fail; 0 means 50 MB
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

// PickMany opens the photo library to choose several. Like Pick, it asks for
// no permission. Photos come back in the order they were chosen.
//
//nimbus:param o how big, and how good
//nimbus:param limit the most they can choose, from 2 up; 0 or less is as many as the system allows
//nimbus:returns the photos
func PickMany(ctx context.Context, o Options, limit int) ([]Photo, error) {
	return plugin.Call[[]Photo](ctx, "camera", "pickMany", struct {
		Options
		Limit int `json:"limit"`
	}{o, max(limit, 0)})
}

// PickVideo opens the library to choose a video. It comes back whole, through
// memory, so keep MaxBytes modest, and use a server upload for anything long.
//
//nimbus:param o the largest file to accept
//nimbus:returns the video
func PickVideo(ctx context.Context, o VideoOptions) (Video, error) {
	if o.MaxBytes <= 0 {
		o.MaxBytes = 50 << 20
	}
	return plugin.Call[Video](ctx, "camera", "pickVideo", o)
}
