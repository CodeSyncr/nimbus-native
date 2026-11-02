package splitview

import (
	"testing"

	"github.com/CodeSyncr/nimbus-native/plugins/hinge"
)

func TestPanes(t *testing.T) {
	v := View{}
	cases := []struct {
		name      string
		w         float32
		f         hinge.Fold
		wide      bool
		list, gap float32
	}{
		{"phone", 390, hinge.Fold{}, false, 320, 0},
		{"tablet", 1024, hinge.Fold{}, true, 320, 0},
		{"half-open book", 600, hinge.Fold{Present: true, Vertical: true, Separates: true, HalfOpen: true, X: 300}, true, 300, 0},
		{"dual screen", 1100, hinge.Fold{Present: true, Vertical: true, Separates: true, Occludes: true, X: 540, W: 20}, true, 540, 20},
		{"flat fold", 600, hinge.Fold{Present: true, Vertical: true, X: 300}, false, 320, 0},
		{"tabletop", 600, hinge.Fold{Present: true, Separates: true, HalfOpen: true, Y: 400}, false, 320, 0},
	}
	for _, c := range cases {
		wide, list, gap := v.panes(c.w, c.f)
		if wide != c.wide || (wide && (list != c.list || gap != c.gap)) {
			t.Errorf("%s: %v %v %v", c.name, wide, list, gap)
		}
	}
	if wide, list, _ := (View{MinWidth: 500, ListWidth: 280}).panes(520, hinge.Fold{}); !wide || list != 280 {
		t.Errorf("own sizes: %v %v", wide, list)
	}
}

func TestOpen(t *testing.T) {
	var set, pushed string
	st := &state{set: func(id string) { set = id }, push: func(id string) { pushed = id }}
	s := Split{st: st}
	s.Open("42")
	if set != "42" || pushed != "42" {
		t.Fatalf("phone: set %q pushed %q", set, pushed)
	}
	st.wide, pushed = true, ""
	s.Open("43")
	if set != "43" || pushed != "" {
		t.Fatalf("tablet: set %q pushed %q", set, pushed)
	}
	Split{}.Open("x") // outside Use: nothing happens
}
