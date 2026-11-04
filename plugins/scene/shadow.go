package scene

import "math"

// sphere is a ball that holds a node however it turns.
type sphere struct {
	c Vec3
	r float32
}

// spheres are the balls around every mesh in nodes, where they stand.
func spheres(nodes []Node) []sphere {
	var out []sphere
	walk(nodes, identity(), nil, func(n Node, base, local Mat4, pivot *Vec3) {
		if n.Mesh == nil || len(n.Mesh.positions) == 0 {
			return
		}
		world := mul(base, local)
		if pivot != nil { // it turns around the spin's center
			out = append(out, sweep(n, world, *pivot))
			return
		}
		lo, hi := bounds(n.Mesh.positions)
		mid := times(add(lo, hi), 0.5)
		out = append(out, sphere{apply(world, mid), length(sub(hi, mid)) * maxScale(world)})
	})
	return out
}

func corners(lo, hi Vec3) [8]Vec3 {
	return [8]Vec3{lo, {hi.X, lo.Y, lo.Z}, {lo.X, hi.Y, lo.Z}, {hi.X, hi.Y, lo.Z},
		{lo.X, lo.Y, hi.Z}, {hi.X, lo.Y, hi.Z}, {lo.X, hi.Y, hi.Z}, hi}
}

// enclose is one ball around them all.
func enclose(ss []sphere) (sphere, bool) {
	if len(ss) == 0 {
		return sphere{}, false
	}
	lo := sub(ss[0].c, V(ss[0].r, ss[0].r, ss[0].r))
	hi := add(ss[0].c, V(ss[0].r, ss[0].r, ss[0].r))
	for _, s := range ss[1:] {
		a, b := sub(s.c, V(s.r, s.r, s.r)), add(s.c, V(s.r, s.r, s.r))
		lo = Vec3{min(lo.X, a.X), min(lo.Y, a.Y), min(lo.Z, a.Z)}
		hi = Vec3{max(hi.X, b.X), max(hi.Y, b.Y), max(hi.Z, b.Z)}
	}
	c := times(add(lo, hi), 0.5)
	var r float32
	for _, s := range ss {
		r = max(r, length(sub(s.c, c))+s.r)
	}
	return sphere{c, max(r, 1e-3)}, true
}

// shadowView is what the first light sees: the whole scene, from far
// enough back along its direction, square on.
func (v View) shadowView(dir Vec3) (Mat4, bool) {
	s, ok := enclose(spheres(v.Nodes))
	if !ok || dir == (Vec3{}) {
		return Mat4{}, false
	}
	eye := sub(s.c, times(dir, 2*s.r))
	up := V(0, 1, 0)
	if math.Abs(float64(dot(dir, up))) > 0.99 {
		up = V(0, 0, -1)
	}
	return mul(ortho(s.r, s.r, 3*s.r), lookAt(eye, s.c, up)), true
}

// Bounds is the box around a node and its children, as placed, ignoring
// Spin.
//
//nimbus:param n the node, such as one from LoadGLTF
//nimbus:returns its lowest and highest corners; both zero for no meshes
func Bounds(n Node) (lo, hi Vec3) {
	first := true
	walk([]Node{n}, identity(), nil, func(m Node, base, local Mat4, _ *Vec3) {
		if m.Mesh == nil || len(m.Mesh.positions) == 0 {
			return
		}
		world := mul(base, local)
		a, b := bounds(m.Mesh.positions)
		for _, p := range corners(a, b) {
			q := apply(world, p)
			if first {
				lo, hi, first = q, q, false
				continue
			}
			lo = Vec3{min(lo.X, q.X), min(lo.Y, q.Y), min(lo.Z, q.Z)}
			hi = Vec3{max(hi.X, q.X), max(hi.Y, q.Y), max(hi.Z, q.Z)}
		}
	})
	return
}

// Fit wraps a node so it's size across at its widest, centered on its new
// parent's place: a model from any tool, at the size your scene wants.
//
//nimbus:param n the node, such as one from LoadGLTF
//nimbus:param size how big its largest side becomes
//nimbus:returns a node holding it; set its Position, Spin and ID as for any other
func Fit(n Node, size float32) Node {
	lo, hi := Bounds(n)
	ext := sub(hi, lo)
	big := max(ext.X, ext.Y, ext.Z)
	if big <= 0 {
		return Node{Children: []Node{n}}
	}
	k := size / big
	mid := times(add(lo, hi), 0.5)
	m := mul(scaling(V(k, k, k)), translate(times(mid, -1)))
	return Node{ID: n.ID, Children: []Node{{Matrix: &m, Children: []Node{n}}}}
}
