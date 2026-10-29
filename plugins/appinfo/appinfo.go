// Package appinfo describes the running app and device, and where the app
// may keep files.
package appinfo

import (
	"context"
	"sync"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Info is the app and device.
type Info struct {
	Platform  string `json:"platform"` // "ios" or "android"
	ID        string `json:"id"`       // bundle id / application id
	Version   string `json:"version"`  // e.g. "1.2.0"
	Build     string `json:"build"`    // e.g. "42"
	OSVersion string `json:"osVersion"`
	Locale    string `json:"locale"`   // BCP 47, e.g. "en-IN"
	DataDir   string `json:"dataDir"`  // kept until the app is deleted
	CacheDir  string `json:"cacheDir"` // the system may clear it
}

var (
	mu     sync.Mutex
	cached *Info
)

// Get returns the app's Info (asked once, then remembered).
func Get(ctx context.Context) (Info, error) {
	mu.Lock()
	defer mu.Unlock()
	if cached != nil {
		return *cached, nil
	}
	i, err := plugin.Call[Info](ctx, "app", "info", nil)
	if err != nil {
		return Info{}, err
	}
	cached = &i
	return i, nil
}
