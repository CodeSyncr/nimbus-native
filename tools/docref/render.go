package main

import (
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// safe keeps generated text from reading as view syntax.
var safe = strings.NewReplacer("{", "&#123;", "}", "&#125;", "@", "&#64;")

func esc(s string) string { return safe.Replace(html.EscapeString(s)) }

func code(s string) string { return "<code>" + esc(s) + "</code>" }

// prose turns a doc comment into HTML, with its code blocks shaped like the
// guide's.
func (p *plug) prose(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	pr := p.parsed.Printer()
	out := string(pr.HTML(p.parsed.Parser().Parse(s)))
	out = strings.ReplaceAll(out, "<pre>", `<pre><code class="language-go">`)
	out = strings.ReplaceAll(out, "</pre>", "</code></pre>")
	// Inside a code block, a name is already code.
	parts := strings.Split(safe.Replace(out), "</code></pre>")
	for i, part := range parts {
		text, block, _ := strings.Cut(part, "<pre>")
		parts[i] = names(text)
		if block != "" {
			parts[i] += "<pre>" + block
		}
	}
	return strings.Join(parts, "</code></pre>")
}

var name = regexp.MustCompile(`\b(?:plugin\.[A-Z]\w*|ctx\.Err\(\)|ctx\b)`)

// names marks what's code in a sentence.
func names(s string) string {
	return name.ReplaceAllStringFunc(s, func(n string) string { return "<code>" + n + "</code>" })
}

// sentence is a table cell from a comment that may start in lower case.
func sentence(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return names(esc(string(r)))
}

var word = regexp.MustCompile(`[A-Z][A-Za-z0-9]*`)

// typeRef is a type as code, linking any of the plugin's own types to their
// entry.
func (p *plug) typeRef(t string) string {
	e := esc(t)
	return "<code>" + word.ReplaceAllStringFunc(e, func(w string) string {
		if !p.local[w] {
			return w
		}
		return fmt.Sprintf(`<a href="#ref-%s">%s</a>`, strings.ToLower(w), w)
	}) + "</code>"
}

func table(b *strings.Builder, head []string, rows [][]string) {
	b.WriteString(`<div class="tbl"><table><thead><tr>`)
	for _, h := range head {
		b.WriteString("<th>" + h + "</th>")
	}
	b.WriteString("</tr></thead><tbody>")
	for _, r := range rows {
		b.WriteString("<tr>")
		for _, c := range r {
			b.WriteString("<td>" + c + "</td>")
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table></div>\n")
}

// platforms is the line under a page's title.
func (p *plug) platforms() string {
	one := func(name, key string, on bool) string {
		switch {
		case !on:
			return `<span class="nd-plat off">Not on ` + name + `</span>`
		case p.notes[key] != "":
			return `<span class="nd-plat">` + name + ` <small>` + esc(p.notes[key]) + `</small></span>`
		}
		return `<span class="nd-plat">` + name + `</span>`
	}
	return `<p class="nd-plats">` + one("iOS", "ios", p.ios) + one("Android", "android", p.android) + one("the web", "web", false) + "</p>\n"
}

func (p *plug) writeFn(b *strings.Builder, f fn, level, id string) {
	fmt.Fprintf(b, `<%s id="%s">%s</%s>`+"\n", level, id, esc(f.name), level)
	if f.platforms != "" {
		fmt.Fprintf(b, `<p class="nd-only">Only on %s</p>`+"\n", esc(f.platforms))
	}
	fmt.Fprintf(b, `<pre><code class="language-go">%s</code></pre>`+"\n", esc(f.sig))
	b.WriteString(p.prose(f.doc))
	if len(f.params) > 0 {
		rows := make([][]string, len(f.params))
		for i, a := range f.params {
			rows[i] = []string{code(a.name), p.typeRef(a.typ), sentence(a.desc)}
		}
		table(b, []string{"Parameter", "Type", "Description"}, rows)
	}
	if len(f.results) > 0 {
		kinds := make([]string, len(f.results))
		for i, r := range f.results {
			kinds[i] = p.typeRef(r)
		}
		line := `<p class="nd-returns"><b>Returns</b> ` + strings.Join(kinds, ", ")
		if f.returns != "" {
			line += ": " + names(esc(f.returns))
		}
		b.WriteString(line + "</p>\n")
	}
}

func (p *plug) values(b *strings.Builder, list []value) {
	hasValue := false
	for _, v := range list {
		hasValue = hasValue || v.val != ""
	}
	head := []string{"Name", "Value", "Description"}
	if !hasValue {
		head = []string{"Name", "Description"}
	}
	var rows [][]string
	for _, v := range list {
		r := []string{code(v.name)}
		if hasValue {
			r = append(r, code(v.val))
		}
		rows = append(rows, append(r, sentence(v.doc)))
	}
	table(b, head, rows)
}

// permissions says what `plugin add` changes in the app.
func (p *plug) permissions(b *strings.Builder) {
	var rows [][]string
	add := func(where, entry, says string) { rows = append(rows, []string{where, code(entry), says}) }
	m := p.man
	for _, k := range sorted(m.IOS.Info) {
		add("Info.plist", k, esc(m.IOS.Info[k]))
	}
	for _, k := range sorted(m.IOS.Arrays) {
		add("Info.plist", k, esc(strings.Join(m.IOS.Arrays[k], ", ")))
	}
	for _, k := range sorted(m.IOS.Entitlements) {
		add("Entitlements", k, esc(m.IOS.Entitlements[k]))
	}
	for _, k := range sorted(m.IOS.EntitlementArrays) {
		add("Entitlements", k, esc(strings.Join(m.IOS.EntitlementArrays[k], ", ")))
	}
	for _, s := range m.Android.Permissions {
		add("AndroidManifest.xml", s, "")
	}
	b.WriteString(`<h2 id="permissions">Permissions</h2>` + "\n")
	switch {
	case p.builtin:
		b.WriteString("<p>It's built in, and needs nothing from you.</p>\n")
	case len(rows) == 0:
		b.WriteString("<p>It needs none, on either platform.</p>\n")
	default:
		b.WriteString("<p><code>plugin add</code> writes these for you. Reword the reasons to fit your app: a later <code>add</code> keeps your words.</p>\n")
		table(b, []string{"Where", "Entry", "Says"}, rows)
		switch {
		case len(m.Android.Permissions) == 0:
			b.WriteString("<p>Android needs no permission for this.</p>\n")
		case len(m.IOS.Info)+len(m.IOS.Arrays)+len(m.IOS.Entitlements)+len(m.IOS.EntitlementArrays) == 0:
			b.WriteString("<p>iOS needs no permission for this.</p>\n")
		}
	}
	if len(m.Needs) > 0 {
		fmt.Fprintf(b, "<p>It takes %s when you add it.</p>\n", code("-"+strings.Join(m.Needs, ", -")))
	}
	var deps []string
	for _, s := range m.IOS.Packages {
		deps = append(deps, code(s.Package+" "+s.From))
	}
	for _, s := range m.Android.Dependencies {
		deps = append(deps, code(s))
	}
	if len(deps) > 0 {
		b.WriteString("<p>It brings in " + strings.Join(deps, ", ") + ".</p>\n")
	}
}

func sorted[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// render gives the platform line and the reference.
func (p *plug) render() (head, body string) {
	var b strings.Builder
	b.WriteString(`<div class="nd-ref">` + "\n")
	b.WriteString(`<h2 id="api">API</h2>` + "\n")
	fmt.Fprintf(&b, `<pre><code class="language-go">import "%s%s"</code></pre>`+"\n", importRoot, p.name)
	if len(p.funcs) > 0 {
		b.WriteString(`<h2 id="functions">Functions</h2>` + "\n")
		for _, f := range p.funcs {
			if strings.Contains(f.sig, "context.Context") {
				b.WriteString("<p>A <code>ctx</code> comes first. Cancel it to give up on the call.</p>\n")
				break
			}
		}
		for _, f := range p.funcs {
			p.writeFn(&b, f, "h3", "ref-"+strings.ToLower(f.name))
		}
	}
	if len(p.types) > 0 {
		b.WriteString(`<h2 id="types">Types</h2>` + "\n")
		for _, t := range p.types {
			id := "ref-" + strings.ToLower(t.name)
			fmt.Fprintf(&b, `<h3 id="%s">%s</h3>`+"\n", id, esc(t.name))
			if t.decl != "" {
				fmt.Fprintf(&b, `<pre><code class="language-go">%s</code></pre>`+"\n", esc(t.decl))
			}
			b.WriteString(p.prose(t.doc))
			if len(t.fields) > 0 {
				rows := make([][]string, len(t.fields))
				for i, f := range t.fields {
					rows[i] = []string{code(f.name), p.typeRef(f.typ), sentence(f.doc)}
				}
				table(&b, []string{"Field", "Type", "Description"}, rows)
			}
			if len(t.consts) > 0 {
				p.values(&b, t.consts)
			}
			for _, m := range t.methods {
				p.writeFn(&b, m, "h4", id+"-"+strings.ToLower(m.name))
			}
		}
	}
	if len(p.consts) > 0 {
		b.WriteString(`<h2 id="constants">Constants</h2>` + "\n")
		p.values(&b, p.consts)
	}
	if len(p.vars) > 0 {
		b.WriteString(`<h2 id="variables">Errors and variables</h2>` + "\n")
		p.values(&b, p.vars)
	}
	p.permissions(&b)
	b.WriteString("</div>\n")
	return p.platforms(), b.String()
}
