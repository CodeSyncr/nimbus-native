// Package medialibrary saves to and reads from the phone's photos.
//
//	nimbus native plugin add media-library
//
//	id, err := medialibrary.Save(ctx, medialibrary.Item{Name: "receipt.jpg", Type: "image/jpeg", Data: b})
//
//	if ok, _ := medialibrary.Request(ctx); ok {
//		recent, err := medialibrary.List(ctx, 20)
//		jpeg, err := medialibrary.Read(ctx, recent[0].ID, 800)
//	}
//
// To let someone pick one picture, use the camera plugin's Pick: it needs no
// permission. This is for going through their photos yourself.
package medialibrary

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Item is something to save, as bytes.
type Item struct {
	Name string `json:"name"` // its file name; Android shows it, iOS ignores it
	Type string `json:"type"` // a MIME type, "image/..." or "video/..."
	Data []byte `json:"data"` // its contents
}

// Asset is something in their photos.
type Asset struct {
	ID      string    `json:"id"`     // for Read
	Video   bool      `json:"video"`  // false for a photo
	Width   int       `json:"width"`  // in pixels
	Height  int       `json:"height"` // in pixels
	Created time.Time `json:"-"`      // when it was taken
	Time    float64   `json:"time"`   // the same, in seconds since 1970
}

// Request asks to read their photos, once. Save doesn't need it.
func Request(ctx context.Context) (bool, error) {
	r, err := plugin.Call[struct {
		Granted bool `json:"granted"`
	}](ctx, "medialibrary", "request", nil)
	return r.Granted, err
}

// Save adds it to their photos, and asks first on iOS. Android needs version
// 10 or later.
//
//nimbus:param i what to save
//nimbus:returns its ID
func Save(ctx context.Context, i Item) (string, error) {
	r, err := plugin.Call[struct {
		ID string `json:"id"`
	}](ctx, "medialibrary", "save", i)
	return r.ID, err
}

// List gives their newest photos and videos, after Request.
//
//nimbus:param limit how many; 50 if it's 0 or less
//nimbus:returns the newest first
func List(ctx context.Context, limit int) ([]Asset, error) {
	if limit <= 0 {
		limit = 50
	}
	list, err := plugin.Call[[]Asset](ctx, "medialibrary", "list", map[string]int{"limit": limit})
	for i := range list {
		list[i].Created = time.Unix(int64(list[i].Time), 0)
	}
	return list, err
}

// Read gives one as a JPEG, or a still from a video.
//
//nimbus:param id the Asset's ID
//nimbus:param maxSize its longer side, in pixels; 0 is full size
//nimbus:returns the JPEG
func Read(ctx context.Context, id string, maxSize int) ([]byte, error) {
	r, err := plugin.Call[struct {
		Data []byte `json:"data"`
	}](ctx, "medialibrary", "read", map[string]any{"id": id, "maxSize": maxSize})
	return r.Data, err
}
