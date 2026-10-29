package toolchain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVersionLess(t *testing.T) {
	cases := []struct {
		a, b string
		less bool
	}{
		{"28.2.13676358", "29.0.14206865", true},
		{"36.1.0", "37.0.0", true},
		{"android-9", "android-36", true}, // numeric, not lexical
		{"37.0.0", "36.1.0", false},
		{"35.0.0", "35.0.0", false},
	}
	for _, c := range cases {
		if got := versionLess(c.a, c.b); got != c.less {
			t.Errorf("versionLess(%q, %q) = %v, want %v", c.a, c.b, got, c.less)
		}
	}
}

func TestNewest(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"android-9", "android-36", "android-34", ".hidden-99"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got := filepath.Base(newest(dir)); got != "android-36" {
		t.Fatalf("newest = %s, want android-36", got)
	}
	if newest(filepath.Join(dir, "missing")) != "" {
		t.Fatal("newest of a missing dir should be empty")
	}
}

func TestLoadAppDefaults(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my-app")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	a, err := LoadApp(dir, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.Name != "MyApp" || a.ID != "space.nimbusgo.myapp" {
		t.Fatalf("defaults: name %q id %q", a.Name, a.ID)
	}
}

func TestIdentifier(t *testing.T) {
	for in, want := range map[string]string{"hello": "Hello", "my-app": "MyApp", "2fast app": "FastApp", "---": "App"} {
		if got := identifier(in); got != want {
			t.Errorf("identifier(%q) = %q, want %q", in, got, want)
		}
	}
}
