package scene

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 255 / w), uint8(y * 255 / h), 128, 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

func TestWire(t *testing.T) {
	th := ui.Theme{Primary: ui.Hex("#0d7377"), Background: ui.Hex("#fafafa")}
	tex, err := NewTexture(pngBytes(t, 8, 4))
	if err != nil || tex.Width != 8 || tex.Height != 4 {
		t.Fatalf("%v %v", tex, err)
	}
	m := Mat4(translate(V(0, 5, 0)))
	v := View{Shadows: true, Orbit: true, Nodes: []Node{
		{ID: "a", Mesh: Box(1, 1, 1), Spin: Spin{Speed: 30}, Texture: tex},
		{ID: "b", Mesh: Box(1, 1, 1), Position: V(2, 0, 0), Color: ui.Hex("#ff000080"), Shine: 3, NoShadow: true, DoubleSided: true,
			Children: []Node{{ID: "c", Mesh: Sphere(0.2), Position: V(0, 1, 0)}}},
		{ID: "d", Mesh: Plane(1, 1), Matrix: &m, Unlit: true},
	}}
	w := v.wire(th)
	if len(w.Nodes) != 4 || w.Background != uint32(th.Background) || w.Ambient != 0.25 || len(w.Lights) != 1 || w.FOV != 60 || !w.Orbit {
		t.Fatalf("%+v", w)
	}
	a, b, c, d := w.Nodes[0], w.Nodes[1], w.Nodes[2], w.Nodes[3]
	if a.Color != rgba(ui.Hex("#ffffff")) || a.Texture == "" || a.Speed != 30 || !a.Cast {
		t.Fatalf("textured node %+v", a)
	}
	if b.Shine != 1 || b.Cast || !b.Double || b.Color[3] < 0.5 || b.Color[3] > 0.51 || b.Texture != "" {
		t.Fatalf("node b %+v", b)
	}
	if a.Mesh != b.Mesh {
		t.Fatal("the same box twice has two keys")
	}
	if p := apply(c.Base, Vec3{}); !nearV(p, V(2, 1, 0)) {
		t.Fatalf("child at %v", p)
	}
	if p := apply(mul(d.Base, d.Local), Vec3{}); !nearV(p, V(0, 5, 0)) || d.Cast {
		t.Fatalf("matrix node at %v, cast %v", p, d.Cast)
	}
	if w.Shadow == nil {
		t.Fatal("no shadow view")
	}
	// What the phone gets carries no mesh data: that's sent when asked.
	j, _ := json.Marshal(w)
	if len(j) > 4000 || strings.Contains(string(j), "vertices") {
		t.Fatalf("props are %d bytes", len(j))
	}
	if got := len((View{Lights: make([]Light, 6)}).wire(th).Lights); got != 4 {
		t.Fatalf("%d lights", got)
	}
}

func TestChildrenTurnWithSpin(t *testing.T) {
	v := View{Camera: Camera{Position: V(0, 0, 5)}, Nodes: []Node{
		{ID: "gift", Mesh: Box(1, 1, 1), Position: V(1, 0, 0), Spin: Spin{Speed: 20},
			Children: []Node{{ID: "ribbon", Mesh: Box(1.1, 0.1, 1.1), Position: V(0, 0.2, 0)}}},
		{ID: "turntable", Position: V(-2, 0, 0), Spin: Spin{Speed: -10}, Children: []Node{
			{ID: "cup", Mesh: Sphere(0.3), Position: V(0.5, 0, 0)}}},
		{ID: "table", Mesh: Box(3, 0.1, 2), Position: V(0, -1, 0)},
	}}
	w := v.wire(ui.Theme{})
	if len(w.Nodes) != 5 {
		t.Fatalf("%d nodes", len(w.Nodes))
	}
	gift, ribbon, turntable, cup, table := w.Nodes[0], w.Nodes[1], w.Nodes[2], w.Nodes[3], w.Nodes[4]
	if gift.Parent != 0 || ribbon.Parent != 1 || turntable.Parent != 0 || cup.Parent != 3 || table.Parent != 0 {
		t.Fatalf("parents %d %d %d %d %d", gift.Parent, ribbon.Parent, turntable.Parent, cup.Parent, table.Parent)
	}
	// Within the node it turns with, a child's place is its own.
	if p := apply(ribbon.Base, Vec3{}); !nearV(p, V(0, 0.2, 0)) {
		t.Fatalf("ribbon at %v within the gift", p)
	}
	// A group that spins goes without a mesh, drawing and casting nothing.
	if turntable.Mesh != "" || turntable.Cast || turntable.Speed != -10 || cup.Mesh == "" {
		t.Fatalf("turntable %+v, cup %+v", turntable, cup)
	}
	// A tap meets what turns by the ball it sweeps around the spin's center.
	// x is where a point at depth 0 shows, across the view (60° FOV, 5 away).
	x := func(at float32) float32 { return 0.5 + at/(2*5*float32(math.Tan(math.Pi/6))) }
	if id := v.Hit(x(1.73), 0.5, 1); id != "gift" && id != "ribbon" { // past the box, inside its sweep
		t.Fatalf("tap beside the turning gift hit %q", id)
	}
	if id := v.Hit(x(-2.45), 0.5, 1); id != "cup" { // where the cup comes round to
		t.Fatalf("tap on the cup's circle hit %q", id)
	}
}

func TestTextureChecks(t *testing.T) {
	if _, err := NewTexture([]byte("not a picture")); err == nil {
		t.Error("junk passed")
	}
	if _, err := NewTexture(pngBytes(t, MaxTexture+1, 1)); err == nil {
		t.Error("too wide passed")
	}
}

// The shadow's view holds every node, in front of the light.
func TestShadowView(t *testing.T) {
	v := View{Nodes: []Node{
		{Mesh: Plane(10, 10), Position: V(0, -1, 0)},
		{Mesh: Box(1, 2, 1), Position: V(3, 0, -2), Spin: Spin{Speed: 10}},
	}}
	vp, ok := v.shadowView(normalize(V(-0.3, -1, -0.6)))
	if !ok {
		t.Fatal("no view")
	}
	for _, p := range []Vec3{V(-5, -1, -5), V(5, -1, 5), V(3.5, 1, -2.5), V(2.5, -1, -1.5)} {
		q := apply(vp, p)
		if q.X < -1 || q.X > 1 || q.Y < -1 || q.Y > 1 || q.Z < 0 || q.Z > 1 {
			t.Errorf("%v falls outside the shadow at %v", p, q)
		}
	}
	// Nearer the light is less deep.
	if apply(vp, V(0, 2, 0)).Z >= apply(vp, V(0, -1, 0)).Z {
		t.Error("depth runs the wrong way")
	}
	if _, ok := (View{}).shadowView(V(0, -1, 0)); ok {
		t.Error("an empty scene has a shadow view")
	}
}

func TestBoundsAndFit(t *testing.T) {
	n := Node{Position: V(10, 0, 0), Children: []Node{{Mesh: Box(2, 4, 2), Scale: V(2, 1, 1)}}}
	lo, hi := Bounds(n)
	if !nearV(lo, V(8, -2, -1)) || !nearV(hi, V(12, 2, 1)) {
		t.Fatalf("bounds %v %v", lo, hi)
	}
	f := Fit(n, 1)
	lo, hi = Bounds(f)
	if !nearV(lo, V(-0.5, -0.5, -0.25)) || !nearV(hi, V(0.5, 0.5, 0.25)) {
		t.Fatalf("fitted %v %v", lo, hi)
	}
}

func TestHitMatrix(t *testing.T) {
	m := Mat4(translate(V(0, 0, -1)))
	v := View{Nodes: []Node{{ID: "m", Mesh: Box(1, 1, 1), Matrix: &m}}}
	if v.Hit(0.5, 0.5, 1) != "m" || v.Hit(0.05, 0.05, 1) != "" {
		t.Fatal("matrix node hit wrong")
	}
}

// The phone asks for what it lacks, and gets each once.
func TestNeed(t *testing.T) {
	var mu sync.Mutex
	got := map[string]map[string]any{}
	old := call
	defer func() { call = old }()
	call = func(ctx context.Context, method string, args any) error {
		b, _ := json.Marshal(args)
		var m map[string]any
		json.Unmarshal(b, &m)
		mu.Lock()
		got[method+" "+m["key"].(string)] = m
		mu.Unlock()
		return nil
	}
	tex, _ := NewTexture(pngBytes(t, 2, 2))
	w := View{Nodes: []Node{{Mesh: Box(3, 3, 3), Texture: tex}}}.wire(ui.Theme{})
	n := w.Nodes[0]
	need{Keys: []string{n.Mesh, n.Texture, "nonsense"}}.send(context.Background())
	mesh, texture := got["mesh "+n.Mesh], got["texture "+n.Texture]
	if mesh == nil || texture == nil || len(got) != 2 {
		t.Fatalf("sent %v", got)
	}
	if mesh["count"].(float64) != 36 || len(mesh["vertices"].(string)) == 0 || len(texture["data"].(string)) == 0 {
		t.Fatalf("mesh %v", mesh)
	}
}

// A small .glb made here: an interleaved, indexed quad with a texture and a
// color, under a node turned by a quaternion, with a named child.
func glb(t *testing.T) []byte {
	t.Helper()
	var bin []byte
	f := func(fs ...float32) {
		for _, x := range fs {
			bin = binary.LittleEndian.AppendUint32(bin, math.Float32bits(x))
		}
	}
	// position, normal, uv; stride 32
	f(-1, -1, 0, 0, 0, 1, 0, 1)
	f(1, -1, 0, 0, 0, 1, 1, 1)
	f(1, 1, 0, 0, 0, 1, 1, 0)
	f(-1, 1, 0, 0, 0, 1, 0, 0)
	for _, i := range []uint16{0, 1, 2, 0, 2, 3} {
		bin = binary.LittleEndian.AppendUint16(bin, i)
	}
	imgAt := len(bin)
	pic := pngBytes(t, 4, 4)
	bin = append(bin, pic...)
	for len(bin)%4 != 0 {
		bin = append(bin, 0)
	}
	s := float32(math.Sqrt(0.5))
	doc := map[string]any{
		"asset": map[string]any{"version": "2.0"}, "scene": 0,
		"scenes": []any{map[string]any{"nodes": []int{0}}},
		"nodes": []any{
			map[string]any{"name": "turned", "rotation": []float32{0, s, 0, s}, "translation": []float32{0, 2, 0}, "children": []int{1}},
			map[string]any{"name": "quad", "mesh": 0, "scale": []float32{2, 2, 2}},
		},
		"meshes": []any{map[string]any{"primitives": []any{map[string]any{
			"attributes": map[string]int{"POSITION": 0, "NORMAL": 1, "TEXCOORD_0": 2}, "indices": 3, "material": 0}}}},
		"accessors": []any{
			map[string]any{"bufferView": 0, "byteOffset": 0, "componentType": 5126, "count": 4, "type": "VEC3"},
			map[string]any{"bufferView": 0, "byteOffset": 12, "componentType": 5126, "count": 4, "type": "VEC3"},
			map[string]any{"bufferView": 0, "byteOffset": 24, "componentType": 5126, "count": 4, "type": "VEC2"},
			map[string]any{"bufferView": 1, "componentType": 5123, "count": 6, "type": "SCALAR"},
		},
		"bufferViews": []any{
			map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 128, "byteStride": 32},
			map[string]any{"buffer": 0, "byteOffset": 128, "byteLength": 12},
			map[string]any{"buffer": 0, "byteOffset": imgAt, "byteLength": len(pic)},
		},
		"buffers":  []any{map[string]any{"byteLength": len(bin)}},
		"images":   []any{map[string]any{"bufferView": 2, "mimeType": "image/png"}},
		"textures": []any{map[string]any{"source": 0}},
		"materials": []any{map[string]any{"doubleSided": true, "pbrMetallicRoughness": map[string]any{
			"baseColorFactor": []float32{1, 0.2140, 0, 1}, "baseColorTexture": map[string]int{"index": 0}, "roughnessFactor": 0.5}}},
	}
	js, _ := json.Marshal(doc)
	for len(js)%4 != 0 {
		js = append(js, ' ')
	}
	out := binary.LittleEndian.AppendUint32(nil, glbMagic)
	out = binary.LittleEndian.AppendUint32(out, 2)
	out = binary.LittleEndian.AppendUint32(out, uint32(12+8+len(js)+8+len(bin)))
	out = binary.LittleEndian.AppendUint32(out, uint32(len(js)))
	out = binary.LittleEndian.AppendUint32(out, chunkJSON)
	out = append(out, js...)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(bin)))
	out = binary.LittleEndian.AppendUint32(out, chunkBIN)
	return append(out, bin...)
}

func TestLoadGLB(t *testing.T) {
	model, err := LoadGLTF(glb(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	var quad *Node
	walk([]Node{model}, identity(), nil, func(n Node, base, local Mat4, _ *Vec3) {
		if n.Mesh != nil {
			quad = &n
		}
	})
	if quad == nil || quad.ID != "quad" || quad.Texture == nil || !quad.DoubleSided || len(quad.Mesh.indices) != 6 {
		t.Fatalf("quad %+v", quad)
	}
	if quad.Color != ui.Hex("#ff8000ff") && quad.Color != ui.Hex("#ff7f00ff") {
		t.Fatalf("color %08x (linear 0.214 should be about sRGB 0.5)", uint32(quad.Color))
	}
	if !near(quad.Shine, 0.4) || quad.Mesh.uvs[2] != (UV{1, 0}) {
		t.Fatalf("shine %v uvs %v", quad.Shine, quad.Mesh.uvs)
	}
	// Turned 90° around Y and raised 2: the quad, scaled 2, stands at x=0 facing +X.
	lo, hi := Bounds(model)
	if !nearV(lo, V(0, 0, -2)) || !nearV(hi, V(0, 4, 2)) {
		t.Fatalf("bounds %v %v", lo, hi)
	}
	// Tapping it from the side finds it by name.
	v := View{Camera: Camera{Position: V(5, 2, 0), Target: V(0, 2, 0)}, Nodes: []Node{model}}
	if got := v.Hit(0.5, 0.5, 1); got != "quad" {
		t.Fatalf("hit %q", got)
	}
}

func TestLoadGLTFRefusals(t *testing.T) {
	cases := map[string]string{
		"draco":     `{"asset":{"version":"2.0"},"extensionsRequired":["KHR_draco_mesh_compression"]}`,
		"version 1": `{"asset":{"version":"1.0"}}`,
		"external":  `{"asset":{"version":"2.0"},"buffers":[{"uri":"model.bin","byteLength":4}]}`,
		"junk":      `{`,
	}
	for name, doc := range cases {
		if _, err := LoadGLTF([]byte(doc), nil); err == nil {
			t.Errorf("%s passed", name)
		}
	}
	// External files come through files.
	doc := `{"asset":{"version":"2.0"},"buffers":[{"uri":"model.bin","byteLength":4}],"nodes":[{"name":"empty"}]}`
	n, err := LoadGLTF([]byte(doc), func(uri string) ([]byte, error) { return []byte{1, 2, 3, 4}, nil })
	if err != nil || len(n.Children) != 1 || n.Children[0].ID != "empty" {
		t.Fatalf("%+v %v", n, err)
	}
	// A data URI buffer.
	doc = `{"asset":{"version":"2.0"},"buffers":[{"uri":"data:application/octet-stream;base64,AAAAAA==","byteLength":4}]}`
	if _, err := LoadGLTF([]byte(doc), nil); err != nil {
		t.Fatal(err)
	}
}
