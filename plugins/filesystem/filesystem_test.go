package filesystem

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"testing"
)

func TestFiles(t *testing.T) {
	root := t.TempDir()
	dir = func(context.Context, Folder) (string, error) { return root, nil }
	ctx := context.Background()
	if err := Write(ctx, Data, "orders/a.json", []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := Write(ctx, Data, "orders/a.json", []byte("two")); err != nil {
		t.Fatal(err)
	}
	if err := Append(ctx, Data, "orders/a.json", []byte("!")); err != nil {
		t.Fatal(err)
	}
	if b, err := Read(ctx, Data, "orders/a.json"); err != nil || string(b) != "two!" {
		t.Fatalf("%q %v", b, err)
	}
	if err := Write(ctx, Data, "orders/b.json", nil); err != nil {
		t.Fatal(err)
	}
	if names, err := List(ctx, Data, "orders"); err != nil || !slices.Equal(names, []string{"a.json", "b.json"}) {
		t.Fatalf("%v %v", names, err)
	}
	if ok, err := Exists(ctx, Data, "orders/a.json"); !ok || err != nil {
		t.Fatalf("%v %v", ok, err)
	}
	if err := Delete(ctx, Data, "orders/a.json"); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(ctx, Data, "orders/a.json"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("after delete: %v", err)
	}
	if names, err := List(ctx, Data, "nothing"); err != nil || len(names) != 0 {
		t.Fatalf("%v %v", names, err)
	}
}

func TestPathsStayInside(t *testing.T) {
	root := t.TempDir()
	dir = func(context.Context, Folder) (string, error) { return root, nil }
	ctx := context.Background()
	for _, bad := range []string{"../x", "a/../../x", "/etc/passwd", ".."} {
		if err := Write(ctx, Data, bad, []byte("x")); !errors.Is(err, ErrOutside) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if err := Delete(ctx, Data, "."); !errors.Is(err, ErrOutside) {
		t.Errorf("delete everything: %v", err)
	}
	if err := Write(ctx, Data, "a/../b.txt", []byte("x")); err != nil {
		t.Errorf("a path that stays inside: %v", err)
	}
}
