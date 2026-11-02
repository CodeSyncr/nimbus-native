package scene

import "math"

// Matrices are 4×4, column by column, as Metal and Vulkan read them.

func identity() Mat4 { return Mat4{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1} }

func mul(a, b Mat4) Mat4 {
	var m Mat4
	for c := 0; c < 4; c++ {
		for r := 0; r < 4; r++ {
			var s float32
			for k := 0; k < 4; k++ {
				s += a[k*4+r] * b[c*4+k]
			}
			m[c*4+r] = s
		}
	}
	return m
}

func apply(m Mat4, p Vec3) Vec3 {
	return Vec3{m[0]*p.X + m[4]*p.Y + m[8]*p.Z + m[12], m[1]*p.X + m[5]*p.Y + m[9]*p.Z + m[13], m[2]*p.X + m[6]*p.Y + m[10]*p.Z + m[14]}
}

func translate(v Vec3) Mat4 {
	m := identity()
	m[12], m[13], m[14] = v.X, v.Y, v.Z
	return m
}

func scaling(v Vec3) Mat4 {
	m := identity()
	m[0], m[5], m[10] = v.X, v.Y, v.Z
	return m
}

func rad(deg float32) float64 { return float64(deg) * math.Pi / 180 }

// rotateXYZ turns around X, then Y, then Z.
func rotateXYZ(d Vec3) Mat4 {
	sx, cx := math.Sincos(rad(d.X))
	sy, cy := math.Sincos(rad(d.Y))
	sz, cz := math.Sincos(rad(d.Z))
	x := Mat4{1, 0, 0, 0, 0, float32(cx), float32(sx), 0, 0, float32(-sx), float32(cx), 0, 0, 0, 0, 1}
	y := Mat4{float32(cy), 0, float32(-sy), 0, 0, 1, 0, 0, float32(sy), 0, float32(cy), 0, 0, 0, 0, 1}
	z := Mat4{float32(cz), float32(sz), 0, 0, float32(-sz), float32(cz), 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
	return mul(z, mul(y, x))
}

// lookAt is the view matrix of a camera at eye looking at target.
func lookAt(eye, target, up Vec3) Mat4 {
	f := normalize(sub(target, eye))
	if f == (Vec3{}) {
		f = V(0, 0, -1)
	}
	s := normalize(cross(f, up))
	if s == (Vec3{}) { // looking straight up or down
		s = normalize(cross(f, V(0, 0, -1)))
	}
	u := cross(s, f)
	return Mat4{
		s.X, u.X, -f.X, 0,
		s.Y, u.Y, -f.Y, 0,
		s.Z, u.Z, -f.Z, 0,
		-dot(s, eye), -dot(u, eye), dot(f, eye), 1,
	}
}

func sub(a, b Vec3) Vec3           { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func add(a, b Vec3) Vec3           { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func times(a Vec3, k float32) Vec3 { return Vec3{a.X * k, a.Y * k, a.Z * k} }
func dot(a, b Vec3) float32        { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func cross(a, b Vec3) Vec3 {
	return Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}
func length(a Vec3) float32 { return float32(math.Sqrt(float64(dot(a, a)))) }

func normalize(a Vec3) Vec3 {
	l := length(a)
	if l == 0 {
		return Vec3{}
	}
	return times(a, 1/l)
}

// ortho is an orthographic projection of a box seen down -Z: x and y from
// -r to r, z from -n to -f, onto depth 0 to 1.
func ortho(r, n, f float32) Mat4 {
	return Mat4{1 / r, 0, 0, 0, 0, 1 / r, 0, 0, 0, 0, 1 / (n - f), 0, 0, 0, n / (n - f), 1}
}

// quat turns a unit quaternion (x, y, z, w), as glTF writes rotations,
// into a matrix.
func quat(x, y, z, w float32) Mat4 {
	return Mat4{
		1 - 2*(y*y+z*z), 2 * (x*y + z*w), 2 * (x*z - y*w), 0,
		2 * (x*y - z*w), 1 - 2*(x*x+z*z), 2 * (y*z + x*w), 0,
		2 * (x*z + y*w), 2 * (y*z - x*w), 1 - 2*(x*x+y*y), 0,
		0, 0, 0, 1,
	}
}

// maxScale is how much m stretches a length, at most.
func maxScale(m Mat4) float32 {
	sx := length(V(m[0], m[1], m[2]))
	sy := length(V(m[4], m[5], m[6]))
	sz := length(V(m[8], m[9], m[10]))
	return float32(math.Max(float64(sx), math.Max(float64(sy), float64(sz))))
}
