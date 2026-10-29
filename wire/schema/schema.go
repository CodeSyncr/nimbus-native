// Package schema reads a wire schema: a Go package of plain types marked
// with //wire: directives. It is what the generators in wire/gen consume.
//
//	//wire:message           on a struct: a top-level message (gets an id)
//	//wire:union             on an interface with one unexported method:
//	                         a value that is one of several structs
//	//wire:variant <Union>   on a struct: one of the union's variants
//	//wire:enum              on a named integer type: its constants are the values
//
// Every other exported struct in the package is a plain nested type. The
// schema package must stand alone (no imports) so it can be type-checked
// without a build.
package schema

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CodeSyncr/nimbus-native/wire"
)

// Kind is a wire type's shape.
type Kind int

const (
	Bool Kind = iota
	Uint      // Bits: 8, 16, 32, 64
	Int       // Bits: 8, 16, 32, 64
	Float32
	Float64
	String
	Bytes
	List     // Elem
	Map      // Key, Elem
	Optional // Elem: a basic, string, enum or struct
	Struct   // Name
	Union    // Name; always nullable
	Enum     // Name; Bits and Signed from its base type
)

// Type is a field's type.
type Type struct {
	Kind   Kind
	Bits   int
	Signed bool // for Enum
	Name   string
	Elem   *Type
	Key    *Type
	GoType string // the Go spelling, e.g. "[]*Child"
}

type Field struct {
	Name string // Go name, exported
	Type *Type
}

type StructDef struct {
	Name    string
	Doc     string
	Fields  []Field
	Message bool // has //wire:message
}

type UnionDef struct {
	Name     string
	Doc      string
	Marker   string   // the interface's one method, which variants get
	Variants []string // struct names, in declaration order
}

type EnumValue struct {
	Name  string
	Value int64 // uint64 values are stored as their bit pattern
}

type EnumDef struct {
	Name   string
	Doc    string
	Bits   int
	Signed bool
	Values []EnumValue
}

// Schema is one package's wire types.
type Schema struct {
	Package  string // Go package name, e.g. "proto"
	Dir      string
	Structs  []*StructDef
	Unions   []*UnionDef
	Enums    []*EnumDef
	Messages []string // message struct names; a message's id is its index + 1
	Hash     uint64   // FNV-1a of Canonical()
}

// MessageUnion is the name of the generated union over every message.
const MessageUnion = "Message"

// Load reads the schema package in dir. Generated files (*_gen.go) and
// tests are ignored.
func Load(dir string) (*Schema, error) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []*ast.File
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") || strings.HasSuffix(n, "_gen.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, n), nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		if len(f.Imports) > 0 {
			return nil, fmt.Errorf("%s: a schema package must not import anything", fset.Position(f.Imports[0].Pos()))
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s: no Go files", dir)
	}

	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check(files[0].Name.Name, fset, files, nil)
	if err != nil {
		return nil, err
	}

	// Directives live on the AST; collect them by type name, in source order.
	type decl struct {
		name string
		doc  string
		dirs []string
		pos  token.Pos
	}
	var decls []decl
	for _, f := range files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, s := range gd.Specs {
				ts := s.(*ast.TypeSpec)
				doc := ts.Doc
				if doc == nil && len(gd.Specs) == 1 {
					doc = gd.Doc
				}
				text, dirs := splitDoc(doc)
				decls = append(decls, decl{ts.Name.Name, text, dirs, ts.Pos()})
			}
		}
	}
	sort.SliceStable(decls, func(i, j int) bool {
		pi, pj := fset.Position(decls[i].pos), fset.Position(decls[j].pos)
		if pi.Filename != pj.Filename {
			return pi.Filename < pj.Filename
		}
		return pi.Offset < pj.Offset
	})

	s := &Schema{Package: pkg.Name(), Dir: dir}
	l := &loader{pkg: pkg, s: s, kinds: map[string]Kind{}}

	// First pass: classify every named type so fields can refer to any of them.
	variantOf := map[string][]string{} // union → variants in order
	for _, d := range decls {
		obj := pkg.Scope().Lookup(d.name)
		if obj == nil {
			continue
		}
		u := obj.Type().Underlying()
		switch {
		case has(d.dirs, "union"):
			iface, ok := u.(*types.Interface)
			if !ok || iface.NumMethods() != 1 || iface.Method(0).Exported() {
				return nil, fmt.Errorf("%s: //wire:union needs an interface with exactly one unexported method", d.name)
			}
			l.kinds[d.name] = Union
			s.Unions = append(s.Unions, &UnionDef{Name: d.name, Doc: d.doc, Marker: iface.Method(0).Name()})
		case has(d.dirs, "enum"):
			b, ok := u.(*types.Basic)
			if !ok || b.Info()&types.IsInteger == 0 {
				return nil, fmt.Errorf("%s: //wire:enum needs a named integer type", d.name)
			}
			bits, signed := intShape(b)
			l.kinds[d.name] = Enum
			s.Enums = append(s.Enums, &EnumDef{Name: d.name, Doc: d.doc, Bits: bits, Signed: signed})
		default:
			if _, ok := u.(*types.Struct); ok && obj.Exported() {
				l.kinds[d.name] = Struct
			}
		}
		for _, dir := range d.dirs {
			if v, ok := strings.CutPrefix(dir, "variant "); ok {
				variantOf[strings.TrimSpace(v)] = append(variantOf[strings.TrimSpace(v)], d.name)
			}
		}
	}
	if _, clash := l.kinds[MessageUnion]; clash {
		return nil, fmt.Errorf("%s: the name %q is reserved for the generated message union", dir, MessageUnion)
	}

	// Second pass: fields, variants, enum values, messages.
	for _, d := range decls {
		if l.kinds[d.name] != Struct {
			continue
		}
		obj := pkg.Scope().Lookup(d.name)
		if _, ok := obj.Type().Underlying().(*types.Struct); !ok {
			continue
		}
		st := obj.Type().Underlying().(*types.Struct)
		def := &StructDef{Name: d.name, Doc: d.doc, Message: has(d.dirs, "message")}
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			switch {
			case f.Embedded():
				return nil, fmt.Errorf("%s.%s: embedded fields are not supported", d.name, f.Name())
			case !f.Exported():
				return nil, fmt.Errorf("%s.%s: fields must be exported", d.name, f.Name())
			}
			t, err := l.typeOf(f.Type())
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", d.name, f.Name(), err)
			}
			def.Fields = append(def.Fields, Field{Name: f.Name(), Type: t})
		}
		if len(def.Fields) == 0 {
			return nil, fmt.Errorf("%s: a wire struct needs at least one field", d.name)
		}
		s.Structs = append(s.Structs, def)
		if def.Message {
			s.Messages = append(s.Messages, def.Name)
		}
	}
	for _, u := range s.Unions {
		u.Variants = variantOf[u.Name]
		delete(variantOf, u.Name)
		if len(u.Variants) == 0 {
			return nil, fmt.Errorf("union %s has no //wire:variant structs", u.Name)
		}
		for _, v := range u.Variants {
			if l.kinds[v] != Struct {
				return nil, fmt.Errorf("%s: //wire:variant %s needs an exported struct", v, u.Name)
			}
		}
	}
	for name := range variantOf {
		return nil, fmt.Errorf("//wire:variant %s: no //wire:union named %s", name, name)
	}
	for _, e := range s.Enums {
		e.Values = enumValues(pkg, e.Name, fset)
	}
	if len(s.Messages) == 0 {
		return nil, fmt.Errorf("%s: no //wire:message structs", dir)
	}
	if err := s.check(); err != nil {
		return nil, err
	}
	s.Hash = hashString(s.Canonical())
	return s, nil
}

// check enforces the rules every target language can honour:
//   - a struct may contain itself only through a list or map (Swift value
//     types can't nest themselves through an optional), and
//   - map values are not unions (a nil union can't be a Swift dictionary value).
func (s *Schema) check() error {
	byName := map[string]*StructDef{}
	for _, st := range s.Structs {
		byName[st.Name] = st
	}
	var walk func(t *Type, fn func(*Type))
	walk = func(t *Type, fn func(*Type)) {
		fn(t)
		if t.Elem != nil {
			walk(t.Elem, fn)
		}
	}
	for _, st := range s.Structs {
		for _, f := range st.Fields {
			var err error
			walk(f.Type, func(t *Type) {
				if t.Kind == Map && t.Elem.Kind == Union && err == nil {
					err = fmt.Errorf("%s.%s: map values can't be unions; wrap the union in a struct", st.Name, f.Name)
				}
			})
			if err != nil {
				return err
			}
		}
	}

	// Direct containment: Struct fields and Optional(Struct) fields.
	const (
		unseen = iota
		active
		done
	)
	state := map[string]int{}
	var visit func(name string, path []string) error
	visit = func(name string, path []string) error {
		switch state[name] {
		case active:
			return fmt.Errorf("%s contains itself (%s); recursion must go through a list, e.g. []*%s",
				name, strings.Join(append(path, name), " → "), name)
		case done:
			return nil
		}
		state[name] = active
		for _, f := range byName[name].Fields {
			t := f.Type
			if t.Kind == Optional {
				t = t.Elem
			}
			if t.Kind == Struct {
				if err := visit(t.Name, append(path, name+"."+f.Name)); err != nil {
					return err
				}
			}
		}
		state[name] = done
		return nil
	}
	for _, st := range s.Structs {
		if err := visit(st.Name, nil); err != nil {
			return err
		}
	}
	return nil
}

type loader struct {
	pkg   *types.Package
	s     *Schema
	kinds map[string]Kind
}

func (l *loader) typeOf(t types.Type) (*Type, error) {
	spell := types.TypeString(t, types.RelativeTo(l.pkg))
	switch t := t.(type) {
	case *types.Named:
		name := t.Obj().Name()
		if t.Obj().Pkg() != l.pkg {
			return nil, fmt.Errorf("type %s is from another package", spell)
		}
		k, ok := l.kinds[name]
		if !ok {
			return nil, fmt.Errorf("type %s is not a wire type (exported struct, //wire:union or //wire:enum)", name)
		}
		out := &Type{Kind: k, Name: name, GoType: spell}
		if k == Enum {
			out.Bits, out.Signed = intShape(t.Underlying().(*types.Basic))
		}
		return out, nil
	case *types.Basic:
		switch {
		case t.Kind() == types.Bool:
			return &Type{Kind: Bool, GoType: spell}, nil
		case t.Kind() == types.String:
			return &Type{Kind: String, GoType: spell}, nil
		case t.Kind() == types.Float32:
			return &Type{Kind: Float32, GoType: spell}, nil
		case t.Kind() == types.Float64:
			return &Type{Kind: Float64, GoType: spell}, nil
		case t.Info()&types.IsInteger != 0 && t.Kind() != types.Uintptr:
			bits, signed := intShape(t)
			k := Uint
			if signed {
				k = Int
			}
			return &Type{Kind: k, Bits: bits, GoType: spell}, nil
		}
	case *types.Slice:
		if b, ok := t.Elem().(*types.Basic); ok && b.Kind() == types.Byte {
			return &Type{Kind: Bytes, GoType: spell}, nil
		}
		elem, err := l.typeOf(t.Elem())
		if err != nil {
			return nil, err
		}
		if elem.Kind == Optional && elem.Elem.Kind == Struct {
			// []*T is the natural Go spelling for a list of structs; elements
			// are never nil on the wire.
			elem = &Type{Kind: Struct, Name: elem.Elem.Name, GoType: elem.GoType}
		}
		return &Type{Kind: List, Elem: elem, GoType: spell}, nil
	case *types.Map:
		key, err := l.typeOf(t.Key())
		if err != nil {
			return nil, err
		}
		if key.Kind != String && key.Kind != Uint && key.Kind != Int {
			return nil, fmt.Errorf("map keys must be strings or integers, not %s", key.GoType)
		}
		elem, err := l.typeOf(t.Elem())
		if err != nil {
			return nil, err
		}
		return &Type{Kind: Map, Key: key, Elem: elem, GoType: spell}, nil
	case *types.Pointer:
		elem, err := l.typeOf(t.Elem())
		if err != nil {
			return nil, err
		}
		switch elem.Kind {
		case List, Map, Bytes, Optional, Union:
			return nil, fmt.Errorf("optional %s is not supported (use the empty value, or a struct)", elem.GoType)
		}
		return &Type{Kind: Optional, Elem: elem, GoType: spell}, nil
	}
	return nil, fmt.Errorf("type %s cannot cross the wire", spell)
}

func intShape(b *types.Basic) (bits int, signed bool) {
	switch b.Kind() {
	case types.Int8:
		return 8, true
	case types.Int16:
		return 16, true
	case types.Int32:
		return 32, true
	case types.Int, types.Int64:
		return 64, true
	case types.Uint8:
		return 8, false
	case types.Uint16:
		return 16, false
	case types.Uint32:
		return 32, false
	default:
		return 64, false
	}
}

func enumValues(pkg *types.Package, typeName string, fset *token.FileSet) []EnumValue {
	type pv struct {
		v   EnumValue
		pos token.Position
	}
	var vals []pv
	for _, name := range pkg.Scope().Names() {
		c, ok := pkg.Scope().Lookup(name).(*types.Const)
		if !ok || !c.Exported() {
			continue
		}
		n, ok := c.Type().(*types.Named)
		if !ok || n.Obj().Name() != typeName {
			continue
		}
		var v int64
		if i, exact := constant.Int64Val(c.Val()); exact {
			v = i
		} else if u, exact := constant.Uint64Val(c.Val()); exact {
			v = int64(u)
		}
		vals = append(vals, pv{EnumValue{name, v}, fset.Position(c.Pos())})
	}
	sort.SliceStable(vals, func(i, j int) bool {
		if vals[i].pos.Filename != vals[j].pos.Filename {
			return vals[i].pos.Filename < vals[j].pos.Filename
		}
		return vals[i].pos.Offset < vals[j].pos.Offset
	})
	out := make([]EnumValue, len(vals))
	for i, p := range vals {
		out[i] = p.v
	}
	return out
}

func splitDoc(cg *ast.CommentGroup) (text string, dirs []string) {
	if cg == nil {
		return "", nil
	}
	var lines []string
	for _, c := range cg.List {
		if d, ok := strings.CutPrefix(c.Text, "//wire:"); ok {
			dirs = append(dirs, strings.TrimSpace(d))
			continue
		}
		lines = append(lines, strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(c.Text, "//"), " ")))
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), dirs
}

func has(dirs []string, d string) bool {
	for _, x := range dirs {
		if x == d {
			return true
		}
	}
	return false
}

// Canonical is a stable text form of everything that affects the bytes on
// the wire; Hash is computed from it. Docs and Go spellings are left out.
func (s *Schema) Canonical() string {
	var b strings.Builder
	for i, m := range s.Messages {
		fmt.Fprintf(&b, "message %d %s\n", i+1, m)
	}
	for _, st := range s.Structs {
		fmt.Fprintf(&b, "struct %s {", st.Name)
		for _, f := range st.Fields {
			fmt.Fprintf(&b, " %s %s;", f.Name, f.Type.wireName())
		}
		b.WriteString(" }\n")
	}
	for _, u := range s.Unions {
		fmt.Fprintf(&b, "union %s { %s }\n", u.Name, strings.Join(u.Variants, " "))
	}
	for _, e := range s.Enums {
		fmt.Fprintf(&b, "enum %s %s {", e.Name, (&Type{Kind: Enum, Bits: e.Bits, Signed: e.Signed}).baseName())
		for _, v := range e.Values {
			fmt.Fprintf(&b, " %s=%d;", v.Name, v.Value)
		}
		b.WriteString(" }\n")
	}
	return b.String()
}

func (t *Type) wireName() string {
	switch t.Kind {
	case Bool:
		return "bool"
	case Uint:
		return fmt.Sprintf("uint%d", t.Bits)
	case Int:
		return fmt.Sprintf("int%d", t.Bits)
	case Float32:
		return "float32"
	case Float64:
		return "float64"
	case String:
		return "string"
	case Bytes:
		return "bytes"
	case List:
		return "[]" + t.Elem.wireName()
	case Map:
		return "map[" + t.Key.wireName() + "]" + t.Elem.wireName()
	case Optional:
		return "?" + t.Elem.wireName()
	}
	return t.Name
}

func (t *Type) baseName() string {
	if t.Signed {
		return fmt.Sprintf("int%d", t.Bits)
	}
	return fmt.Sprintf("uint%d", t.Bits)
}

func hashString(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// Combine is wire.Combine, the value hosts pass to nn_start.
func Combine(hashes []uint64) uint64 { return wire.Combine(hashes) }
