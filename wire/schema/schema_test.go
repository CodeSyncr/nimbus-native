package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func load(t *testing.T, src string) (*Schema, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s.go"), []byte("package proto\n\n"+src), 0o644); err != nil {
		t.Fatal(err)
	}
	return Load(dir)
}

func TestLoadErrors(t *testing.T) {
	const msg = "//wire:message\ntype M struct{ A int32 }\n"
	cases := map[string]struct{ src, want string }{
		"import":        {`import "time"` + "\n" + msg + "var _ time.Duration", "must not import"},
		"unexported":    {"//wire:message\ntype M struct{ a int32 }", "must be exported"},
		"embedded":      {"type E struct{ A int32 }\n//wire:message\ntype M struct{ E }", "embedded"},
		"no fields":     {"//wire:message\ntype M struct{}", "at least one field"},
		"no messages":   {"type M struct{ A int32 }", "no //wire:message"},
		"chan":          {"//wire:message\ntype M struct{ C chan int }", "cannot cross the wire"},
		"unknown type":  {"type x struct{ A int }\n//wire:message\ntype M struct{ X x }", "not a wire type"},
		"opt list":      {"//wire:message\ntype M struct{ L *[]int32 }", "optional"},
		"map key":       {"//wire:message\ntype M struct{ L map[float64]int32 }", "map keys"},
		"map of union":  {"//wire:union\ntype U interface{ isU() }\n//wire:variant U\ntype V struct{ A int32 }\n//wire:message\ntype M struct{ L map[string]U }", "map values can't be unions"},
		"self via ptr":  {"//wire:message\ntype M struct{ A int32; Next *M }", "contains itself"},
		"cycle":         {"type A struct{ B *B }\ntype B struct{ A *A }\n//wire:message\ntype M struct{ A A }", "contains itself"},
		"empty union":   {msg + "//wire:union\ntype U interface{ isU() }", "no //wire:variant"},
		"bad union":     {msg + "//wire:union\ntype U interface{ IsU() }", "one unexported method"},
		"bad variant":   {msg + "//wire:variant Nope\ntype V struct{ A int32 }", "no //wire:union named"},
		"reserved name": {msg + "type Message struct{ A int32 }", "reserved"},
		"bad enum":      {msg + "//wire:enum\ntype E string", "named integer"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := load(t, c.src)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want an error containing %q", err, c.want)
			}
		})
	}
}

func TestRecursionThroughList(t *testing.T) {
	s, err := load(t, "//wire:message\ntype Node struct{ ID uint32; Children []*Node; Index map[string]Node }")
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Structs[0].Fields[1].Type; got.Kind != List || got.Elem.Kind != Struct || got.Elem.GoType != "*Node" {
		t.Fatalf("[]*Node loaded as %+v / %+v", got, got.Elem)
	}
}

// The hash covers what is on the wire — names, order, types — and nothing else.
func TestHash(t *testing.T) {
	base := "//wire:message\ntype M struct{ A int32; B string }\n"
	h := func(src string) uint64 {
		s, err := load(t, src)
		if err != nil {
			t.Fatal(err)
		}
		return s.Hash
	}
	a := h(base)
	if h("// Docs don't matter.\n"+base) != a {
		t.Error("a doc comment changed the hash")
	}
	for _, changed := range []string{
		"//wire:message\ntype M struct{ B string; A int32 }\n", // field order
		"//wire:message\ntype M struct{ A int64; B string }\n", // field type
		"//wire:message\ntype M struct{ A int32; C string }\n", // field name
		"//wire:message\ntype N struct{ A int32; B string }\n", // message name
		base + "//wire:message\ntype M2 struct{ A int32 }\n",   // another message
	} {
		if h(changed) == a {
			t.Errorf("hash unchanged for:\n%s", changed)
		}
	}
	if Combine(nil) != 0 || Combine([]uint64{1, 2}) != Combine([]uint64{2, 1, 2}) {
		t.Error("Combine must ignore order and duplicates, and be 0 for none")
	}
}
