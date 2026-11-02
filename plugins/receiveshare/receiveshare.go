// Package receiveshare puts your app in the share sheet, so people can send
// it text, links, pictures and files from other apps.
//
//	nimbus native plugin add receive-share
//
//	go receiveshare.Watch(ctx, receiveshare.Options{}, func(s receiveshare.Share) {
//		if s.URL != "" {
//			save(s.URL)
//		}
//		for _, f := range s.Files {
//			upload(f.Name, f.Data)
//		}
//	})
//
// On Android, the share opens your app. On iOS it goes through a Share
// Extension, which plugin add puts in the Xcode project: the person sees a
// short "Sent" sheet and stays where they were, and the share waits for your
// app's next time on screen. iOS doesn't let a share extension open its app.
package receiveshare

import (
	"context"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Share is one thing someone sent, from one share sheet.
type Share struct {
	Text    string `json:"text"`    // what they shared as text; may hold a link too
	URL     string `json:"url"`     // a web address, when they shared one
	Subject string `json:"subject"` // a title some apps send with it
	Files   []File `json:"files"`   // pictures, videos, documents
	// At is when it arrived.
	At   time.Time `json:"-"`
	Time float64   `json:"time"` // the same, in seconds since 1970
}

// File is a shared file. It comes as bytes, so it works under
// `nimbus native dev` too.
type File struct {
	Name string `json:"name"` // its file name
	Type string `json:"type"` // its media type, such as "image/jpeg"
	Size int64  `json:"size"` // in bytes
	// Data is its contents, or empty when it's larger than Options.MaxBytes.
	Data []byte `json:"data"`
}

// Options say what to take.
type Options struct {
	// MaxBytes is the largest file that comes with its Data; 0 means 20 MB.
	// Larger ones still come, with their name, type and size.
	MaxBytes int64 `json:"maxBytes,omitempty"`
}

// Take gives the shares that have arrived, oldest first, and forgets them:
// each comes once. Call it when the app starts, so one that launched the app
// isn't missed.
//
//nimbus:param o the largest file to bring the bytes of
//nimbus:returns the shares
func Take(ctx context.Context, o Options) ([]Share, error) {
	if o.MaxBytes <= 0 {
		o.MaxBytes = 20 << 20
	}
	list, err := plugin.Call[[]Share](ctx, "receiveshare", "take", o)
	for i := range list {
		list[i].At = time.UnixMilli(int64(list[i].Time * 1000))
	}
	return list, err
}

// Watch calls f with each share as it arrives, including any waiting from
// before, until ctx ends. It blocks, so run it from a goroutine in an effect.
//
//nimbus:param o the largest file to bring the bytes of
//nimbus:param f called with each share
//nimbus:returns why it stopped, which is ctx.Err() when ctx ends
func Watch(ctx context.Context, o Options, f func(Share)) error {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		list, err := Take(ctx, o)
		if err != nil {
			return err
		}
		for _, s := range list {
			f(s)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}
