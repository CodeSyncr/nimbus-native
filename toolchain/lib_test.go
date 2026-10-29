package toolchain

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncDir(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{"a.swift": []byte("a"), "sub/b.swift": []byte("b")}
	if changed, err := syncDir(dir, files); err != nil || !changed {
		t.Fatalf("first sync: changed %v, err %v", changed, err)
	}
	if changed, _ := syncDir(dir, files); changed {
		t.Fatal("an identical sync reported a change")
	}
	write(t, filepath.Join(dir, "stale.swift"), "old")
	write(t, filepath.Join(dir, ".swiftpm/state"), "xcode's")
	if changed, _ := syncDir(dir, files); !changed {
		t.Fatal("removing a stale file is a change")
	}
	if exists(filepath.Join(dir, "stale.swift")) || !exists(filepath.Join(dir, ".swiftpm/state")) {
		t.Fatal("sync should remove stale files and leave dot-directories alone")
	}

	// A fresh copy is not "updated": the first build needn't run twice.
	fresh := filepath.Join(t.TempDir(), "kit")
	if changed, _ := syncKit(fresh, files); changed {
		t.Fatal("creating the kit reported an update")
	}
	if changed, _ := syncKit(fresh, map[string][]byte{"a.swift": []byte("a2")}); !changed {
		t.Fatal("changing the kit did not report an update")
	}
}

// The phone app may not link Nimbus or the project's server packages.
func TestCheckPhoneDeps(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module shop\n\ngo 1.21\n")
	write(t, filepath.Join(root, "features/orders/server/server.go"), "package server\n")
	write(t, filepath.Join(root, "features/orders/domain/domain.go"), "package domain\n")
	write(t, filepath.Join(root, "app/app.go"), "package app\n\nimport _ \"shop/features/orders/domain\"\n")
	write(t, filepath.Join(root, "app/mobile/main.go"), "package main\n\nimport _ \"shop/app\"\n\nfunc main() {}\n")

	a, err := LoadProject(root, "app/mobile", "space.nimbusgo.shop")
	if err != nil {
		t.Fatal(err)
	}
	e := &Env{}
	if err := e.checkPhoneDeps(a); err != nil {
		t.Fatalf("clean app refused: %v", err)
	}

	write(t, filepath.Join(root, "app/app.go"), "package app\n\nimport _ \"shop/features/orders/server\"\n")
	err = e.checkPhoneDeps(a)
	if err == nil || !strings.Contains(err.Error(), "shop/app\n  → shop/features/orders/server") {
		t.Fatalf("server import not refused with its chain: %v", err)
	}
}

// The ids `run` launches are read back from the native projects new writes.
func TestProjectIDs(t *testing.T) {
	root := t.TempDir()
	r := strings.NewReplacer("__ID__", "com.example.shop", "__NAME__", "Shop", "__MODULE__", "shop")
	for _, name := range []string{"native/ios/Runner.xcodeproj/project.pbxproj", "native/android/app/build.gradle.kts"} {
		b, err := fs.ReadFile(projectTemplate, "template/"+name)
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(root, name), r.Replace(string(b)))
	}
	if id, err := IOSProjectID(root); id != "com.example.shop" {
		t.Errorf("iOS id %q, %v", id, err)
	}
	if id, err := AndroidProjectID(root); id != "com.example.shop" {
		t.Errorf("Android id %q, %v", id, err)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
