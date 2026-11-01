package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// components writes the reference of the component pages from package ui.
// list names, for each page, the types and functions it covers; a method is
// Type.Method, and is listed as a function.
func components(uiDir, list, examples, tested, out string) (failed bool, err error) {
	b, err := os.ReadFile(list)
	if err != nil {
		return false, err
	}
	var pages map[string][]string
	if err := json.Unmarshal(b, &pages); err != nil {
		return false, err
	}
	results, err := loadTested(tested)
	if err != nil {
		return false, err
	}
	for slug, names := range pages {
		p, err := load(uiDir, "ui")
		if err != nil {
			return false, err
		}
		p.pick(names)
		for _, w := range p.missing {
			fmt.Fprintf(os.Stderr, "docref: %s: %s\n", slug, w)
			failed = true
		}
		if results[slug] == nil {
			fmt.Fprintf(os.Stderr, "docref: %s: no test results\n", slug)
			failed = true
		}
		ex, err := example(examples, strings.ReplaceAll(slug, "-", "_"))
		if err != nil {
			return false, err
		}
		var ref strings.Builder
		ref.WriteString(`<div class="nd-ref">` + "\n" + `<h2 id="api">API</h2>` + "\n")
		ref.WriteString(`<pre><code class="language-go">import "github.com/CodeSyncr/nimbus-native/ui"</code></pre>` + "\n")
		p.reference(&ref)
		ref.WriteString("</div>\n")
		parts := map[string]string{"-head": componentHead(slug), "": ref.String(), "-example": ex, "-tested": renderTested(results[slug])}
		for suffix, text := range parts {
			name := filepath.Join(out, "c-"+slug+suffix+".nimbus")
			if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
				return false, err
			}
		}
	}
	return failed, nil
}

// noWeb are the components a browser can't do yet.
var noWeb = map[string]bool{"date-field": true}

func componentHead(slug string) string {
	web := `<span class="nd-plat">The web</span>`
	if noWeb[slug] {
		web = `<span class="nd-plat off">Not on the web</span>`
	}
	return `<p class="nd-plats"><span class="nd-plat">iOS</span><span class="nd-plat">Android</span>` + web + "</p>\n"
}

// pick keeps only what a page covers, in the order it's listed.
func (p *plug) pick(names []string) {
	funcs, types := []fn{}, []typ{}
	keep := map[string]bool{}
	for _, n := range names {
		keep[n] = true
		owner, method, isMethod := strings.Cut(n, ".")
		if isMethod {
			for _, t := range p.types {
				if t.name != owner {
					continue
				}
				for _, m := range t.methods {
					if m.name == method {
						m.sig = strings.Replace(m.sig, "func (o OverlayNav) ", "func (ui.OverlayNav) ", 1)
						funcs = append(funcs, m)
					}
				}
			}
			continue
		}
		for _, f := range p.funcs {
			if f.name == n {
				funcs = append(funcs, f)
			}
		}
		for _, t := range p.types {
			if t.name == n {
				types = append(types, t)
			}
		}
	}
	var missing []string
	for _, m := range p.missing {
		name, _, _ := strings.Cut(m, ":")
		for k := range keep {
			if name == k || strings.HasPrefix(name, k+".") {
				missing = append(missing, m)
				break
			}
		}
	}
	p.funcs, p.types, p.consts, p.vars, p.missing = funcs, types, nil, nil, missing
	p.local = map[string]bool{}
	for _, t := range types {
		p.local[t.name] = true
	}
}
