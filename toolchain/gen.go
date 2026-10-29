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
	Combined   uint64
	SwiftFiles []string // compile into the iOS app
	JavaFiles  []string // compile into the Android app
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
	out := filepath.Join(a.Dir, "build", "gen")
	if err := os.RemoveAll(out); err != nil {
		return nil, err
	}
	swiftDir := filepath.Join(out, "swift")
	javaDir := filepath.Join(out, "java", filepath.FromSlash(strings.ReplaceAll(a.ID, ".", "/")))
	for _, d := range []string{swiftDir, javaDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}

	res := &Generated{}
	var hashes []uint64
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
			continue
		}
		o, err := gen.Generate(dir, a.ID)
		if err != nil {
			return nil, fmt.Errorf("schema %s: %w", rel(a.Dir, dir), err)
		}
		e.step("gen %s → Go, Swift, Java (%#016x)", rel(a.Dir, dir), o.Schema.Hash)
		if err := writeIfChanged(filepath.Join(dir, gen.GoFile), o.Go); err != nil {
			return nil, err
		}
		ns := gen.Namespace(o.Schema)
		files := map[string][]byte{
			filepath.Join(swiftDir, ns+".swift"): o.Swift,
			filepath.Join(javaDir, ns+".java"):   o.Java,
		}
		for path, src := range files {
			if err := os.WriteFile(path, src, 0o644); err != nil {
				return nil, err
			}
		}
		res.SwiftFiles = append(res.SwiftFiles, filepath.Join(swiftDir, ns+".swift"))
		res.JavaFiles = append(res.JavaFiles, filepath.Join(javaDir, ns+".java"))
		hashes = append(hashes, o.Schema.Hash)
	}

	res.Combined = wire.Combine(hashes)
	sw, jv := filepath.Join(swiftDir, "NimbusSchema.swift"), filepath.Join(javaDir, "NimbusSchema.java")
	if err := os.WriteFile(sw, gen.SwiftSchema(res.Combined), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(jv, gen.JavaSchema(a.ID, res.Combined), 0o644); err != nil {
		return nil, err
	}
	res.SwiftFiles = append(res.SwiftFiles, sw)
	res.JavaFiles = append(res.JavaFiles, jv)
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
