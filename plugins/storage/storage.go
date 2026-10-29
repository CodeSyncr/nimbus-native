// Package storage is a small key-value store for app data that should
// survive restarts: settings, drafts, the last-seen screen. Values are
// JSON in one file in the app's data directory; keep it to kilobytes, and
// put secrets in securestore instead.
//
//	storage.Set(ctx, "theme", "dark")
//	var theme string
//	ok, err := storage.Get(ctx, "theme", &theme)
//
// Under `nimbus native dev` the app runs on this machine, so the file lives
// in the user cache directory instead of on the device.
package storage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/CodeSyncr/nimbus-native/plugins/appinfo"
)

var (
	mu     sync.Mutex
	loaded bool
	values map[string]json.RawMessage
	file   string

	// dir finds the directory for the file; tests replace it.
	dir = func(ctx context.Context) (string, error) {
		if os.Getenv("NIMBUS_DEV_ADDR") != "" {
			d, err := os.UserCacheDir()
			if err != nil {
				return "", err
			}
			return filepath.Join(d, "nimbus-native", "dev-storage"), nil
		}
		info, err := appinfo.Get(ctx)
		return info.DataDir, err
	}
)

func load(ctx context.Context) error {
	if loaded {
		return nil
	}
	d, err := dir(ctx)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0o700); err != nil {
		return err
	}
	file = filepath.Join(d, "nimbus-storage.json")
	values = map[string]json.RawMessage{}
	if b, err := os.ReadFile(file); err == nil {
		_ = json.Unmarshal(b, &values) // a damaged file starts over empty
	}
	loaded = true
	return nil
}

// Get decodes the value under key into dst; ok is false when there is none.
func Get(ctx context.Context, key string, dst any) (ok bool, err error) {
	mu.Lock()
	defer mu.Unlock()
	if err := load(ctx); err != nil {
		return false, err
	}
	raw, ok := values[key]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(raw, dst)
}

// Set stores value (anything JSON can encode) under key and saves.
func Set(ctx context.Context, key string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	if err := load(ctx); err != nil {
		return err
	}
	values[key] = b
	return save()
}

// Delete removes key and saves.
func Delete(ctx context.Context, key string) error {
	mu.Lock()
	defer mu.Unlock()
	if err := load(ctx); err != nil {
		return err
	}
	if _, ok := values[key]; !ok {
		return nil
	}
	delete(values, key)
	return save()
}

// save writes the whole store to a temporary file and renames it over the
// old one, so a crash mid-write never leaves half a file.
func save() error {
	b, err := json.Marshal(values)
	if err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}
