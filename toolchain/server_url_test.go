package toolchain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalServer(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("PORT=4100\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LocalServer(root, OnSimulator); got != "http://127.0.0.1:4100" {
		t.Fatalf("simulator: %s", got)
	}
	if got := LocalServer(root, OnEmulator); got != "http://10.0.2.2:4100" {
		t.Fatalf("emulator: %s", got)
	}
	if got := LocalServer(root, OnPhone); got != "" && (strings.Contains(got, "127.0.0.1") || !strings.HasSuffix(got, ":4100")) {
		t.Fatalf("phone: %s", got)
	}
}

func TestSetServer(t *testing.T) {
	t.Setenv(serverEnv, "")
	if err := SetServer("https://api.example.com/"); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(serverEnv); got != "https://api.example.com" {
		t.Fatalf("got %q", got)
	}
	for _, bad := range []string{"api.example.com", "ftp://x", "http://", "https://a b.com", "https://a.com' -X evil"} {
		if SetServer(bad) == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}
