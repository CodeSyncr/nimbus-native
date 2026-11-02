package meshgradient

import (
	"encoding/base64"
	"encoding/binary"
	"math"
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui"
)

func decode(t *testing.T, s string) []float32 {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]float32, len(b)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-3 }

func TestGrid(t *testing.T) {
	red, blue, white := ui.Hex("#ff0000"), ui.Hex("#0000ff"), ui.Hex("#ffffff")
	v := View{Columns: 3, Rows: 2, Detail: 4, Colors: []ui.Color{red, white, blue, red, white, blue}}
	g, ok := v.grid()
	if !ok || g.W != 8 || g.H != 4 {
		t.Fatalf("%v %+v", ok, g)
	}
	pos, col := decode(t, g.Positions), decode(t, g.Colors)
	if len(pos) != 9*5*2 || len(col) != 9*5*4 {
		t.Fatalf("%d %d", len(pos), len(col))
	}
	vert := func(x, y int) (ui.Point, [4]float32) {
		i := y*9 + x
		return ui.Point{X: pos[i*2], Y: pos[i*2+1]}, [4]float32{col[i*4], col[i*4+1], col[i*4+2], col[i*4+3]}
	}
	// The curve passes through every control point, with its color.
	for _, c := range []struct {
		x, y int
		p    ui.Point
		rgb  [3]float32
	}{{0, 0, ui.Point{X: 0, Y: 0}, [3]float32{1, 0, 0}}, {4, 0, ui.Point{X: 0.5, Y: 0}, [3]float32{1, 1, 1}}, {8, 4, ui.Point{X: 1, Y: 1}, [3]float32{0, 0, 1}}} {
		p, k := vert(c.x, c.y)
		if !near(p.X, c.p.X) || !near(p.Y, c.p.Y) || !near(k[0], c.rgb[0]) || !near(k[1], c.rgb[1]) || !near(k[2], c.rgb[2]) || !near(k[3], 1) {
			t.Errorf("(%d,%d) = %v %v", c.x, c.y, p, k)
		}
	}
	// Even points stay evenly spaced, and colors stay in range in between.
	if p, _ := vert(2, 1); !near(p.X, 0.25) || !near(p.Y, 0.25) {
		t.Errorf("(2,1) at %v", p)
	}
	for _, f := range col {
		if f < 0 || f > 1 {
			t.Fatalf("color %v out of range", f)
		}
	}
}

func TestBentPoints(t *testing.T) {
	c := ui.Hex("#808080")
	pts := []ui.Point{{X: 0, Y: 0}, {X: 0.5, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 0.5}, {X: 0.7, Y: 0.3}, {X: 1, Y: 0.5}, {X: 0, Y: 1}, {X: 0.5, Y: 1}, {X: 1, Y: 1}}
	v := View{Columns: 3, Rows: 3, Points: pts, Detail: 2, Colors: []ui.Color{c, c, c, c, c, c, c, c, c}}
	g, _ := v.grid()
	pos := decode(t, g.Positions)
	i := 2*5 + 2 // the middle control point
	if !near(pos[i*2], 0.7) || !near(pos[i*2+1], 0.3) {
		t.Fatalf("middle at %v,%v", pos[i*2], pos[i*2+1])
	}
}

func TestBadShapes(t *testing.T) {
	for _, v := range []View{
		{Columns: 1, Rows: 2, Colors: make([]ui.Color, 2)},
		{Columns: 2, Rows: 2, Colors: make([]ui.Color, 3)},
		{Columns: 2, Rows: 2, Colors: make([]ui.Color, 4), Points: make([]ui.Point, 3)},
	} {
		if _, ok := v.grid(); ok {
			t.Errorf("%+v passed", v)
		}
	}
}
