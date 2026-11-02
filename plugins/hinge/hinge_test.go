package hinge

import (
	"encoding/json"
	"testing"
)

// What the Android probe sends decodes as a Fold.
func TestFoldJSON(t *testing.T) {
	var f Fold
	err := json.Unmarshal([]byte(`{"present":true,"halfOpen":true,"vertical":true,"separates":true,"occludes":false,"x":336.5,"y":0,"w":0,"h":840}`), &f)
	if err != nil || !f.Present || !f.HalfOpen || !f.Vertical || !f.Separates || f.Occludes || f.X != 336.5 || f.H != 840 {
		t.Fatalf("%+v %v", f, err)
	}
}
