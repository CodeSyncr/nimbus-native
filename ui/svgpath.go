package ui

import (
	"math"
	"strconv"
)

// Path adds SVG path data ("M10 10 L20 20 …", as an icon's d attribute)
// to the path: every command, relative and absolute, arcs included. It
// stops at the first thing it can't read.
//
//nimbus:param d the path data
func (p *Painter) Path(d string) {
	s := svgScanner{s: d}
	var cmd byte
	var cx, cy, sx, sy float32 // where it is, and where the subpath began
	var lcx, lcy float32       // the last control point, for S and T
	var last byte              // the last command, upper case
	for {
		s.space()
		if s.done() {
			return
		}
		if c := s.s[s.i]; isSVGCommand(c) {
			cmd = c
			s.i++
		} else if cmd == 0 {
			return
		}
		rel := cmd >= 'a'
		up := cmd &^ 0x20
		var ox, oy float32
		if rel {
			ox, oy = cx, cy
		}
		ok := true
		nums := func(n int) []float32 {
			out := make([]float32, n)
			for i := range out {
				v, good := s.number()
				if !good {
					ok = false
					return nil
				}
				out[i] = v
			}
			return out
		}
		switch up {
		case 'Z':
			p.Close()
			cx, cy = sx, sy
			last = 'Z'
			continue
		case 'M':
			v := nums(2)
			if !ok {
				return
			}
			cx, cy = ox+v[0], oy+v[1]
			sx, sy = cx, cy
			p.MoveTo(cx, cy)
			// More pairs after a moveto are linetos.
			if rel {
				cmd = 'l'
			} else {
				cmd = 'L'
			}
		case 'L':
			v := nums(2)
			if !ok {
				return
			}
			cx, cy = ox+v[0], oy+v[1]
			p.LineTo(cx, cy)
		case 'H':
			v := nums(1)
			if !ok {
				return
			}
			cx = ox + v[0]
			p.LineTo(cx, cy)
		case 'V':
			v := nums(1)
			if !ok {
				return
			}
			if rel {
				cy += v[0]
			} else {
				cy = v[0]
			}
			p.LineTo(cx, cy)
		case 'C':
			v := nums(6)
			if !ok {
				return
			}
			p.CubicTo(ox+v[0], oy+v[1], ox+v[2], oy+v[3], ox+v[4], oy+v[5])
			lcx, lcy = ox+v[2], oy+v[3]
			cx, cy = ox+v[4], oy+v[5]
		case 'S':
			v := nums(4)
			if !ok {
				return
			}
			c1x, c1y := cx, cy
			if last == 'C' || last == 'S' {
				c1x, c1y = 2*cx-lcx, 2*cy-lcy
			}
			p.CubicTo(c1x, c1y, ox+v[0], oy+v[1], ox+v[2], oy+v[3])
			lcx, lcy = ox+v[0], oy+v[1]
			cx, cy = ox+v[2], oy+v[3]
		case 'Q':
			v := nums(4)
			if !ok {
				return
			}
			p.QuadTo(ox+v[0], oy+v[1], ox+v[2], oy+v[3])
			lcx, lcy = ox+v[0], oy+v[1]
			cx, cy = ox+v[2], oy+v[3]
		case 'T':
			v := nums(2)
			if !ok {
				return
			}
			qx, qy := cx, cy
			if last == 'Q' || last == 'T' {
				qx, qy = 2*cx-lcx, 2*cy-lcy
			}
			p.QuadTo(qx, qy, ox+v[0], oy+v[1])
			lcx, lcy = qx, qy
			cx, cy = ox+v[0], oy+v[1]
		case 'A':
			v := nums(3)
			if !ok {
				return
			}
			large, ok1 := s.flag()
			sweep, ok2 := s.flag()
			end := nums(2)
			if !ok || !ok1 || !ok2 {
				return
			}
			x, y := ox+end[0], oy+end[1]
			p.svgArc(cx, cy, v[0], v[1], v[2], large, sweep, x, y)
			cx, cy = x, y
		}
		last = up
	}
}

func isSVGCommand(c byte) bool {
	switch c &^ 0x20 {
	case 'M', 'L', 'H', 'V', 'C', 'S', 'Q', 'T', 'A', 'Z':
		return true
	}
	return false
}

// svgArc adds SVG's elliptical arc from (x1, y1) to (x2, y2) as cubic
// curves (the SVG spec's appendix B.2.4 conversion).
func (p *Painter) svgArc(x1, y1, rx, ry, angle float32, large, sweep bool, x2, y2 float32) {
	if x1 == x2 && y1 == y2 {
		return
	}
	if rx == 0 || ry == 0 {
		p.LineTo(x2, y2)
		return
	}
	a, b := math.Abs(float64(rx)), math.Abs(float64(ry))
	phi := float64(angle) * math.Pi / 180
	cos, sin := math.Cos(phi), math.Sin(phi)
	dx, dy := float64(x1-x2)/2, float64(y1-y2)/2
	x1p, y1p := cos*dx+sin*dy, -sin*dx+cos*dy
	if l := x1p*x1p/(a*a) + y1p*y1p/(b*b); l > 1 {
		a, b = a*math.Sqrt(l), b*math.Sqrt(l)
	}
	num := a*a*b*b - a*a*y1p*y1p - b*b*x1p*x1p
	den := a*a*y1p*y1p + b*b*x1p*x1p
	k := math.Sqrt(math.Max(num/den, 0))
	if large == sweep {
		k = -k
	}
	cxp, cyp := k*a*y1p/b, -k*b*x1p/a
	cx := cos*cxp - sin*cyp + float64(x1+x2)/2
	cy := sin*cxp + cos*cyp + float64(y1+y2)/2
	ang := func(ux, uy, vx, vy float64) float64 { return math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy) }
	t1 := ang(1, 0, (x1p-cxp)/a, (y1p-cyp)/b)
	dt := ang((x1p-cxp)/a, (y1p-cyp)/b, (-x1p-cxp)/a, (-y1p-cyp)/b)
	if !sweep && dt > 0 {
		dt -= 2 * math.Pi
	} else if sweep && dt < 0 {
		dt += 2 * math.Pi
	}
	n := int(math.Ceil(math.Abs(dt) / (math.Pi / 2)))
	step := dt / float64(n)
	h := 4.0 / 3 * math.Tan(step/4)
	at := func(t float64) (float64, float64, float64, float64) { // a point, and its tangent
		ct, st := math.Cos(t), math.Sin(t)
		return cx + a*ct*cos - b*st*sin, cy + a*ct*sin + b*st*cos, -a*st*cos - b*ct*sin, -a*st*sin + b*ct*cos
	}
	for i := 0; i < n; i++ {
		ta, tb := t1+float64(i)*step, t1+float64(i+1)*step
		ax, ay, adx, ady := at(ta)
		bx, by, bdx, bdy := at(tb)
		if i == n-1 {
			bx, by = float64(x2), float64(y2)
		}
		p.CubicTo(float32(ax+h*adx), float32(ay+h*ady), float32(bx-h*bdx), float32(by-h*bdy), float32(bx), float32(by))
	}
}

type svgScanner struct {
	s string
	i int
}

func (s *svgScanner) done() bool { return s.i >= len(s.s) }

func (s *svgScanner) space() {
	for !s.done() && (s.s[s.i] == ' ' || s.s[s.i] == ',' || s.s[s.i] == '\t' || s.s[s.i] == '\n' || s.s[s.i] == '\r') {
		s.i++
	}
}

func (s *svgScanner) number() (float32, bool) {
	s.space()
	j := s.i
	if j < len(s.s) && (s.s[j] == '-' || s.s[j] == '+') {
		j++
	}
	dot, exp, digits := false, false, false
	for j < len(s.s) {
		c := s.s[j]
		switch {
		case c >= '0' && c <= '9':
			digits = true
		case c == '.' && !dot && !exp:
			dot = true
		case (c == 'e' || c == 'E') && digits && !exp:
			exp = true
			if j+1 < len(s.s) && (s.s[j+1] == '-' || s.s[j+1] == '+') {
				j++
			}
		default:
			goto end
		}
		j++
	}
end:
	v, err := strconv.ParseFloat(s.s[s.i:j], 32)
	if err != nil {
		return 0, false
	}
	s.i = j
	return float32(v), true
}

// flag reads an arc's 0 or 1, which may run into the next number ("a1 1 0 01.5 2").
func (s *svgScanner) flag() (bool, bool) {
	s.space()
	if s.done() || (s.s[s.i] != '0' && s.s[s.i] != '1') {
		return false, false
	}
	s.i++
	return s.s[s.i-1] == '1', true
}
