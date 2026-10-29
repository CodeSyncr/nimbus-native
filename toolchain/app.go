package toolchain

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// App is the project being built. For now its layout is fixed:
//
//	<dir>/*.go             the Go app (package main)
//	<dir>/ios/*.swift      the iOS host code
//	<dir>/android/*.java   the Android host code (package <id>, MainActivity)
type App struct {
	Dir  string // absolute
	Name string // display and executable name, e.g. "Hello"
	ID   string // bundle id / application id, e.g. "space.nimbusgo.hello"

	// For dev shells: the Go package compiled into the app instead of Dir,
	// whose schemas also count toward the hash.
	GoDir string
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
	return filepath.Join(a.Dir, "build", platform)
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
