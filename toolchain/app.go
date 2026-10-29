package toolchain

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// App is the app being built, in one of two layouts. A single directory
// (the examples):
//
//	<dir>/*.go             the Go app (package main)
//	<dir>/ios/*.swift      the iOS host code
//	<dir>/android/*.kt     the Android host code (package <id>, MainActivity)
//
// or a project made by `nimbus native new`, where Root is the Go module and
// Dir its phone entry package:
//
//	<root>/app/mobile      the phone's package main (Dir)
//	<root>/native/ios      an Xcode project
//	<root>/native/android  a Gradle project
type App struct {
	Dir  string // absolute
	Name string // display and executable name, e.g. "Hello"
	ID   string // bundle id / application id, e.g. "space.nimbusgo.hello"

	// Root is the project's module root; empty for a single directory.
	// Schemas anywhere under it are generated, and build output goes there.
	Root string

	// For dev shells: the Go package compiled into the app instead of Dir,
	// whose schemas also count toward the hash.
	GoDir string
}

// root is where the app's own code and build output live.
func (a *App) root() string {
	if a.Root != "" {
		return a.Root
	}
	return a.Dir
}

// goDir is the Go package built into the app's native library.
func (a *App) goDir() string {
	if a.GoDir != "" {
		return a.GoDir
	}
	return a.Dir
}

var idPart = regexp.MustCompile(`[^a-z0-9]`)

// LoadApp describes the app in dir. Name and ID default from the directory
// name when empty.
func LoadApp(dir, name, id string) (*App, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, fmt.Errorf("app directory: %w", err)
	}
	base := filepath.Base(abs)
	if name == "" {
		name = identifier(base)
	}
	if id == "" {
		id = "space.nimbusgo." + idPart.ReplaceAllString(strings.ToLower(base), "")
	}
	return &App{Dir: abs, Name: name, ID: id}, nil
}

// BuildDir is where build output for a platform goes.
func (a *App) BuildDir(platform string) string {
	return filepath.Join(a.root(), "build", platform)
}

// identifier turns a directory name into an app name that is also a valid
// Swift module and executable name: "my-app" → "MyApp".
func identifier(s string) string {
	var b strings.Builder
	upper := true
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' && b.Len() > 0:
			if upper {
				r = unicode.ToUpper(r)
			}
			b.WriteRune(r)
			upper = false
		default:
			upper = true
		}
	}
	if b.Len() == 0 {
		return "App"
	}
	return b.String()
}

func globs(patterns ...string) []string {
	var out []string
	for _, p := range patterns {
		m, _ := filepath.Glob(p)
		out = append(out, m...)
	}
	return out
}

// IsProject reports whether dir is a project made by `nimbus native new`:
// a Go module with native/ios or native/android.
func IsProject(dir string) bool {
	return exists(filepath.Join(dir, "go.mod")) &&
		(exists(filepath.Join(dir, "native/ios")) || exists(filepath.Join(dir, "native/android")))
}

// LoadProject describes the project whose Go module is at root, with its
// phone entry package at root/entry. id may be empty; IOSProjectID and
// AndroidProjectID read it from the native projects.
func LoadProject(root, entry, id string) (*App, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(abs, filepath.FromSlash(entry))
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("phone entry package: %w", err)
	}
	return &App{Dir: dir, Root: abs, Name: identifier(filepath.Base(abs)), ID: id}, nil
}

var (
	bundleID      = regexp.MustCompile(`PRODUCT_BUNDLE_IDENTIFIER = "?([A-Za-z0-9.\-]+)"?;`)
	applicationID = regexp.MustCompile(`applicationId\s*=\s*"([^"]+)"`)
)

// IOSProjectID is the bundle id in native/ios's Xcode project.
func IOSProjectID(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "native/ios/Runner.xcodeproj/project.pbxproj"))
	if err != nil {
		return "", err
	}
	if m := bundleID.FindSubmatch(b); m != nil {
		return string(m[1]), nil
	}
	return "", fmt.Errorf("no PRODUCT_BUNDLE_IDENTIFIER in the Xcode project")
}

// AndroidProjectID is the application id in native/android's app module.
func AndroidProjectID(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "native/android/app/build.gradle.kts"))
	if err != nil {
		return "", err
	}
	if m := applicationID.FindSubmatch(b); m != nil {
		return string(m[1]), nil
	}
	return "", fmt.Errorf("no applicationId in native/android/app/build.gradle.kts")
}
