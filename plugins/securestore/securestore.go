// Package securestore keeps small secrets, such as auth tokens, in the
// platform's secure storage: the Keychain on iOS, an Android Keystore key
// on Android. Values stay on the device: they are not synced or backed up.
//
//	if err := securestore.Set(ctx, "token", t); err != nil { ... }
//	token, ok, err := securestore.Get(ctx, "token")
package securestore

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

const name = "secureStorage"

type keyArgs struct {
	Key string `json:"key"`
}

type setArgs struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Get reads key: ok is false when nothing is stored under it.
func Get(ctx context.Context, key string) (value string, ok bool, err error) {
	r, err := plugin.Call[struct {
		Value *string `json:"value"`
		Found bool    `json:"found"`
	}](ctx, name, "get", keyArgs{key})
	if err != nil || !r.Found || r.Value == nil {
		return "", false, err
	}
	return *r.Value, true, nil
}

// Set stores value under key, replacing what was there.
func Set(ctx context.Context, key, value string) error {
	_, err := plugin.Call[any](ctx, name, "set", setArgs{key, value})
	return err
}

// Delete removes key; deleting a missing key is not an error.
func Delete(ctx context.Context, key string) error {
	_, err := plugin.Call[any](ctx, name, "delete", keyArgs{key})
	return err
}
