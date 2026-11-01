package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `// Package thing is a sample.
package thing

import "context"

// Mode is how it runs.
type Mode string

const (
	// Fast is quick.
	Fast Mode = "fast"
	Slow Mode = "slow" // takes a while
)

// ErrGone means it left.
var ErrGone = error(nil)

// Options say how.
type Options struct {
	Mode Mode ` + "`json:\"mode\"`" + ` // how it runs
	hidden int
}

// Close stops it.
//
//nimbus:platforms iOS
func (o Options) Close() error { return nil }

// Start begins. It's filed under Options by go/doc, but belongs with the
// functions.
//
//nimbus:param o how to begin
//nimbus:returns the options in use
func Start(ctx context.Context, o Options) (Options, error) { return o, nil }

// Bare has an undescribed parameter.
func Bare(n int) {}
`

func write(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "thing")
	for name, s := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestReference(t *testing.T) {
	dir := write(t, map[string]string{
		"thing.go":        sample,
		"thing_test.go":   "package thing\n\nfunc Skipped() {}\n",
		"ios/Thing.swift": "",
		"plugin.json":     `{"platforms": {"ios": "device only"}, "ios": {"info": {"NSThing": "Why"}}, "android": {"dependencies": ["a:b:1"]}}`,
	})
	p, err := load(dir, "thing")
	if err != nil {
		t.Fatal(err)
	}
	head, body := p.render()
	for _, want := range []string{
		`iOS <small>device only</small>`, `Not on Android`, `Not on the web`,
	} {
		if !strings.Contains(head, want) {
			t.Errorf("head has no %q:\n%s", want, head)
		}
	}
	for _, want := range []string{
		`import "github.com/CodeSyncr/nimbus-native/plugins/thing"`,
		`<h3 id="ref-start">Start</h3>`, // moved back from under Options
		`<td><code>o</code></td><td><code><a href="#ref-options">Options</a></code></td><td>How to begin</td>`,
		`: the options in use</p>`,
		`<td><code>Mode</code></td><td><code><a href="#ref-mode">Mode</a></code></td><td>How it runs</td>`,
		`<code>Fast</code>`, `Takes a while`, `<code>ErrGone</code>`,
		`<h4 id="ref-options-close">Close</h4>`, `Only on iOS`,
		`<code>NSThing</code>`, `Why`, `<code>a:b:1</code>`,
		`iOS`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body has no %q:\n%s", want, body)
		}
	}
	for _, not := range []string{"Skipped", "hidden", "ctx</code></td>", "nimbus:param"} {
		if strings.Contains(body, not) {
			t.Errorf("body has %q", not)
		}
	}
	if len(p.missing) != 1 || !strings.Contains(p.missing[0], "Bare: no description of n") {
		t.Errorf("missing = %v", p.missing)
	}
}

func TestViewSyntaxIsEscaped(t *testing.T) {
	if got := esc("map[string]int{{1}} @if"); strings.ContainsAny(got, "{}@") {
		t.Fatalf("esc = %q", got)
	}
}

func TestNamesAreCode(t *testing.T) {
	got := names("it ends with ctx.Err() when ctx ends, or plugin.Unavailable.")
	want := "it ends with <code>ctx.Err()</code> when <code>ctx</code> ends, or <code>plugin.Unavailable</code>."
	if got != want {
		t.Fatalf("names = %q", got)
	}
}

func TestExample(t *testing.T) {
	dir := write(t, map[string]string{"thing.go": "package guide\n\nvar x = map[string]int{}\n"})
	got, err := example(dir, "thing")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "package main") || strings.Contains(got, "package guide") || strings.Contains(got, "{}") {
		t.Fatalf("example = %s", got)
	}
	if _, err := example(dir, "other"); err == nil {
		t.Fatal("a missing example should fail")
	}
}

func TestTested(t *testing.T) {
	got := renderTested([]result{{"Android emulator", "fails with plugin.Unavailable"}})
	if !strings.Contains(got, "<td>Android emulator</td>") || !strings.Contains(got, "<code>plugin.Unavailable</code>") {
		t.Fatalf("tested = %s", got)
	}
}

func TestBuiltIn(t *testing.T) {
	p, err := load(write(t, map[string]string{"thing.go": "package thing\n"}), "thing")
	if err != nil {
		t.Fatal(err)
	}
	head, body := p.render()
	if strings.Contains(head, "Not on iOS") || strings.Contains(head, "Not on Android") || !strings.Contains(head, "Not on the web") {
		t.Errorf("head = %s", head)
	}
	if !strings.Contains(body, "built in") {
		t.Errorf("body = %s", body)
	}
}
