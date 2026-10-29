package gen_test

import (
	"bytes"
	"encoding/binary"
	"flag"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/CodeSyncr/nimbus-native/wire/gen"
	"github.com/CodeSyncr/nimbus-native/wire/internal/kitchen"
)

var update = flag.Bool("update", false, "rewrite wire/internal/kitchen/wire_gen.go")

const kitchenDir = "../internal/kitchen"

// The committed generated code must match what the generator produces now.
func TestKitchenUpToDate(t *testing.T) {
	out, err := gen.Generate(kitchenDir, "test.wire")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(kitchenDir, gen.GoFile)
	if *update {
		if err := os.WriteFile(path, out.Go, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	have, _ := os.ReadFile(path)
	if !bytes.Equal(have, out.Go) {
		t.Fatalf("%s is stale; run go test ./wire/gen -run KitchenUpToDate -update", path)
	}
	if out.Schema.Hash != kitchen.SchemaHash {
		t.Fatalf("schema hash %#x, generated code says %#x", out.Schema.Hash, kitchen.SchemaHash)
	}
}

func fixtures() []kitchen.Message {
	u32, s, f := uint32(7), "maybe 🌧", -0.0
	k := kitchen.Kind(77) // not a declared value: enums are open
	return []kitchen.Message{
		&kitchen.Scalars{},
		&kitchen.Scalars{
			B: true, U8: math.MaxUint8, U16: math.MaxUint16, U32: math.MaxUint32, U64: math.MaxUint64,
			I8: math.MinInt8, I16: math.MinInt16, I32: math.MinInt32, I64: math.MinInt64,
			I: math.MaxInt64, U: 1 << 63,
			F32: math.Float32frombits(0x7fc00001), // NaN with a payload
			F64: math.Inf(-1), S: "héllo, 世界 🌍", Raw: []byte{0, 1, 2, 0xff},
		},
		&kitchen.Scalars{I8: math.MaxInt8, I16: math.MaxInt16, I32: math.MaxInt32, I64: math.MaxInt64,
			F32: float32(math.SmallestNonzeroFloat32), F64: math.Float64frombits(1 << 63)}, // -0
		&kitchen.Collections{
			Names:  []string{"", "a", "ü", "🌧"},
			Matrix: [][]int32{{}, {1, -1}, {math.MinInt32, math.MaxInt32}},
			// Keys sort by UTF-8 bytes on every side: "Z" < "a" < "é" < "😀"
			Scores:   map[string]float64{"a": 1.5, "Z": -2, "é": math.NaN(), "😀": math.MaxFloat64, "": 0},
			ByID:     map[uint64]string{0: "zero", 1 << 63: "big", math.MaxUint64: "max", 5: "five"},
			Signed:   map[int32]bool{-3: true, 0: false, 9: true},
			Nested:   map[string][]uint16{"x": {1, 2, 65535}, "w": nil},
			MaybeU32: &u32, MaybeS: &s, MaybeF: &f,
			Kinds:  []kitchen.Kind{kitchen.KindText, kitchen.KindBigText, 200},
			MaybeK: &k, Level: kitchen.LevelLow,
		},
		&kitchen.Collections{Level: kitchen.Level(-32768)},
		&kitchen.Node{
			ID: 1, Kind: kitchen.KindImage, Label: "root", Props: map[string]string{"b": "2", "a": "1"},
			Default: "keyword",
			Meta:    &kitchen.Meta{Created: -1, Tags: []string{"t"}},
			Children: []*kitchen.Node{
				{ID: 2, Label: "leaf"},
				{ID: 3, Children: []*kitchen.Node{{ID: 4, Meta: &kitchen.Meta{}}}},
			},
		},
		&kitchen.Batch{
			Seq: 42,
			Ops: []kitchen.Op{
				&kitchen.Create{Parent: 1, Node: kitchen.Node{ID: 9, Label: "new"}},
				nil,
				&kitchen.Remove{ID: 3},
			},
			Current: &kitchen.Remove{ID: 8},
		},
		&kitchen.Batch{},
	}
}

func encoded(t *testing.T) [][]byte {
	var out [][]byte
	for _, m := range fixtures() {
		out = append(out, kitchen.Encode(m))
	}
	return out
}

func TestGoRoundTrip(t *testing.T) {
	for i, b := range encoded(t) {
		m, err := kitchen.Decode(b)
		if err != nil {
			t.Fatalf("fixture %d: %v", i, err)
		}
		if again := kitchen.Encode(m); !bytes.Equal(again, b) {
			t.Fatalf("fixture %d: re-encoding differs\n have %x\n want %x", i, again, b)
		}
	}
}

// Corrupt input must produce an error, never a panic or a hang.
func TestGoCorrupt(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i, b := range encoded(t) {
		for n := 0; n < len(b); n++ {
			if _, err := kitchen.Decode(b[:n]); err == nil && n < len(b) {
				// A prefix can be a valid message only if it ends exactly at a
				// field boundary of a shorter message — never for these fixtures.
				t.Fatalf("fixture %d: truncated to %d/%d bytes decoded without error", i, n, len(b))
			}
		}
		if _, err := kitchen.Decode(append(append([]byte{}, b...), 0)); err == nil {
			t.Fatalf("fixture %d: trailing byte accepted", i)
		}
		for j := 0; j < 200; j++ {
			c := append([]byte{}, b...)
			c[rng.Intn(len(c))] ^= byte(1 << rng.Intn(8))
			kitchen.Decode(c) // must not panic
		}
	}
}

// cases is what the Swift and Java hosts get: every fixture, plus corrupt
// variants, with the result Go itself produces for each.
type testCase struct {
	in   []byte
	ok   bool
	want []byte // the re-encoding when ok
}

func cases(t *testing.T) []testCase {
	var out []testCase
	add := func(b []byte) {
		m, err := kitchen.Decode(b)
		c := testCase{in: b, ok: err == nil}
		if c.ok {
			c.want = kitchen.Encode(m)
		}
		out = append(out, c)
	}
	for _, b := range encoded(t) {
		add(b)
		add(b[:len(b)/2])
		add(b[:len(b)-1])
		add(append(append([]byte{}, b...), 0))
	}
	add([]byte{99})                     // unknown message id
	add([]byte{0})                      // "no message"
	add([]byte{1, 2})                   // bool 2
	add([]byte{4, 1, 0xff, 0xff, 0x03}) // Batch whose op count runs past the end
	add([]byte{1, 0, 0xff, 0x01})       // Scalars cut off right after a valid uint8 (255)
	add([]byte{1, 0, 0x80, 0x02})       // Scalars with uint8 = 256: out of range
	return out
}

func writeCases(t *testing.T, path string, cs []testCase) {
	var b []byte
	for _, c := range cs {
		b = binary.LittleEndian.AppendUint32(b, uint32(len(c.in)))
		b = append(b, c.in...)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func checkResults(t *testing.T, lang, path string, cs []testCase) {
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range cs {
		if len(out) < 5 {
			t.Fatalf("%s: output ends at case %d", lang, i)
		}
		ok, n := out[0] == 1, int(binary.LittleEndian.Uint32(out[1:5]))
		got := out[5 : 5+n]
		out = out[5+n:]
		switch {
		case ok != c.ok:
			t.Errorf("%s case %d: decoded=%v, Go says %v (input %x)", lang, i, ok, c.ok, c.in)
		case ok && !bytes.Equal(got, c.want):
			t.Errorf("%s case %d: re-encoding differs from Go\n %s %x\n go    %x", lang, i, lang, got, c.want)
		}
	}
	if len(out) != 0 {
		t.Errorf("%s: %d extra output bytes", lang, len(out))
	}
}

func generate(t *testing.T) *gen.Output {
	out, err := gen.Generate(kitchenDir, "test.wire")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func repo(p string) string { return filepath.Join("..", "..", p) }

func TestSwiftConformance(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("swiftc runs on macOS")
	}
	if _, err := exec.LookPath("swiftc"); err != nil {
		t.Skip("no swiftc")
	}
	dir := t.TempDir()
	out := generate(t)
	must(t, os.WriteFile(filepath.Join(dir, "Kitchen.swift"), out.Swift, 0o644))
	bin := filepath.Join(dir, "conformance")
	run(t, "swiftc", "-O", "-o", bin,
		repo("hosts/ios/Sources/NimbusNativeKit/Wire.swift"), filepath.Join(dir, "Kitchen.swift"), "testdata/conformance/main.swift")

	cs := cases(t)
	in, res := filepath.Join(dir, "in.bin"), filepath.Join(dir, "out.bin")
	writeCases(t, in, cs)
	run(t, bin, in, res)
	checkResults(t, "swift", res, cs)
}

func TestJavaConformance(t *testing.T) {
	for _, tool := range []string{"javac", "java"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
	dir := t.TempDir()
	out := generate(t)
	src := filepath.Join(dir, "src")
	must(t, os.MkdirAll(src, 0o755))
	must(t, os.WriteFile(filepath.Join(src, "Kitchen.java"), out.Java, 0o644))
	classes := filepath.Join(dir, "classes")
	run(t, "javac", "--release", "11", "-Xlint:all", "-Werror", "-d", classes,
		repo("hosts/android/src/space/nimbusgo/nativekit/WireReader.java"),
		repo("hosts/android/src/space/nimbusgo/nativekit/WireWriter.java"),
		repo("hosts/android/src/space/nimbusgo/nativekit/WireException.java"),
		filepath.Join(src, "Kitchen.java"), "testdata/conformance/Main.java")

	cs := cases(t)
	in, res := filepath.Join(dir, "in.bin"), filepath.Join(dir, "out.bin")
	writeCases(t, in, cs)
	run(t, "java", "-cp", classes, "Main", in, res)
	checkResults(t, "java", res, cs)
}

func run(t *testing.T, name string, args ...string) {
	t.Helper()
	if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", name, err, out)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
