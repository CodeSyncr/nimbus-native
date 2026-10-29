package toolchain

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
)

//go:embed template
var projectTemplate embed.FS

// NewProject describes a project for New to create.
type NewProject struct {
	Dir    string // created; must not exist or be empty
	Name   string // display name, e.g. "Shop"
	Module string // Go module path, e.g. "shop"
	ID     string // bundle id / application id, e.g. "space.nimbusgo.shop"

	// The nimbus-native the project requires: a version, or a local
	// checkout (Replace) for working on nimbus-native itself.
	Version string
	Replace string
}

// NewProjectFor fills in a project's names from its directory, and the
// nimbus-native version from this toolchain's own build: the project gets
// the version that made it. A toolchain run from a checkout has no
// version, so the project points at that checkout.
func NewProjectFor(dir, name, module, id string) (NewProject, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return NewProject{}, err
	}
	base := filepath.Base(abs)
	p := NewProject{Dir: abs, Name: name, Module: module, ID: id}
	if p.Name == "" {
		p.Name = identifier(base)
	}
	if p.Module == "" {
		p.Module = idPart.ReplaceAllString(strings.ToLower(base), "")
	}
	if p.ID == "" {
		p.ID = "space.nimbusgo." + idPart.ReplaceAllString(strings.ToLower(base), "")
	}
	if !validID.MatchString(p.ID) {
		return p, fmt.Errorf("%q is not a valid application id (like com.example.shop)", p.ID)
	}
	if p.Module == "" {
		return p, fmt.Errorf("cannot make a module path from %q; pass -module", base)
	}
	if v := ownVersion(); v != "" {
		p.Version = v
	} else if repo := moduleDir("."); repo != "" {
		p.Version, p.Replace = "v0.0.0", repo
	} else {
		return p, fmt.Errorf("this nimbus-native has no version; run it with go run github.com/CodeSyncr/nimbus-native/cmd/nimbus-native@<version>")
	}
	return p, nil
}

var validID = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)

// ownVersion is the nimbus-native version this binary was built from.
func ownVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	mods := append([]*debug.Module{&bi.Main}, bi.Deps...)
	for _, m := range mods {
		if m.Path == "github.com/CodeSyncr/nimbus-native" {
			if m.Replace != nil || m.Version == "" || m.Version == "(devel)" {
				return ""
			}
			return m.Version
		}
	}
	return ""
}

// New creates a project: a Go module with the server, the shared UI and
// the phone entry, and native/ios and native/android projects that open in
// Xcode and Android Studio.
func (e *Env) New(p NewProject) error {
	if entries, err := os.ReadDir(p.Dir); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s already exists and is not empty", p.Dir)
	}
	idPath := strings.ReplaceAll(p.ID, ".", "/")
	r := strings.NewReplacer("__NAME__", p.Name, "__MODULE__", p.Module, "__ID__", p.ID,
		"__IDPATH__", idPath, "__NATIVE_VERSION__", p.Version)

	e.step("create %s", p.Dir)
	err := fs.WalkDir(projectTemplate, "template", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := projectTemplate.ReadFile(name)
		if err != nil {
			return err
		}
		if path.Ext(name) != ".jar" {
			b = []byte(r.Replace(string(b)))
		}
		rel := r.Replace(strings.TrimSuffix(strings.TrimPrefix(name, "template/"), ".tmpl"))
		if rel == "gitignore" {
			rel = ".gitignore" // stored without the dot so it doesn't apply to this repo
		}
		out := filepath.Join(p.Dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if path.Base(name) == "gradlew" {
			mode = 0o755
		}
		return os.WriteFile(out, b, mode)
	})
	if err != nil {
		return err
	}
	if p.Replace != "" {
		f, err := os.OpenFile(filepath.Join(p.Dir, "go.mod"), os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(f, "\nreplace github.com/CodeSyncr/nimbus-native => %s\n", p.Replace)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	if e.AndroidSDK != "" {
		props := "sdk.dir=" + e.AndroidSDK + "\n"
		if err := os.WriteFile(filepath.Join(p.Dir, "native/android/local.properties"), []byte(props), 0o644); err != nil {
			return err
		}
	}

	e.step("go mod tidy")
	if err := e.runIn(p.Dir, nil, "go", "mod", "tidy"); err != nil {
		return err
	}

	// The kits come from the nimbus-native the project now requires.
	e.Repo = moduleDir(p.Dir)
	if e.Repo == "" {
		return fmt.Errorf("nimbus-native module not found from %s", p.Dir)
	}
	if _, err := e.syncIOSKit(filepath.Join(p.Dir, "native/ios/NimbusNativeKit")); err != nil {
		return err
	}
	if _, err := e.syncAndroidKit(filepath.Join(p.Dir, "native/android/nimbusnativekit/src/main/kotlin")); err != nil {
		return err
	}
	// Xcode compiles only the files its synchronized folder held when the
	// build started, so the generated Swift exists from the start.
	a, err := LoadProject(p.Dir, "app/mobile", p.ID)
	if err != nil {
		return err
	}
	return e.writeSwift(a, filepath.Join(p.Dir, IOSGeneratedSwift))
}

// IOSGeneratedSwift is where a project's generated Swift lives.
const IOSGeneratedSwift = "native/ios/Runner/Generated/NimbusGenerated.swift"
