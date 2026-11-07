// Package scene shows 3D inside your screen. Go describes the scene: a
// camera, lights, and objects with a shape, a color or a picture, and a
// place. Models come from glTF files, the format Blender and most 3D tools
// export. The phone draws it with its own GPU API, Metal on iOS and Vulkan
// on Android, with shadows if you ask, and runs animations such as Spin and
// the orbit camera itself, every frame, so they stay smooth whatever Go is
// doing.
//
//	nimbus native plugin add scene
//
//	cup, err := scene.LoadGLTF(glb, nil)
//	scene.View{
//		Camera:  scene.Camera{Position: scene.V(0, 1.5, 4)},
//		Orbit:   true,
//		Shadows: true,
//		Nodes: []scene.Node{
//			scene.Fit(cup, 1.5),
//			{ID: "floor", Mesh: scene.Plane(6, 6), Color: ui.Hex("#e8e2d8"), Position: scene.V(0, -0.75, 0)},
//		},
//		OnTap: func(id string) { picked(id) },
//		Style: ui.Style{Height: ui.Pt(300)},
//	}
//
// Changing anything re-describes the scene, so change it in answer to
// something (a tap, new data), not every frame; for motion, use Spin.
// Meshes and pictures go to the phone once each, when it first needs them.
package scene

import (
	"encoding/json"
	"math"

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

// Mat4 is a 4×4 transform, column by column, as glTF and the GPUs keep it.
type Mat4 [16]float32

// Camera is where the scene is seen from.
type Camera struct {
	Position Vec3    // where it stands; zero means (0, 0, 5)
	Target   Vec3    // what it looks at, and what Orbit turns around
	FOV      float32 // vertical field of view in degrees; 0 means 60
	Near     float32 // nothing closer is drawn; 0 means 0.1
	Far      float32 // nothing farther is drawn; 0 means 100
}

// Light is sunlight from one direction. Ambient light fills in the rest.
// With Shadows, the first light casts them.
type Light struct {
	Direction Vec3     // the way the light travels
	Color     ui.Color // white when 0
	Intensity float32  // 0 means 1
}

// Spin turns a node around its own center, forever, drawn by the phone. Its
// children turn with it.
type Spin struct {
	Axis  Vec3    // what it turns around; zero means up (0, 1, 0)
	Speed float32 // degrees a second; negative turns the other way
}

// Node is one object, or a group of them.
type Node struct {
	ID          string   // your name for it, sent to OnTap
	Mesh        *Mesh    // its shape: Box, Sphere, Plane, NewMesh, or from LoadGLTF; nil for a group
	Texture     *Texture // a picture wrapped on it, tinted by Color; nil for none
	Color       ui.Color // the theme's Primary when 0, or white with a Texture
	Unlit       bool     // ignore the lights and shadows, and show Color as it is
	Shine       float32  // how glossy, 0 (matte) to 1
	DoubleSided bool     // draw its back faces too, for leaves and open shapes
	NoShadow    bool     // cast no shadow
	Position    Vec3     // where its center is
	Rotation    Vec3     // degrees around X, then Y, then Z
	Scale       Vec3     // zero means (1, 1, 1)
	// Matrix, when set, places it instead of Position, Rotation and Scale.
	Matrix   *Mat4
	Spin     Spin   // a turn the phone animates
	Children []Node // placed relative to this node, turning with its Spin
}

// View is a 3D scene inside your screen.
type View struct {
	Camera     Camera   // where it's seen from
	Orbit      bool     // drag to turn the camera around Target, pinch to zoom
	Lights     []Light  // none means one from above and in front
	Ambient    float32  // light that comes from everywhere, 0 to 1; 0 means 0.25
	Shadows    bool     // the first light casts shadows
	Background ui.Color // the theme's Background when 0; set alpha to 0 to see through, on iOS
	Nodes      []Node   // the objects
	// OnTap gets the ID of the node tapped, or "" for none.
	OnTap func(id string)
	Style ui.Style // its size and place on the screen
}

// Render describes the scene to the phone.
func (v View) Render(c *ui.Ctx) ui.Component {
	th := ui.UseTheme(c)
	w := v.wire(th)
	ctx := c.Context() // where the asset calls go: the phone, or the tab on the web
	return ui.Native("scene", w, v.Style).OnEvent(func(name, data string) {
		switch name {
		case "need":
			var n need
			if json.Unmarshal([]byte(data), &n) == nil {
				go n.send(ctx)
			}
		case "tap":
			if v.OnTap == nil {
				return
			}
			var e struct {
				X, Y, W, H  float32
				Eye, Target *Vec3
			}
			if json.Unmarshal([]byte(data), &e) != nil || e.W <= 0 || e.H <= 0 {
				return
			}
			at := v
			if e.Eye != nil && e.Target != nil { // where Orbit has taken the camera
				at.Camera.Position, at.Camera.Target = *e.Eye, *e.Target
			}
			v.OnTap(at.Hit(e.X/e.W, e.Y/e.H, e.W/e.H))
		}
	})
}

// wire is what the phone gets: every node flattened to a world matrix,
// meshes and pictures by key, sent when the phone asks.
type wire struct {
	Background uint32      `json:"background"`
	Eye        Vec3        `json:"eye"`
	Target     Vec3        `json:"target"`
	FOV        float32     `json:"fov"`
	Near       float32     `json:"near"`
	Far        float32     `json:"far"`
	Orbit      bool        `json:"orbit"`
	Ambient    float32     `json:"ambient"`
	Lights     []wireLight `json:"lights"`
	Shadow     *wireShadow `json:"shadow,omitempty"`
	Nodes      []wireNode  `json:"nodes"`
}

type wireLight struct {
	Direction Vec3       `json:"direction"` // normalized, the way it travels
	Color     [3]float32 `json:"color"`     // linear, times intensity
}

// wireShadow is the first light's view, fitted around the whole scene.
type wireShadow struct {
	VP Mat4 `json:"vp"` // world → the light's clip space, depth 0 to 1
}

type wireNode struct {
	ID      string     `json:"id"`
	Mesh    string     `json:"mesh"`
	Texture string     `json:"texture,omitempty"`
	Color   [4]float32 `json:"color"`
	Unlit   bool       `json:"unlit"`
	Shine   float32    `json:"shine"`
	Double  bool       `json:"double"`
	Cast    bool       `json:"cast"`
	Base    Mat4       `json:"base"`  // parent, then its place
	Local   Mat4       `json:"local"` // its rotation and scale, after Spin
	Axis    Vec3       `json:"axis"`
	Speed   float32    `json:"speed"` // degrees a second
	// Parent is 1 + the index of the spinning node it turns with, 0 for
	// none; Base is then relative to that node.
	Parent int `json:"parent"`
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

// place is where a node sits: base is its parent and position (or its
// Matrix), local its rotation and scale. Spin goes between them.
func (n Node) place(parent Mat4) (base, local Mat4) {
	if n.Matrix != nil {
		return mul(parent, *n.Matrix), identity()
	}
	scale := n.Scale
	if scale == (Vec3{}) {
		scale = V(1, 1, 1)
	}
	return mul(parent, translate(n.Position)), mul(rotateXYZ(n.Rotation), scaling(scale))
}

// walk calls f for every node with its place, parents first. pivot is
// the center of the nearest Spin at or above it, nil when nothing turns it.
func walk(nodes []Node, parent Mat4, pivot *Vec3, f func(n Node, base, local Mat4, pivot *Vec3)) {
	for _, n := range nodes {
		base, local := n.place(parent)
		p := pivot
		if n.Spin.Speed != 0 {
			c := apply(base, Vec3{})
			p = &c
		}
		f(n, base, local, p)
		walk(n.Children, mul(base, local), p, f)
	}
}

// sweep is the ball a node fills as it turns around pivot: its corners keep
// their distance from the center of a turn.
func sweep(n Node, world Mat4, pivot Vec3) sphere {
	lo, hi := bounds(n.Mesh.positions)
	var r float32
	for _, p := range corners(lo, hi) {
		r = max(r, length(sub(apply(world, p), pivot)))
	}
	return sphere{pivot, r}
}

func (v View) wire(th ui.Theme) wire {
	cam := v.Camera.defaults()
	w := wire{Eye: cam.Position, Target: cam.Target, FOV: cam.FOV, Near: cam.Near, Far: cam.Far, Orbit: v.Orbit,
		Ambient: v.Ambient, Nodes: []wireNode{}, Lights: []wireLight{}}
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
	w.add(v.Nodes, identity(), 0, th)
	if v.Shadows {
		if vp, ok := v.shadowView(w.Lights[0].Direction); ok {
			w.Shadow = &wireShadow{VP: vp}
		}
	}
	return w
}

// add puts nodes on the wire, parents first. A spinning node goes even
// without a mesh, so its children can turn with it: under is 1 + its index,
// and parent is then the place within it.
func (w *wire) add(nodes []Node, parent Mat4, under int, th ui.Theme) {
	for _, n := range nodes {
		base, local := n.place(parent)
		drawn := n.Mesh != nil && len(n.Mesh.indices) > 0
		if !drawn && n.Spin.Speed == 0 {
			w.add(n.Children, mul(base, local), under, th)
			continue
		}
		color := n.Color
		if color == 0 {
			color = th.Primary
			if n.Texture != nil {
				color = ui.Hex("#ffffff")
			}
		}
		axis := n.Spin.Axis
		if axis == (Vec3{}) {
			axis = V(0, 1, 0)
		}
		wn := wireNode{ID: n.ID, Color: rgba(color), Unlit: n.Unlit, Shine: clamp01(n.Shine), Double: n.DoubleSided,
			Cast: drawn && !n.NoShadow && !n.Unlit, Base: base, Local: local, Axis: normalize(axis), Speed: n.Spin.Speed, Parent: under}
		if drawn {
			wn.Mesh = n.Mesh.register()
		}
		if n.Texture != nil {
			wn.Texture = n.Texture.register()
		}
		w.Nodes = append(w.Nodes, wn)
		if n.Spin.Speed != 0 {
			w.add(n.Children, identity(), len(w.Nodes), th)
		} else {
			w.add(n.Children, mul(base, local), under, th)
		}
	}
}

func clamp01(x float32) float32 { return float32(math.Max(0, math.Min(1, float64(x)))) }
