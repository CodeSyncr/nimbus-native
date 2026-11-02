// Package meshgradient fills a view with a mesh gradient: a grid of points,
// each with a color, blended smoothly between them. Move the points off
// their grid and the colors bend with them.
//
//	nimbus native plugin add mesh-gradient
//
//	meshgradient.View{
//		Columns: 3, Rows: 3,
//		Colors: []ui.Color{
//			teal, teal, sky,
//			coral, sand, sky,
//			coral, coral, sand,
//		},
//		Style: ui.Style{Grow: 1},
//	}
//
// Go works out the blend, so it looks the same on both phones; the phone
// draws it stretched to the view's frame.
package meshgradient

import (
	"encoding/base64"
	"encoding/binary"
	"math"

	"github.com/CodeSyncr/nimbus-native/ui"
)

// View is a mesh gradient.
type View struct {
	Columns, Rows int // points across and down; each at least 2
	// Points are where each point sits, row by row from the top left, in
	// the view's unit square: (0, 0) top left, (1, 1) bottom right. Nil
	// spaces them evenly. Move inner points to bend the colors; keep edge
	// points on their edge, or the corners show through.
	Points []ui.Point
	Colors []ui.Color // one per point, row by row
	Detail int        // steps between two points; 0 means 16
	Style  ui.Style   // its size and place on the screen
}

// grid is the gradient cut fine: (W+1)×(H+1) vertices with a place and a
// color, which the phone draws as triangles.
type grid struct {
	W         int    `json:"w"`
	H         int    `json:"h"`
	Positions string `json:"positions"` // float32 x y per vertex, in the unit square
	Colors    string `json:"colors"`    // float32 r g b a per vertex
}

// Render draws it.
func (v View) Render(c *ui.Ctx) ui.Component {
	g, ok := v.grid()
	if !ok {
		return ui.View(v.Style)
	}
	return ui.Native("meshgradient", g, v.Style)
}

func (v View) grid() (grid, bool) {
	cols, rows := v.Columns, v.Rows
	if cols < 2 || rows < 2 || len(v.Colors) != cols*rows || (v.Points != nil && len(v.Points) != cols*rows) {
		return grid{}, false
	}
	detail := v.Detail
	if detail <= 0 {
		detail = 16
	}
	at := func(i, j int) sample { // the control point in column i, row j
		var p ui.Point
		if v.Points != nil {
			p = v.Points[j*cols+i]
		} else {
			p = ui.Point{X: float32(i) / float32(cols-1), Y: float32(j) / float32(rows-1)}
		}
		c := v.Colors[j*cols+i]
		return sample{float64(p.X), float64(p.Y), float64(c>>24&255) / 255, float64(c>>16&255) / 255, float64(c>>8&255) / 255, float64(c&255) / 255}
	}
	// Catmull-Rom across a row of samples, at t between k and k+1.
	curve := func(get func(int) sample, n, k int, t float64) sample {
		p1, p2 := get(k), get(k+1)
		p0, p3 := p1, p2
		if k > 0 {
			p0 = get(k - 1)
		} else {
			p0 = lerp(p2, p1, 2) // mirror, so the edge's slope carries on
		}
		if k+2 < n {
			p3 = get(k + 2)
		} else {
			p3 = lerp(p1, p2, 2)
		}
		return hermite(p0, p1, p2, p3, t)
	}
	// Android draws with 16-bit indices: keep under 32,768 vertices.
	for detail > 1 && ((cols-1)*detail+1)*((rows-1)*detail+1) > 32767 {
		detail--
	}
	W, H := (cols-1)*detail, (rows-1)*detail
	pos := make([]byte, 0, (W+1)*(H+1)*8)
	col := make([]byte, 0, (W+1)*(H+1)*16)
	put := func(b []byte, fs ...float64) []byte {
		for _, f := range fs {
			b = binary.LittleEndian.AppendUint32(b, math.Float32bits(float32(f)))
		}
		return b
	}
	for y := 0; y <= H; y++ {
		ky, ty := span(y, detail, rows)
		for x := 0; x <= W; x++ {
			kx, tx := span(x, detail, cols)
			// Each column's curve at this height, then across them.
			column := func(i int) sample {
				return curve(func(j int) sample { return at(i, j) }, rows, ky, ty)
			}
			s := curve(column, cols, kx, tx)
			pos = put(pos, s.x, s.y)
			col = put(col, clamp(s.r), clamp(s.g), clamp(s.b), clamp(s.a))
		}
	}
	return grid{W: W, H: H, Positions: base64.StdEncoding.EncodeToString(pos), Colors: base64.StdEncoding.EncodeToString(col)}, true
}

// span finds which pair of control points step n of the cut falls between.
func span(n, detail, count int) (int, float64) {
	k := n / detail
	if k >= count-1 {
		return count - 2, 1
	}
	return k, float64(n%detail) / float64(detail)
}

// sample is a place and a color, blended as six numbers.
type sample struct{ x, y, r, g, b, a float64 }

func (s sample) each(o sample, f func(u, v float64) float64) sample {
	return sample{f(s.x, o.x), f(s.y, o.y), f(s.r, o.r), f(s.g, o.g), f(s.b, o.b), f(s.a, o.a)}
}

func lerp(a, b sample, t float64) sample {
	return a.each(b, func(u, v float64) float64 { return u + (v-u)*t })
}

// hermite is the Catmull-Rom curve through p1 and p2 at t, its slopes set by
// the points either side.
func hermite(p0, p1, p2, p3 sample, t float64) sample {
	t2, t3 := t*t, t*t*t
	f := func(p0, p1, p2, p3 float64) float64 {
		m1, m2 := (p2-p0)/2, (p3-p1)/2
		return (2*t3-3*t2+1)*p1 + (t3-2*t2+t)*m1 + (-2*t3+3*t2)*p2 + (t3-t2)*m2
	}
	return sample{f(p0.x, p1.x, p2.x, p3.x), f(p0.y, p1.y, p2.y, p3.y), f(p0.r, p1.r, p2.r, p3.r),
		f(p0.g, p1.g, p2.g, p3.g), f(p0.b, p1.b, p2.b, p3.b), f(p0.a, p1.a, p2.a, p3.a)}
}

func clamp(f float64) float64 { return math.Max(0, math.Min(1, f)) }
