package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const importRoot = "github.com/CodeSyncr/nimbus-native/plugins/"

// plug is one plugin, read.
type plug struct {
	name    string
	ios     bool
	android bool
	builtin bool
	notes   map[string]string // from plugin.json's "platforms", such as "device only"
	man     manifest
	funcs   []fn
	types   []typ
	consts  []value
	vars    []value
	parsed  *doc.Package
	local   map[string]bool // its exported types, for links
	missing []string
}

// manifest is the part of plugin.json a reader needs to know about.
type manifest struct {
	Platforms map[string]string `json:"platforms"`
	Needs     []string          `json:"needs"`
	IOS       struct {
		Info              map[string]string   `json:"info"`
		Arrays            map[string][]string `json:"arrays"`
		Entitlements      map[string]string   `json:"entitlements"`
		EntitlementArrays map[string][]string `json:"entitlementArrays"`
		Packages          []struct {
			Package string `json:"package"`
			From    string `json:"from"`
		} `json:"packages"`
	} `json:"ios"`
	Android struct {
		Permissions  []string `json:"permissions"`
		Dependencies []string `json:"dependencies"`
	} `json:"android"`
}

type param struct{ name, typ, desc string }

// fn is a function or a method.
type fn struct {
	name, sig, doc string
	params         []param
	results        []string
	returns        string
	platforms      string
}

type field struct{ name, typ, doc string }

type typ struct {
	name, doc, decl string
	fields          []field
	consts          []value
	methods         []fn
}

type value struct{ name, val, doc string }

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func load(dir, name string) (*plug, error) {
	p := &plug{name: name, ios: exists(filepath.Join(dir, "ios")), android: exists(filepath.Join(dir, "android")), local: map[string]bool{}}
	// With no sources of its own, it ships in the kit: every app has it.
	if !p.ios && !p.android {
		p.ios, p.android, p.builtin = true, true, true
	}
	if b, err := os.ReadFile(filepath.Join(dir, "plugin.json")); err == nil {
		if err := json.Unmarshal(b, &p.man); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}
	p.notes = p.man.Platforms

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(f os.FileInfo) bool {
		return !strings.HasSuffix(f.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var files []*ast.File
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			files = append(files, f)
		}
	}
	sort.Slice(files, func(i, j int) bool { return fset.File(files[i].Pos()).Name() < fset.File(files[j].Pos()).Name() })
	dp, err := doc.NewFromFiles(fset, files, importRoot+name, doc.PreserveAST)
	if err != nil {
		return nil, err
	}
	p.parsed = dp

	for _, t := range dp.Types {
		p.local[t.Name] = true
	}
	for _, f := range dp.Funcs {
		p.funcs = append(p.funcs, p.function(fset, f, ""))
	}
	for _, t := range dp.Types {
		// go/doc files a function that returns a type under that type;
		// a reader looks for it among the functions.
		for _, f := range t.Funcs {
			p.funcs = append(p.funcs, p.function(fset, f, ""))
		}
		p.types = append(p.types, p.typeOf(fset, t))
	}
	sort.Slice(p.funcs, func(i, j int) bool { return p.funcs[i].name < p.funcs[j].name })
	for _, c := range dp.Consts {
		p.consts = append(p.consts, values(fset, c)...)
	}
	for _, v := range dp.Vars {
		p.vars = append(p.vars, values(fset, v)...)
	}
	return p, nil
}

func source(fset *token.FileSet, n any) string {
	var b bytes.Buffer
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	if err := cfg.Fprint(&b, fset, n); err != nil {
		return ""
	}
	return b.String()
}

func (p *plug) function(fset *token.FileSet, f *doc.Func, owner string) fn {
	sig := *f.Decl
	sig.Doc, sig.Body = nil, nil
	out := fn{name: f.Name, sig: source(fset, &sig), doc: f.Doc}
	descs := map[string]string{}
	if f.Decl.Doc != nil {
		for _, c := range f.Decl.Doc.List {
			line, ok := strings.CutPrefix(c.Text, "//nimbus:")
			if !ok {
				continue
			}
			key, rest, _ := strings.Cut(line, " ")
			switch key {
			case "param":
				n, d, _ := strings.Cut(rest, " ")
				descs[n] = d
			case "returns":
				out.returns = rest
			case "platforms":
				out.platforms = rest
			}
		}
	}
	for _, list := range f.Decl.Type.Params.List {
		t := source(fset, list.Type)
		for _, n := range list.Names {
			// Every call takes a ctx, and the page explains it once. A
			// component's c is just what it was given.
			if t == "context.Context" || t == "*Ctx" {
				continue
			}
			out.params = append(out.params, param{n.Name, t, descs[n.Name]})
			if descs[n.Name] == "" {
				p.missing = append(p.missing, fmt.Sprintf("%s%s: no description of %s", owner, f.Name, n.Name))
			}
		}
	}
	if f.Decl.Type.Results != nil {
		for _, r := range f.Decl.Type.Results.List {
			n := len(r.Names)
			if n == 0 {
				n = 1
			}
			for i := 0; i < n; i++ {
				out.results = append(out.results, source(fset, r.Type))
			}
		}
	}
	return out
}

func (p *plug) typeOf(fset *token.FileSet, t *doc.Type) typ {
	out := typ{name: t.Name, doc: t.Doc}
	for _, spec := range t.Decl.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok || ts.Name.Name != t.Name {
			continue
		}
		if st, ok := ts.Type.(*ast.StructType); ok {
			for _, f := range st.Fields.List {
				for _, n := range f.Names {
					if ast.IsExported(n.Name) {
						out.fields = append(out.fields, field{n.Name, source(fset, f.Type), comment(f)})
						if comment(f) == "" {
							p.missing = append(p.missing, fmt.Sprintf("%s.%s: no description", t.Name, n.Name))
						}
					}
				}
				if len(f.Names) == 0 { // embedded
					out.fields = append(out.fields, field{source(fset, f.Type), source(fset, f.Type), comment(f)})
				}
			}
		} else {
			out.decl = "type " + t.Name + " " + source(fset, ts.Type)
		}
	}
	for _, c := range t.Consts {
		out.consts = append(out.consts, values(fset, c)...)
	}
	for _, m := range t.Methods {
		// Render is how a component works, not something to call.
		if ast.IsExported(m.Name) && m.Name != "Render" {
			out.methods = append(out.methods, p.function(fset, m, t.Name+"."))
		}
	}
	return out
}

func comment(f *ast.Field) string {
	if f.Doc != nil {
		return strings.TrimSpace(f.Doc.Text())
	}
	if f.Comment != nil {
		return strings.TrimSpace(f.Comment.Text())
	}
	return ""
}

// values lists a const or var block. A comment on the block goes to any name
// with none of its own.
func values(fset *token.FileSet, v *doc.Value) []value {
	var out []value
	for _, spec := range v.Decl.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		d := ""
		if vs.Doc != nil {
			d = vs.Doc.Text()
		} else if vs.Comment != nil {
			d = vs.Comment.Text()
		} else if len(v.Decl.Specs) == 1 {
			d = v.Doc
		}
		for i, n := range vs.Names {
			if !ast.IsExported(n.Name) {
				continue
			}
			val := ""
			if i < len(vs.Values) {
				val = source(fset, vs.Values[i])
			}
			out = append(out, value{n.Name, val, strings.TrimSpace(d)})
		}
	}
	return out
}
