// Package scene shows 3D inside your screen. Go describes the scene: a
// camera, lights, and objects with a shape, a color and a place. The phone
// draws it with its own GPU API, Metal on iOS and Vulkan on Android, and
// runs animations such as Spin itself, every frame, so they stay smooth
// whatever Go is doing.
//
//	nimbus native plugin add scene
//
//	scene.View{
//		Camera: scene.Camera{Position: scene.V(0, 1.5, 4)},
//		Lights: []scene.Light{{Direction: scene.V(-1, -2, -1)}},
//		Nodes: []scene.Node{
//			{ID: "cup", Mesh: scene.Box(1, 1.2, 1), Color: ui.Hex("#0d7377"), Spin: scene.Spin{Speed: 45}},
//		},
//		OnTap: func(id string) { picked(id) },
//		Style: ui.Style{Height: ui.Pt(300)},
//	}
//
// Changing anything re-describes the scene, so change it in answer to
// something (a tap, new data), not every frame; for motion, use Spin.
package scene

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"hash/fnv"
	"math"
	"sync"

	"github.com/CodeSyncr/nimbus-native/ui"
)

// Vec3 is a point or a direction. Y is up; the camera looks down -Z unless
// told otherwise.
type Vec3 struct {
	X float32 // across, to the right
	Y float32 // up
	Z float32 // toward the viewer
}

// V makes a Vec3.
//
//nimbus:param x across
//nimbus:param y up
//nimbus:param z toward the viewer
func V(x, y, z float32) Vec3 { return Vec3{x, y, z} }

// Camera is where the scene is seen from.
type Camera struct {
	Position Vec3    // where it stands; zero means (0, 0, 5)
	Target   Vec3    // what it looks at
	FOV      float32 // vertical field of view in degrees; 0 means 60
	Near     float32 // nothing closer is drawn; 0 means 0.1
	Far      float32 // nothing farther is drawn; 0 means 100
}

// Light is sunlight from one direction. Ambient light fills in the rest.
type Light struct {
	Direction Vec3     // the way the light travels
	Color     ui.Color // white when 0
	Intensity float32  // 0 means 1
}

// Spin turns a node around its own center, forever, drawn by the phone.
type Spin struct {
	Axis  Vec3    // what it turns around; zero means up (0, 1, 0)
	Speed float32 // degrees a second; negative turns the other way
}

// Node is one object.
type Node struct {
	ID       string   // your name for it, sent to OnTap
	Mesh     *Mesh    // its shape: Box, Sphere, Plane or NewMesh
	Color    ui.Color // the theme's Primary when 0
	Unlit    bool     // ignore the lights and show Color as it is
	Shine    float32  // how glossy, 0 (matte) to 1
	Position Vec3     // where its center is
	Rotation Vec3     // degrees around X, then Y, then Z
	Scale    Vec3     // zero means (1, 1, 1)
	Spin     Spin     // a turn the phone animates
	Children []Node   // placed relative to this node; they don't share its Spin
}

// View is a 3D scene inside your screen.
type View struct {
	Camera     Camera          // where it's seen from
	Lights     []Light         // none means one from above and in front
	Ambient    float32         // light that comes from everywhere, 0 to 1; 0 means 0.25
	Background ui.Color        // the theme's Background when 0; set alpha to 0 to see through
	Nodes      []Node          // the objects
	OnTap      func(id string) // the node tapped, or "" for none
	Style      ui.Style        // its size and place on the screen
}

// Render describes the scene to the phone.
func (v View) Render(c *ui.Ctx) ui.Component {
	th := ui.UseTheme(c)
	w := v.wire(th)
	return ui.Native("scene", w, v.Style).OnEvent(func(name, data string) {
		if name != "tap" || v.OnTap == nil {
			return
		}
		var e struct{ X, Y, W, H float32 }
		if json.Unmarshal([]byte(data), &e) != nil || e.W <= 0 || e.H <= 0 {
			return
		}
		v.OnTap(v.Hit(e.X/e.W, e.Y/e.H, e.W/e.H))
	})
}

// wire is what the phone gets: every node flattened to a world matrix,
// meshes once each by content.
type wire struct {
	Background uint32               `json:"background"`
	View       [16]float32          `json:"view"`
	FOV        float32              `json:"fov"`
	Near       float32              `json:"near"`
	Far        float32              `json:"far"`
	Eye        Vec3                 `json:"eye"`
	Ambient    float32              `json:"ambient"`
	Lights     []wireLight          `json:"lights"`
	Meshes     map[string]*wireMesh `json:"meshes"`
	Nodes      []wireNode           `json:"nodes"`
}

type wireLight struct {
	Direction Vec3       `json:"direction"` // normalized, the way it travels
	Color     [3]float32 `json:"color"`     // linear, times intensity
}

type wireMesh struct {
	Vertices string `json:"vertices"` // float32 little-endian: x y z nx ny nz, per vertex
	Indices  string `json:"indices"`  // uint32 little-endian, three a triangle
	Count    int    `json:"count"`    // indices
}

type wireNode struct {
	ID    string      `json:"id"`
	Mesh  string      `json:"mesh"`
	Color [4]float32  `json:"color"`
	Unlit bool        `json:"unlit"`
	Shine float32     `json:"shine"`
	Base  [16]float32 `json:"base"`  // parent, then its position
	Local [16]float32 `json:"local"` // its rotation and scale, after Spin
	Axis  Vec3        `json:"axis"`
	Speed float32     `json:"speed"` // degrees a second
}

func (c Camera) defaults() Camera {
	if c.Position == (Vec3{}) {
		c.Position = V(0, 0, 5)
	}
	if c.FOV <= 0 {
		c.FOV = 60
	}
	if c.Near <= 0 {
		c.Near = 0.1
	}
	if c.Far <= c.Near {
		c.Far = 100
	}
	return c
}

func rgba(c ui.Color) [4]float32 {
	return [4]float32{float32(c>>24&255) / 255, float32(c>>16&255) / 255, float32(c>>8&255) / 255, float32(c&255) / 255}
}

func (v View) wire(th ui.Theme) wire {
	cam := v.Camera.defaults()
	w := wire{View: lookAt(cam.Position, cam.Target, V(0, 1, 0)), FOV: cam.FOV, Near: cam.Near, Far: cam.Far, Eye: cam.Position,
		Ambient: v.Ambient, Meshes: map[string]*wireMesh{}, Nodes: []wireNode{}, Lights: []wireLight{}}
	w.Background = uint32(v.Background)
	if v.Background == 0 {
		w.Background = uint32(th.Background)
	}
	if w.Ambient <= 0 {
		w.Ambient = 0.25
	}
	lights := v.Lights
	if len(lights) == 0 {
		lights = []Light{{Direction: V(-0.3, -1, -0.6)}}
	}
	for i, l := range lights {
		if i == 4 {
			break // the shaders take four
		}
		col := l.Color
		if col == 0 {
			col = ui.Hex("#ffffff")
		}
		in := l.Intensity
		if in <= 0 {
			in = 1
		}
		c := rgba(col)
		w.Lights = append(w.Lights, wireLight{normalize(l.Direction), [3]float32{c[0] * in, c[1] * in, c[2] * in}})
	}
	var add func(n Node, parent [16]float32)
	add = func(n Node, parent [16]float32) {
		base := mul(parent, translate(n.Position))
		scale := n.Scale
		if scale == (Vec3{}) {
			scale = V(1, 1, 1)
		}
		local := mul(rotateXYZ(n.Rotation), scaling(scale))
		if n.Mesh != nil && len(n.Mesh.indices) > 0 {
			key := n.Mesh.key()
			w.Meshes[key] = n.Mesh.w
			color := n.Color
			if color == 0 {
				color = th.Primary
			}
			axis := n.Spin.Axis
			if axis == (Vec3{}) {
				axis = V(0, 1, 0)
			}
			w.Nodes = append(w.Nodes, wireNode{ID: n.ID, Mesh: key, Color: rgba(color), Unlit: n.Unlit, Shine: clamp01(n.Shine),
				Base: base, Local: local, Axis: normalize(axis), Speed: n.Spin.Speed})
		}
		for _, k := range n.Children {
			add(k, mul(base, local))
		}
	}
	for _, n := range v.Nodes {
		add(n, identity())
	}
	return w
}

func clamp01(x float32) float32 { return float32(math.Max(0, math.Min(1, float64(x)))) }

// Mesh is a shape made of triangles. Box, Sphere and Plane make the usual
// ones; NewMesh takes your own.
type Mesh struct {
	positions []Vec3
	normals   []Vec3
	indices   []uint32
	once      sync.Once
	k         string
	w         *wireMesh // what key encoded, kept for the next render
}

// NewMesh makes a shape from triangles: positions and their normals, one
// each per vertex, and three indices into them per triangle, counter-
// clockwise seen from the front. A nil normals works them out, flat.
//
//nimbus:param positions the vertices
//nimbus:param normals each vertex's outward direction, or nil
//nimbus:param indices three a triangle
func NewMesh(positions, normals []Vec3, indices []uint32) *Mesh {
	if normals == nil || len(normals) != len(positions) {
		positions, normals, indices = flatNormals(positions, indices)
	}
	return &Mesh{positions: positions, normals: normals, indices: indices}
}

func (m *Mesh) key() string {
	m.once.Do(func() {
		m.w = m.wire()
		h := fnv.New64a()
		h.Write([]byte(m.w.Vertices))
		h.Write([]byte(m.w.Indices))
		m.k = base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	})
	return m.k
}

func (m *Mesh) wire() *wireMesh {
	v := make([]byte, 0, len(m.positions)*24)
	for i, p := range m.positions {
		n := m.normals[i]
		for _, f := range [6]float32{p.X, p.Y, p.Z, n.X, n.Y, n.Z} {
			v = binary.LittleEndian.AppendUint32(v, math.Float32bits(f))
		}
	}
	ix := make([]byte, 0, len(m.indices)*4)
	for _, i := range m.indices {
		ix = binary.LittleEndian.AppendUint32(ix, i)
	}
	return &wireMesh{Vertices: base64.StdEncoding.EncodeToString(v), Indices: base64.StdEncoding.EncodeToString(ix), Count: len(m.indices)}
}

func flatNormals(pos []Vec3, idx []uint32) ([]Vec3, []Vec3, []uint32) {
	var p, n []Vec3
	var out []uint32
	for t := 0; t+2 < len(idx); t += 3 {
		a, b, c := pos[idx[t]], pos[idx[t+1]], pos[idx[t+2]]
		f := normalize(cross(sub(b, a), sub(c, a)))
		base := uint32(len(p))
		p = append(p, a, b, c)
		n = append(n, f, f, f)
		out = append(out, base, base+1, base+2)
	}
	return p, n, out
}
