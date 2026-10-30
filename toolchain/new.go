package toolchain

import (
	"embed"
	"fmt"
	"image/color"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
)

//go:embed all:template
var projectTemplate embed.FS

// NewProject describes a project for New to create.
type NewProject struct {
	Dir    string // created; must not exist or be empty
	Name   string // display name, e.g. "Shop"
	Module string // Go module path, e.g. "shop"
	ID     string // bundle id / application id, e.g. "space.nimbusgo.shop"

	// Template is the starting app; "counter", the only one for now.
	Template string

	// Server is what the app talks to. "none" (the default) makes the phone
	// app alone, for any backend (Node, Go, anything that speaks HTTP).
	// "nimbus" adds the app to Dir, a Nimbus app that `nimbus new` just
	// made: its server also serves the app's pages to browsers.
	Server string

	// The nimbus-native the project requires: a version, or a local
	// checkout (Replace) for working on nimbus-native itself.
	Version string
	Replace string
}

// Templates are the apps New can start from.
var Templates = []string{"counter"}

// templateLooks are each template's icon tile and background, and launch
// screen color (its theme's primary, and its light background).
var templateLooks = map[string]struct{ tile, icon, launch color.RGBA }{
	"counter": {tile: color.RGBA{0x8b, 0x5c, 0xf6, 0xff}, icon: color.RGBA{0x6d, 0x28, 0xd9, 0xff}, launch: color.RGBA{0xf5, 0xf3, 0xff, 0xff}},
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
	} else if repo := os.Getenv("NIMBUS_NATIVE_CHECKOUT"); repo != "" {
		p.Version, p.Replace = "v0.0.0", repo // working on nimbus-native itself
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

// New creates a project: the shared UI and the phone entry, and
// native/ios and native/android projects that open in Xcode and Android
// Studio; with a Nimbus server, also the web app it serves.
func (e *Env) New(p NewProject) error {
	nimbus := p.Server == "nimbus"
	switch {
	case p.Server != "" && p.Server != "none" && !nimbus:
		return fmt.Errorf("-server %q: use nimbus or none", p.Server)
	case nimbus:
		if !exists(filepath.Join(p.Dir, "go.mod")) || !exists(filepath.Join(p.Dir, "start", "routes.go")) {
			return fmt.Errorf("%s is not a Nimbus app (no go.mod and start/routes.go): make one with `nimbus native new`", p.Dir)
		}
		if exists(filepath.Join(p.Dir, "native")) {
			return fmt.Errorf("%s already has a native/ folder", p.Dir)
		}
		mod, err := modulePath(p.Dir)
		if err != nil {
			return err
		}
		p.Module = mod
	default:
		if entries, err := os.ReadDir(p.Dir); err == nil && len(entries) > 0 {
			return fmt.Errorf("%s already exists and is not empty", p.Dir)
		}
	}
	idPath := strings.ReplaceAll(p.ID, ".", "/")
	r := strings.NewReplacer("__NAME__", p.Name, "__MODULE__", p.Module, "__ID__", p.ID,
		"__IDPATH__", idPath, "__NATIVE_VERSION__", p.Version)

	if p.Template == "" {
		p.Template = "counter"
	}
	if !contains(Templates, p.Template) {
		return fmt.Errorf("no template %q (one of %s)", p.Template, strings.Join(Templates, ", "))
	}
	e.step("create %s (%s)", p.Dir, p.Template)
	// The native projects every app has, the app's own code, then what its
	// server needs: a go.mod alone, or the web app's place in a Nimbus app.
	roots := []string{"template/common", "template/" + p.Template, "template/app-only"}
	if nimbus {
		roots[2] = "template/nimbus"
		ignore, err := os.ReadFile(filepath.Join(p.Dir, ".gitignore"))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		defer mergeGitignore(p.Dir, ignore)
	}
	for _, root := range roots {
		if err := writeTemplate(root, p.Dir, r); err != nil {
			return err
		}
	}
	if nimbus {
		if err := os.MkdirAll(filepath.Join(p.Dir, "public", "fonts"), 0o755); err != nil {
			return err
		}
		if err := mountWebApp(p.Dir); err != nil {
			return err
		}
		e.step("go get github.com/CodeSyncr/nimbus-native@%s", p.Version)
		if err := e.runIn(p.Dir, nil, "go", "mod", "edit", "-require=github.com/CodeSyncr/nimbus-native@"+p.Version); err != nil {
			return err
		}
	}
	// A first icon and launch screen in the app's colors; replace them with
	// `nimbus native icon <image>`.
	look := templateLooks[p.Template]
	if err := WriteIcons(p.Dir, DefaultIcon(look.tile), look.icon, look.launch); err != nil {
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

	// The pages package is only the generated table: it must exist before
	// go mod tidy looks for the packages the app imports.
	if err := e.GenRoutes(p.Dir); err != nil {
		return err
	}
	e.step("go mod tidy")
	if err := e.runIn(p.Dir, nil, "go", "mod", "tidy"); err != nil {
		return err
	}

	a, err := LoadProject(p.Dir, "app/mobile", p.ID)
	if err != nil {
		return err
	}
	_, err = e.Sync(a)
	return err
}

// mergeGitignore puts a Nimbus app's own .gitignore lines (before) back in
// front of the native ones the template wrote over it.
func mergeGitignore(dir string, before []byte) {
	path := filepath.Join(dir, ".gitignore")
	native, err := os.ReadFile(path)
	if err != nil || len(before) == 0 {
		return
	}
	out := strings.TrimRight(string(before), "\n") + "\n\n# Nimbus Native\n"
	have := map[string]bool{}
	for _, l := range strings.Split(string(before), "\n") {
		have[strings.TrimSpace(l)] = true
	}
	for _, l := range strings.Split(string(native), "\n") {
		if t := strings.TrimSpace(l); t == "" || !have[t] {
			out += l + "\n"
		}
	}
	os.WriteFile(path, []byte(strings.TrimRight(out, "\n")+"\n"), 0o644)
}

// mountWebApp makes a Nimbus app's routes serve the app's pages: a call to
// RegisterNative (start/native.go) at the top of RegisterRoutes, and "/"
// left to the app's home page instead of the Nimbus welcome view.
func mountWebApp(dir string) error {
	path := filepath.Join(dir, "start", "routes.go")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s := string(b)
	const fn = "func RegisterRoutes(app *nimbus.App) {\n"
	if !strings.Contains(s, fn) {
		return fmt.Errorf("%s has no RegisterRoutes(app *nimbus.App): add RegisterNative(app) to your routes yourself", path)
	}
	s = strings.Replace(s, fn, fn+"\tRegisterNative(app) // the app's pages, for browsers (start/native.go)\n", 1)
	s = strings.Replace(s, "\tapp.Router.Get(\"/\", homeHandler)\n",
		"\t// \"/\" is the app's home page (pages/page.go), served by RegisterNative.\n", 1)
	return os.WriteFile(path, []byte(s), 0o644)
}

// Sync brings a project's copies up to date with the nimbus-native its
// go.mod requires: the kits under native/ and the generated Swift (which
// Xcode compiles only if it exists when the build starts). It reports
// whether a kit changed.
func (e *Env) Sync(a *App) (bool, error) {
	e.Repo = moduleDir(a.Root)
	if e.Repo == "" {
		return false, fmt.Errorf("nimbus-native module not found from %s", a.Root)
	}
	ios, err := e.syncIOSKit(filepath.Join(a.Root, "native/ios/NimbusNativeKit"), a.Root)
	if err != nil {
		return false, err
	}
	android, err := e.syncAndroidKit(filepath.Join(a.Root, "native/android/nimbusnativekit/src/main/kotlin"), a.Root)
	if err != nil {
		return false, err
	}
	return ios || android, e.writeSwift(a, filepath.Join(a.Root, IOSGeneratedSwift))
}

// IOSGeneratedSwift is where a project's generated Swift lives.
const IOSGeneratedSwift = "native/ios/Runner/Generated/NimbusGenerated.swift"

// writeTemplate writes the files under root (in the embedded templates) to
// dir, filling in the project's names.
func writeTemplate(root, dir string, r *strings.Replacer) error {
	return fs.WalkDir(projectTemplate, root, func(name string, d fs.DirEntry, err error) error {
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
		rel := r.Replace(strings.TrimSuffix(strings.TrimPrefix(name, root+"/"), ".tmpl"))
		if rel == "gitignore" {
			rel = ".gitignore" // stored without the dot so it doesn't apply to this repo
		}
		out := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if path.Base(name) == "gradlew" {
			mode = 0o755
		}
		return os.WriteFile(out, b, mode)
	})
}
