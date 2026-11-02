package applemaps

import (
	"encoding/json"
	"testing"

	"github.com/CodeSyncr/nimbus-native/plugins/googlemaps"
	"github.com/CodeSyncr/nimbus-native/ui"
)

func TestProps(t *testing.T) {
	th := ui.Theme{Primary: ui.Hex("#0d7377")}
	p := Map{Lines: []Polyline{{Points: []LatLng{{1, 2}}}, {Color: ui.Hex("#ff0000"), Width: 2}}}.props(th)
	if p.Zoom != 12 || p.Markers == nil || p.Kind != Standard {
		t.Fatalf("props = %+v", p)
	}
	if p.Lines[0].Color != uint32(th.Primary) || p.Lines[0].Width != 4 || p.Lines[1].Color != uint32(ui.Hex("#ff0000")) || p.Lines[1].Width != 2 {
		t.Fatalf("lines = %+v", p.Lines)
	}
	b, _ := json.Marshal(Map{Kind: Hybrid, Markers: []Marker{{ID: "a", At: LatLng{1, 2}}}}.props(th))
	var back map[string]any
	json.Unmarshal(b, &back)
	if back["kind"] != "hybrid" || back["markers"].([]any)[0].(map[string]any)["at"].(map[string]any)["lat"] != 1.0 {
		t.Fatalf("json = %s", b)
	}
}

func TestEvents(t *testing.T) {
	var tapped LatLng
	var pin string
	var moved float32
	m := Map{
		OnTap:    func(at LatLng) { tapped = at },
		OnMarker: func(id string) { pin = id },
		OnMove:   func(_ LatLng, z float32) { moved = z },
	}
	m.event("tap", `{"at":{"lat":1.5,"lng":2.5}}`)
	m.event("marker", `{"id":"cafe"}`)
	m.event("move", `{"at":{"lat":0,"lng":0},"zoom":15.5}`)
	m.event("move", `junk`)
	if tapped != (LatLng{1.5, 2.5}) || pin != "cafe" || moved != 15.5 {
		t.Fatalf("tap %v, pin %q, zoom %v", tapped, pin, moved)
	}
	Map{}.event("tap", `{}`) // no handlers: nothing happens
}

// The types match googlemaps', so a screen can convert between them.
func TestSameAsGoogle(t *testing.T) {
	_ = LatLng(googlemaps.LatLng{})
	_ = googlemaps.Polyline{Points: []googlemaps.LatLng{googlemaps.LatLng(LatLng{})}}
}
