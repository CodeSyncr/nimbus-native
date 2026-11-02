// Package applemaps shows Apple Maps inside your screen, on iOS. It needs no
// key and no billing account. Apple Maps has no Android SDK, so on Android
// it shows Fallback: a googlemaps.Map, say, or a list of places.
//
//	nimbus native plugin add apple-maps
//
//	applemaps.Map{
//		Center: applemaps.LatLng{Latitude: 18.53, Longitude: 73.89}, Zoom: 14,
//		Markers: []applemaps.Marker{{ID: "cafe", At: cafe, Title: "Monsoon Coffee"}},
//		OnMarker: open,
//		Style: ui.Style{Grow: 1},
//	}
//
// LatLng and Polyline match googlemaps', field for field, so they convert
// with a cast, and one screen can feed both maps.
package applemaps

import (
	"encoding/json"

	"github.com/CodeSyncr/nimbus-native/ui"
)

// LatLng is a place.
type LatLng struct {
	Latitude  float64 `json:"lat"` // in degrees
	Longitude float64 `json:"lng"` // in degrees
}

// Marker is a pin.
type Marker struct {
	ID       string `json:"id"`                 // your name for it, sent to OnMarker
	At       LatLng `json:"at"`                 // where it stands
	Title    string `json:"title,omitempty"`    // shown under the pin; optional
	Subtitle string `json:"subtitle,omitempty"` // shown when it's selected; optional
}

// Polyline is a line through places, such as a route.
type Polyline struct {
	Points []LatLng `json:"points"` // the places it goes through
	Color  ui.Color `json:"color"`  // the theme's Primary when 0
	Width  float32  `json:"width"`  // points; 0 means 4
}

// Kind is how the map is drawn.
type Kind string

// Kinds of map.
const (
	Standard  Kind = ""          // streets and places
	Satellite Kind = "satellite" // photos from above
	Hybrid    Kind = "hybrid"    // photos, with streets and names
)

// Map is a map. Changing Center or Zoom moves it there; panning by hand
// doesn't change them, and OnMove says where it stopped.
type Map struct {
	Center     LatLng     // where it looks
	Zoom       float32    // as on Google Maps: 0 is the world, 20 a building; 0 means 12
	Markers    []Marker   // the pins
	Lines      []Polyline // routes and other lines
	Kind       Kind       // Standard, Satellite or Hybrid
	MyLocation bool       // the blue dot; needs the location permission

	OnTap    func(LatLng)          // the map itself
	OnMarker func(id string)       // a pin
	OnMove   func(LatLng, float32) // after a pan or zoom ends, with the zoom
	Style    ui.Style              // the map's size and place on the screen

	// Fallback is shown instead on Android, in the same Style. Nil shows an
	// empty space.
	Fallback ui.Component
}

type props struct {
	Center     LatLng   `json:"center"`
	Zoom       float32  `json:"zoom"`
	Markers    []Marker `json:"markers"`
	Lines      []line   `json:"lines"`
	Kind       Kind     `json:"kind"`
	MyLocation bool     `json:"myLocation"`
}

type line struct {
	Points []LatLng `json:"points"`
	Color  uint32   `json:"color"`
	Width  float32  `json:"width"`
}

func (m Map) props(th ui.Theme) props {
	p := props{Center: m.Center, Zoom: m.Zoom, Markers: m.Markers, Kind: m.Kind, MyLocation: m.MyLocation}
	if p.Zoom == 0 {
		p.Zoom = 12
	}
	if p.Markers == nil {
		p.Markers = []Marker{}
	}
	p.Lines = make([]line, len(m.Lines))
	for i, l := range m.Lines {
		color, width := l.Color, l.Width
		if color == 0 {
			color = th.Primary
		}
		if width == 0 {
			width = 4
		}
		p.Lines[i] = line{l.Points, uint32(color), width}
	}
	return p
}

// Render shows the map, or Fallback on Android.
func (m Map) Render(c *ui.Ctx) ui.Component {
	if ui.Viewport(c).Platform == "android" {
		if m.Fallback == nil {
			return ui.View(m.Style)
		}
		return ui.View(m.Style, m.Fallback)
	}
	return ui.Native("applemaps", m.props(ui.UseTheme(c)), m.Style).OnEvent(m.event)
}

func (m Map) event(name, data string) {
	var e struct {
		ID   string  `json:"id"`
		At   LatLng  `json:"at"`
		Zoom float32 `json:"zoom"`
	}
	if json.Unmarshal([]byte(data), &e) != nil {
		return
	}
	switch {
	case name == "tap" && m.OnTap != nil:
		m.OnTap(e.At)
	case name == "marker" && m.OnMarker != nil:
		m.OnMarker(e.ID)
	case name == "move" && m.OnMove != nil:
		m.OnMove(e.At, e.Zoom)
	}
}
