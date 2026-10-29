package storage

import (
	"context"
	"testing"
)

func TestStorage(t *testing.T) {
	d := t.TempDir()
	dir = func(context.Context) (string, error) { return d, nil }
	ctx := context.Background()

	type settings struct{ Dark bool }
	if err := Set(ctx, "settings", settings{Dark: true}); err != nil {
		t.Fatal(err)
	}
	if err := Set(ctx, "count", 3); err != nil {
		t.Fatal(err)
	}

	// A fresh process reads the file.
	loaded = false
	var s settings
	if ok, err := Get(ctx, "settings", &s); !ok || err != nil || !s.Dark {
		t.Fatalf("settings: %v %v %+v", ok, err, s)
	}
	if err := Delete(ctx, "count"); err != nil {
		t.Fatal(err)
	}
	loaded = false
	var n int
	if ok, _ := Get(ctx, "count", &n); ok {
		t.Fatal("deleted key came back")
	}
}
