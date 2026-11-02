package scene

import "math"

// Matrices are 4×4, column by column, as Metal and Vulkan read them.

func identity() [16]float32 { return [16]float32{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1} }

func mul(a, b [16]float32) [16]float32 {
	var m [16]float32
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

func apply(m [16]float32, p Vec3) Vec3 {
	return Vec3{m[0]*p.X + m[4]*p.Y + m[8]*p.Z + m[12], m[1]*p.X + m[5]*p.Y + m[9]*p.Z + m[13], m[2]*p.X + m[6]*p.Y + m[10]*p.Z + m[14]}
}

func translate(v Vec3) [16]float32 {
	m := identity()
	m[12], m[13], m[14] = v.X, v.Y, v.Z
	return m
}

func scaling(v Vec3) [16]float32 {
	m := identity()
	m[0], m[5], m[10] = v.X, v.Y, v.Z
	return m
}

func rad(deg float32) float64 { return float64(deg) * math.Pi / 180 }

// rotateXYZ turns around X, then Y, then Z.
func rotateXYZ(d Vec3) [16]float32 {
	sx, cx := math.Sincos(rad(d.X))
	sy, cy := math.Sincos(rad(d.Y))
	sz, cz := math.Sincos(rad(d.Z))
	x := [16]float32{1, 0, 0, 0, 0, float32(cx), float32(sx), 0, 0, float32(-sx), float32(cx), 0, 0, 0, 0, 1}
	y := [16]float32{float32(cy), 0, float32(-sy), 0, 0, 1, 0, 0, float32(sy), 0, float32(cy), 0, 0, 0, 0, 1}
	z := [16]float32{float32(cz), float32(sz), 0, 0, float32(-sz), float32(cz), 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
	return mul(z, mul(y, x))
}

// lookAt is the view matrix of a camera at eye looking at target.
func lookAt(eye, target, up Vec3) [16]float32 {
	f := normalize(sub(target, eye))
	if f == (Vec3{}) {
		f = V(0, 0, -1)
	}
	s := normalize(cross(f, up))
	if s == (Vec3{}) { // looking straight up or down
		s = normalize(cross(f, V(0, 0, -1)))
	}
	u := cross(s, f)
	return [16]float32{
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
