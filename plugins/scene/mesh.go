package scene

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"sync"
)

// UV is a place on a Texture: (0, 0) its top left, (1, 1) its bottom right.
type UV struct {
	U float32 // across, 0 at the left
	V float32 // down, 0 at the top
}

// Mesh is a shape made of triangles. Box, Sphere and Plane make the usual
// ones; NewMesh takes your own, and LoadGLTF reads them from a file.
type Mesh struct {
	positions []Vec3
	normals   []Vec3
	uvs       []UV
	indices   []uint32
	once      sync.Once
	key       string
	min, max  Vec3 // its bounding box
}

// NewMesh makes a shape from triangles: positions and their normals, one
// each per vertex, and three indices into them per triangle, counter-
// clockwise seen from the front. A nil normals works them out, flat.
//
//nimbus:param positions the vertices
//nimbus:param normals each vertex's outward direction, or nil
//nimbus:param indices three a triangle
func NewMesh(positions, normals []Vec3, indices []uint32) *Mesh {
	return NewTexturedMesh(positions, normals, nil, indices)
}

// NewTexturedMesh is NewMesh with where each vertex falls on a Texture.
//
//nimbus:param positions the vertices
//nimbus:param normals each vertex's outward direction, or nil
//nimbus:param uvs each vertex's place on the texture, or nil
//nimbus:param indices three a triangle
func NewTexturedMesh(positions, normals []Vec3, uvs []UV, indices []uint32) *Mesh {
	if len(uvs) != len(positions) {
		uvs = make([]UV, len(positions))
	}
	m := &Mesh{positions: positions, normals: normals, uvs: uvs, indices: indices}
	if len(normals) != len(positions) {
		m.flatten()
	}
	return m
}

// flatten gives each triangle its own corners, facing the way it faces.
func (m *Mesh) flatten() {
	var p, n []Vec3
	var uv []UV
	var out []uint32
	for t := 0; t+2 < len(m.indices); t += 3 {
		i, j, k := m.indices[t], m.indices[t+1], m.indices[t+2]
		if int(i) >= len(m.positions) || int(j) >= len(m.positions) || int(k) >= len(m.positions) {
			continue
		}
		a, b, c := m.positions[i], m.positions[j], m.positions[k]
		f := normalize(cross(sub(b, a), sub(c, a)))
		base := uint32(len(p))
		p = append(p, a, b, c)
		n = append(n, f, f, f)
		uv = append(uv, m.uvs[i], m.uvs[j], m.uvs[k])
		out = append(out, base, base+1, base+2)
	}
	m.positions, m.normals, m.uvs, m.indices = p, n, uv, out
}

// vertices is the mesh as the GPU reads it: x y z, nx ny nz, u v per
// vertex, float32 little-endian.
func (m *Mesh) vertices() []byte {
	v := make([]byte, 0, len(m.positions)*32)
	for i, p := range m.positions {
		n, t := m.normals[i], m.uvs[i]
		for _, f := range [8]float32{p.X, p.Y, p.Z, n.X, n.Y, n.Z, t.U, t.V} {
			v = binary.LittleEndian.AppendUint32(v, math.Float32bits(f))
		}
	}
	return v
}

func (m *Mesh) indexBytes() []byte {
	ix := make([]byte, 0, len(m.indices)*4)
	for _, i := range m.indices {
		ix = binary.LittleEndian.AppendUint32(ix, i)
	}
	return ix
}

// register names the mesh by its content and keeps it for the phone to
// ask for.
func (m *Mesh) register() string {
	m.once.Do(func() {
		h := fnv.New64a()
		h.Write(m.vertices())
		h.Write(m.indexBytes())
		m.key = "m" + base64.RawURLEncoding.EncodeToString(h.Sum(nil))
		m.min, m.max = bounds(m.positions)
		assets.Store(m.key, m)
	})
	return m.key
}

func bounds(ps []Vec3) (lo, hi Vec3) {
	if len(ps) == 0 {
		return
	}
	lo, hi = ps[0], ps[0]
	for _, p := range ps[1:] {
		lo = Vec3{min(lo.X, p.X), min(lo.Y, p.Y), min(lo.Z, p.Z)}
		hi = Vec3{max(hi.X, p.X), max(hi.Y, p.Y), max(hi.Z, p.Z)}
	}
	return
}

// Shapes are made once per size and kept, so a scene that renders again
// sends nothing new.
var shapes sync.Map

func shape(key string, make func() *Mesh) *Mesh {
	if m, ok := shapes.Load(key); ok {
		return m.(*Mesh)
	}
	m, _ := shapes.LoadOrStore(key, make())
	return m.(*Mesh)
}

// Box is a box w wide, h high and d deep, centered on its node. A Texture
// covers each face whole.
//
//nimbus:param w across
//nimbus:param h up
//nimbus:param d deep
func Box(w, h, d float32) *Mesh {
	return shape(fmt.Sprint("box", w, h, d), func() *Mesh {
		x, y, z := w/2, h/2, d/2
		faces := []struct{ n, u, v Vec3 }{ // outward normal, and the face's two edges
			{V(0, 0, 1), V(x, 0, 0), V(0, y, 0)}, {V(0, 0, -1), V(-x, 0, 0), V(0, y, 0)},
			{V(1, 0, 0), V(0, 0, -z), V(0, y, 0)}, {V(-1, 0, 0), V(0, 0, z), V(0, y, 0)},
			{V(0, 1, 0), V(x, 0, 0), V(0, 0, -z)}, {V(0, -1, 0), V(x, 0, 0), V(0, 0, z)},
		}
		m := &Mesh{}
		for _, f := range faces {
			c := Vec3{f.n.X * x, f.n.Y * y, f.n.Z * z}
			base := uint32(len(m.positions))
			for _, corner := range [4][2]float32{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
				m.positions = append(m.positions, add(c, add(times(f.u, corner[0]), times(f.v, corner[1]))))
				m.normals = append(m.normals, f.n)
				m.uvs = append(m.uvs, UV{(corner[0] + 1) / 2, (1 - corner[1]) / 2})
			}
			m.indices = append(m.indices, base, base+1, base+2, base, base+2, base+3)
		}
		return m
	})
}

// Sphere is a ball of radius r, smooth enough at any size on a phone. A
// Texture wraps around it like a map of the world.
//
//nimbus:param r its radius
func Sphere(r float32) *Mesh {
	return shape(fmt.Sprint("sphere", r), func() *Mesh {
		const rings, segments = 24, 40
		m := &Mesh{}
		for i := 0; i <= rings; i++ {
			phi := math.Pi * float64(i) / rings
			for j := 0; j <= segments; j++ {
				theta := 2 * math.Pi * float64(j) / segments
				n := V(float32(math.Sin(phi)*math.Sin(theta)), float32(math.Cos(phi)), float32(math.Sin(phi)*math.Cos(theta)))
				m.positions = append(m.positions, times(n, r))
				m.normals = append(m.normals, n)
				m.uvs = append(m.uvs, UV{float32(j) / segments, float32(i) / rings})
			}
		}
		for i := 0; i < rings; i++ {
			for j := 0; j < segments; j++ {
				a := uint32(i*(segments+1) + j)
				b := a + segments + 1
				m.indices = append(m.indices, a, b, a+1, a+1, b, b+1)
			}
		}
		return m
	})
}

// Plane is a flat rectangle w by d, facing up: a floor or a table top. A
// Texture covers it whole, its top at the far edge.
//
//nimbus:param w across
//nimbus:param d deep
func Plane(w, d float32) *Mesh {
	return shape(fmt.Sprint("plane", w, d), func() *Mesh {
		x, z := w/2, d/2
		up := V(0, 1, 0)
		return &Mesh{
			positions: []Vec3{V(-x, 0, z), V(x, 0, z), V(x, 0, -z), V(-x, 0, -z)},
			normals:   []Vec3{up, up, up, up},
			uvs:       []UV{{0, 1}, {1, 1}, {1, 0}, {0, 0}},
			indices:   []uint32{0, 1, 2, 0, 2, 3},
		}
	})
}
