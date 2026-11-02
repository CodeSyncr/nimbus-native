package scene

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math"
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-4 }
func nearV(a, b Vec3) bool   { return near(a.X, b.X) && near(a.Y, b.Y) && near(a.Z, b.Z) }

func TestMatrices(t *testing.T) {
	// The default camera sees the origin straight ahead, 5 away.
	v := lookAt(V(0, 0, 5), Vec3{}, V(0, 1, 0))
	if p := apply(v, Vec3{}); !nearV(p, V(0, 0, -5)) {
		t.Fatalf("origin in view = %v", p)
	}
	if p := apply(v, V(1, 2, 0)); !nearV(p, V(1, 2, -5)) {
		t.Fatalf("(1,2,0) in view = %v", p)
	}
	// Turning 90° around Y takes +X to -Z; X then Y then Z, in that order.
	if p := apply(rotateXYZ(V(0, 90, 0)), V(1, 0, 0)); !nearV(p, V(0, 0, -1)) {
		t.Fatalf("Y 90 = %v", p)
	}
	if p := apply(rotateXYZ(V(90, 90, 0)), V(0, 1, 0)); !nearV(p, V(1, 0, 0)) {
		t.Fatalf("X then Y = %v", p)
	}
	if p := apply(mul(translate(V(1, 2, 3)), scaling(V(2, 2, 2))), V(1, 1, 1)); !nearV(p, V(3, 4, 5)) {
		t.Fatalf("scale then move = %v", p)
	}
	// Looking straight down doesn't break.
	if m := lookAt(V(0, 5, 0), Vec3{}, V(0, 1, 0)); math.IsNaN(float64(m[0])) || m[0] == 0 && m[4] == 0 && m[8] == 0 {
		t.Fatalf("top-down view = %v", m)
	}
}

func TestShapes(t *testing.T) {
	b := Box(2, 1, 1)
	if len(b.positions) != 24 || len(b.indices) != 36 {
		t.Fatalf("box %d %d", len(b.positions), len(b.indices))
	}
	// Every triangle faces the way its normals say: outward, counter-clockwise.
	for _, m := range []*Mesh{b, Sphere(1), Plane(2, 2)} {
		for i := 0; i < len(m.indices); i += 3 {
			a, c, d := m.positions[m.indices[i]], m.positions[m.indices[i+1]], m.positions[m.indices[i+2]]
			f := cross(sub(c, a), sub(d, a))
			if length(f) < 1e-6 {
				continue // a pole's sliver
			}
			if dot(f, m.normals[m.indices[i]]) <= 0 {
				t.Fatalf("triangle %d faces inward", i/3)
			}
		}
	}
	for _, n := range Sphere(2).normals {
		if !near(length(n), 1) {
			t.Fatal("sphere normal not unit")
		}
	}
	if Box(1, 1, 1) != Box(1, 1, 1) {
		t.Fatal("box made twice")
	}
	m := NewMesh([]Vec3{V(0, 0, 0), V(1, 0, 0), V(0, 1, 0)}, nil, []uint32{0, 1, 2})
	if !nearV(m.normals[0], V(0, 0, 1)) {
		t.Fatalf("flat normal = %v", m.normals[0])
	}
}

func TestHit(t *testing.T) {
	v := View{Nodes: []Node{
		{ID: "back", Mesh: Box(4, 4, 0.1), Position: V(0, 0, -3)},
		{ID: "front", Mesh: Box(1, 1, 1)},
		{ID: "left", Mesh: Sphere(0.5), Position: V(-2, 0, 0), Spin: Spin{Speed: 90}},
		{ID: "group", Position: V(2, 0, 0), Children: []Node{{ID: "child", Mesh: Box(0.5, 0.5, 0.5), Position: V(0, 1, 0)}}},
	}}
	cases := []struct {
		x, y float32
		want string
	}{
		{0.5, 0.5, "front"}, // nearer wins
		{0.5, 0.3, "back"},
		{0.01, 0.01, ""},
	}
	for _, c := range cases {
		if got := v.Hit(c.x, c.y, 1); got != c.want {
			t.Errorf("(%v,%v) = %q, want %q", c.x, c.y, got, c.want)
		}
	}
	// Where a point lands on screen, for the side nodes.
	at := func(p Vec3) (float32, float32) {
		q := apply(lookAt(V(0, 0, 5), Vec3{}, V(0, 1, 0)), p)
		tn := float32(math.Tan(math.Pi / 6))
		return (q.X/(-q.Z)/tn + 1) / 2, (1 - q.Y/(-q.Z)/tn) / 2
	}
	if x, y := at(V(-2, 0, 0.5)); v.Hit(x, y, 1) != "left" {
		t.Errorf("spinning sphere missed at %v,%v", x, y)
	}
	if x, y := at(V(2, 1, 0.25)); v.Hit(x, y, 1) != "child" {
		t.Errorf("child missed at %v,%v", x, y)
	}
}

func TestWire(t *testing.T) {
	th := ui.Theme{Primary: ui.Hex("#0d7377"), Background: ui.Hex("#fafafa")}
	v := View{Nodes: []Node{
		{ID: "a", Mesh: Box(1, 1, 1), Spin: Spin{Speed: 30}},
		{ID: "b", Mesh: Box(1, 1, 1), Position: V(2, 0, 0), Color: ui.Hex("#ff000080"), Shine: 3,
			Children: []Node{{ID: "c", Mesh: Sphere(0.2), Position: V(0, 1, 0)}}},
	}}
	w := v.wire(th)
	if len(w.Meshes) != 2 || len(w.Nodes) != 3 {
		t.Fatalf("%d meshes, %d nodes", len(w.Meshes), len(w.Nodes))
	}
	if w.Background != uint32(th.Background) || w.Ambient != 0.25 || len(w.Lights) != 1 || w.FOV != 60 {
		t.Fatalf("defaults %+v", w)
	}
	a, b, c := w.Nodes[0], w.Nodes[1], w.Nodes[2]
	if a.Color != rgba(th.Primary) || a.Speed != 30 || a.Axis != V(0, 1, 0) || b.Shine != 1 || b.Color[3] < 0.5 || b.Color[3] > 0.51 {
		t.Fatalf("nodes %+v %+v", a, b)
	}
	if p := apply(c.Base, Vec3{}); !nearV(p, V(2, 1, 0)) {
		t.Fatalf("child at %v", p)
	}
	// The vertices decode as x y z nx ny nz.
	raw, _ := base64.StdEncoding.DecodeString(w.Meshes[a.Mesh].Vertices)
	f := func(i int) float32 { return math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:])) }
	if len(raw) != 24*24 || !nearV(V(f(0), f(1), f(2)), V(-0.5, -0.5, 0.5)) || !nearV(V(f(3), f(4), f(5)), V(0, 0, 1)) {
		t.Fatalf("first vertex %v %v %v / %v %v %v", f(0), f(1), f(2), f(3), f(4), f(5))
	}
	if _, err := json.Marshal(w); err != nil {
		t.Fatal(err)
	}
	many := View{Lights: make([]Light, 6)}
	if got := len(many.wire(th).Lights); got != 4 {
		t.Fatalf("%d lights", got)
	}
}
