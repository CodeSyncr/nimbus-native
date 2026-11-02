package scene

import (
	"fmt"
	"math"
	"sync"
)

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

// Box is a box w wide, h high and d deep, centered on its node.
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
			}
			m.indices = append(m.indices, base, base+1, base+2, base, base+2, base+3)
		}
		return m
	})
}

// Sphere is a ball of radius r, smooth enough at any size on a phone.
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

// Plane is a flat rectangle w by d, facing up: a floor or a table top.
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
			indices:   []uint32{0, 1, 2, 0, 2, 3},
		}
	})
}
