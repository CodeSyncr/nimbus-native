package toolchain

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/CodeSyncr/nimbus-native/wire"
	"github.com/CodeSyncr/nimbus-native/wire/gen"
	"github.com/CodeSyncr/nimbus-native/wire/schema"
)

// Generated holds the host code produced by Gen.
type Generated struct {
	Combined    uint64   // every schema the native library links: what hosts pass to nn_start
	AppCombined uint64   // only the schemas the app's own package links
	Hashes      []uint64 // the schemas behind Combined
	SwiftFiles  []string // compile into the iOS app
	KotlinFiles []string // compile into the Android app
}

// Gen regenerates every wire schema inside the app (directories whose Go
// files carry //wire: directives), writing wire_gen.go next to each, and
// host code under build/gen. The combined hash covers every schema package
// the app links, including ones from its dependencies.
func (e *Env) Gen(a *App) (*Generated, error) {
	dirs, err := linkedPackageDirs(a.Dir)
	if err != nil {
		return nil, err
	}
	appDirs := map[string]bool{}
	for _, d := range dirs {
		appDirs[d] = true
	}
	if a.GoDir != "" && a.GoDir != a.Dir {
		more, err := linkedPackageDirs(a.GoDir)
		if err != nil {
			return nil, err
		}
		for _, d := range more {
			if !appDirs[d] {
				dirs = append(dirs, d)
			}
		}
	}
	out := filepath.Join(a.Dir, "build", "gen")
	if err := os.RemoveAll(out); err != nil {
		return nil, err
	}
	swiftDir := filepath.Join(out, "swift")
	ktDir := filepath.Join(out, "kotlin", filepath.FromSlash(strings.ReplaceAll(a.ID, ".", "/")))
	for _, d := range []string{swiftDir, ktDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}

	res := &Generated{}
	var hashes, appHashes []uint64
	for _, dir := range dirs {
		if !isSchemaDir(dir) {
			continue
		}
		if !within(dir, a.Dir) {
			// A dependency's schema: its host code ships with that dependency.
			s, err := schema.Load(dir)
			if err != nil {
				return nil, fmt.Errorf("schema %s: %w", dir, err)
			}
			hashes = append(hashes, s.Hash)
			if appDirs[dir] {
				appHashes = append(appHashes, s.Hash)
			}
			continue
		}
		o, err := gen.Generate(dir, a.ID)
		if err != nil {
			return nil, fmt.Errorf("schema %s: %w", rel(a.Dir, dir), err)
		}
		e.step("gen %s → Go, Swift, Kotlin (%#016x)", rel(a.Dir, dir), o.Schema.Hash)
		if err := writeIfChanged(filepath.Join(dir, gen.GoFile), o.Go); err != nil {
			return nil, err
		}
		ns := gen.Namespace(o.Schema)
		files := map[string][]byte{
			filepath.Join(swiftDir, ns+".swift"): o.Swift,
			filepath.Join(ktDir, ns+".kt"):       o.Kotlin,
		}
		for path, src := range files {
			if err := os.WriteFile(path, src, 0o644); err != nil {
				return nil, err
			}
		}
		res.SwiftFiles = append(res.SwiftFiles, filepath.Join(swiftDir, ns+".swift"))
		res.KotlinFiles = append(res.KotlinFiles, filepath.Join(ktDir, ns+".kt"))
		hashes = append(hashes, o.Schema.Hash)
		if appDirs[dir] {
			appHashes = append(appHashes, o.Schema.Hash)
		}
	}

	res.Combined, res.AppCombined, res.Hashes = wire.Combine(hashes), wire.Combine(appHashes), hashes
	sw, kt := filepath.Join(swiftDir, "NimbusSchema.swift"), filepath.Join(ktDir, "NimbusSchema.kt")
	if err := os.WriteFile(sw, gen.SwiftSchema(res.Combined), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(kt, gen.KotlinSchema(a.ID, res.Combined), 0o644); err != nil {
		return nil, err
	}
	res.SwiftFiles = append(res.SwiftFiles, sw)
	res.KotlinFiles = append(res.KotlinFiles, kt)
	return res, nil
}

// linkedPackageDirs lists the directories of every package the app builds
// with, so schemas anywhere in its dependency graph count toward the hash.
// Cross-compilation build tags don't change which schemas exist, so the
// host platform's view is enough.
func linkedPackageDirs(appDir string) ([]string, error) {
	cmd := exec.Command("go", "list", "-deps", "-f", "{{if not .Standard}}{{.Dir}}{{end}}", ".")
	cmd.Dir = appDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %w\n%s", err, stderr.String())
	}
	var dirs []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			dirs = append(dirs, l)
		}
	}
	return dirs, nil
}

// isSchemaDir reports whether a package declares wire messages.
func isSchemaDir(dir string) bool {
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || strings.HasSuffix(f, "_gen.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err == nil && bytes.Contains(b, []byte("//wire:message")) {
			return true
		}
	}
	return false
}

func writeIfChanged(path string, b []byte) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, b) {
		return nil
	}
	return os.WriteFile(path, b, 0o644)
}

func within(dir, root string) bool {
	r, err := filepath.Rel(root, dir)
	return err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator))
}

func rel(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil {
		return r
	}
	return p
}
