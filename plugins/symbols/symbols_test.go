package symbols

import (
	"testing"

	"github.com/CodeSyncr/nimbus-native/ui"
)

func TestDefaults(t *testing.T) {
	th := ui.Theme{Text: ui.Hex("#111111")}
	s := Symbol{IOS: "heart"}.defaults(th)
	if s.Size != 24 || s.Weight != 400 || s.Color != th.Text {
		t.Fatalf("%+v", s)
	}
	s = Symbol{Size: 30, Weight: 700, Color: ui.Hex("#ff0000")}.defaults(th)
	if s.Size != 30 || s.Weight != 700 || s.Color != ui.Hex("#ff0000") {
		t.Fatalf("%+v", s)
	}
}
