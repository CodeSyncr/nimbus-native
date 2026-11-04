package scene

import "math"

// Hit says which node is under a point of the view: x and y from 0 to 1,
// from the top left, and the view's width over its height. OnTap uses it.
// A spinning node, and anything turning with it, is tested by the sphere it
// sweeps, since only the phone knows how far it has turned.
//
//nimbus:param x across, 0 to 1
//nimbus:param y down, 0 to 1
//nimbus:param aspect the view's width divided by its height
//nimbus:returns the node's ID, or "" for none
func (v View) Hit(x, y, aspect float32) string {
	cam := v.Camera.defaults()
	f := normalize(sub(cam.Target, cam.Position))
	if f == (Vec3{}) {
		f = V(0, 0, -1)
	}
	s := normalize(cross(f, V(0, 1, 0)))
	if s == (Vec3{}) {
		s = normalize(cross(f, V(0, 0, -1)))
	}
	u := cross(s, f)
	t := float32(math.Tan(rad(cam.FOV) / 2))
	dir := normalize(add(f, add(times(s, (2*x-1)*t*aspect), times(u, (1-2*y)*t))))
	best, id := float32(math.MaxFloat32), ""
	walk(v.Nodes, identity(), nil, func(n Node, base, local Mat4, pivot *Vec3) {
		if n.Mesh == nil {
			return
		}
		if d, ok := n.hit(cam.Position, dir, mul(base, local), pivot); ok && d < best {
			best, id = d, n.ID
		}
	})
	return id
}

func (n Node) hit(o, dir Vec3, world Mat4, pivot *Vec3) (float32, bool) {
	m := n.Mesh
	if pivot != nil {
		s := sweep(n, world, *pivot)
		oc := sub(o, s.c)
		b := dot(oc, dir)
		disc := b*b - (dot(oc, oc) - s.r*s.r)
		if disc < 0 {
			return 0, false
		}
		d := -b - float32(math.Sqrt(float64(disc)))
		return d, d > 0
	}
	best, found := float32(math.MaxFloat32), false
	for i := 0; i+2 < len(m.indices); i += 3 {
		a, b, c := apply(world, m.positions[m.indices[i]]), apply(world, m.positions[m.indices[i+1]]), apply(world, m.positions[m.indices[i+2]])
		if d, ok := triangle(o, dir, a, b, c); ok && d < best {
			best, found = d, true
		}
	}
	return best, found
}

// triangle is Möller–Trumbore: how far along the ray it meets the triangle.
func triangle(o, dir, a, b, c Vec3) (float32, bool) {
	e1, e2 := sub(b, a), sub(c, a)
	p := cross(dir, e2)
	det := dot(e1, p)
	if det > -1e-7 && det < 1e-7 {
		return 0, false
	}
	inv := 1 / det
	t := sub(o, a)
	u := dot(t, p) * inv
	if u < 0 || u > 1 {
		return 0, false
	}
	q := cross(t, e1)
	v := dot(dir, q) * inv
	if v < 0 || u+v > 1 {
		return 0, false
	}
	d := dot(e2, q) * inv
	return d, d > 1e-5
}
