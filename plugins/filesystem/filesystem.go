// Package filesystem keeps files for the app. It's plain Go on top of the
// folders the system gives each app, so it needs no plugin add. Paths are
// relative to the folder, and can't leave it.
//
//	filesystem.Write(ctx, filesystem.Data, "orders/1042.json", b)
//	b, err := filesystem.Read(ctx, filesystem.Data, "orders/1042.json")
//	names, err := filesystem.List(ctx, filesystem.Data, "orders")
//
// Under `nimbus native dev` the app runs on this machine, so the files live
// in the user cache directory instead of on the device.
package filesystem

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CodeSyncr/nimbus-native/plugins/appinfo"
)

// Folder is where a file lives.
type Folder int

const (
	Data  Folder = iota // kept until the app is deleted
	Cache               // the system may clear it when space runs low
)

// ErrOutside is a path that leaves its folder.
var ErrOutside = errors.New("filesystem: path leaves its folder")

// dir finds a folder; tests replace it.
var dir = func(ctx context.Context, f Folder) (string, error) {
	if os.Getenv("NIMBUS_DEV_ADDR") != "" {
		d, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(d, "nimbus-native", "dev-files", map[Folder]string{Data: "data", Cache: "cache"}[f]), nil
	}
	info, err := appinfo.Get(ctx)
	if f == Cache {
		return info.CacheDir, err
	}
	return info.DataDir, err
}

// Path is the file's full path, for what takes one: an upload, say.
func Path(ctx context.Context, f Folder, name string) (string, error) {
	if filepath.IsAbs(name) {
		return "", ErrOutside
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrOutside
	}
	d, err := dir(ctx, f)
	if err != nil {
		return "", err
	}
	return filepath.Join(d, clean), nil
}

// Write replaces a file, making its folders. A crash partway leaves the old
// file whole.
func Write(ctx context.Context, f Folder, name string, data []byte) error {
	p, err := Path(ctx, f, name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}

// Append adds to the end of a file, making it if needed.
func Append(ctx context.Context, f Folder, name string, data []byte) error {
	p, err := Path(ctx, f, name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}

// Read returns a file. A missing one is fs.ErrNotExist.
func Read(ctx context.Context, f Folder, name string) ([]byte, error) {
	p, err := Path(ctx, f, name)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

// Exists reports whether a file is there.
func Exists(ctx context.Context, f Folder, name string) (bool, error) {
	p, err := Path(ctx, f, name)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// List returns the names in a folder, sorted. A missing folder is empty.
func List(ctx context.Context, f Folder, name string) ([]string, error) {
	p, err := Path(ctx, f, name)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".tmp-") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// Delete removes a file or folder. A missing one is fine.
func Delete(ctx context.Context, f Folder, name string) error {
	p, err := Path(ctx, f, name)
	if err != nil {
		return err
	}
	if root, _ := Path(ctx, f, "."); p == root {
		return ErrOutside // Delete(".") would take everything
	}
	return os.RemoveAll(p)
}
