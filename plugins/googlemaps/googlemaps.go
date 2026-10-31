// Package googlemaps shows Google Maps inside your screen.
//
//	nimbus native plugin add google-maps -key YOUR_API_KEY
//
//	googlemaps.Map{
//		Center: googlemaps.LatLng{Latitude: 18.53, Longitude: 73.89}, Zoom: 14,
//		Markers: []googlemaps.Marker{{ID: "cafe", At: cafe, Title: "Monsoon Coffee"}},
//		OnMarker: open,
//		Style: ui.Style{Grow: 1},
//	}
//
// The key goes in the app, where anyone can read it: in Google Cloud, limit
// it to your app and to the Maps SDKs. There are no maps on the web yet.
package googlemaps

import (
	"encoding/json"

	"github.com/CodeSyncr/nimbus-native/ui"
)

// LatLng is a place.
type LatLng struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
}

// Marker is a pin.
type Marker struct {
	ID    string `json:"id"`
	At    LatLng `json:"at"`
	Title string `json:"title,omitempty"`
}

// Polyline is a line through places, such as a route.
type Polyline struct {
	Points []LatLng `json:"points"`
	Color  ui.Color `json:"color"` // the theme's Primary when 0
	Width  float32  `json:"width"` // points; 0 means 4
}

// Map is a map. Changing Center or Zoom moves it there; panning by hand
// doesn't change them, and OnMove says where it stopped.
type Map struct {
	Center     LatLng
	Zoom       float32 // 0 means 12
	Markers    []Marker
	Lines      []Polyline
	MyLocation bool // the blue dot; needs the location permission

	OnTap    func(LatLng)          // the map itself
	OnMarker func(id string)       // a pin
	OnMove   func(LatLng, float32) // after a pan or zoom ends, with the zoom
	Style    ui.Style
}

type props struct {
	Center     LatLng   `json:"center"`
	Zoom       float32  `json:"zoom"`
	Markers    []Marker `json:"markers"`
	Lines      []line   `json:"lines"`
	MyLocation bool     `json:"myLocation"`
}

type line struct {
	Points []LatLng `json:"points"`
	Color  uint32   `json:"color"`
	Width  float32  `json:"width"`
}

// Render shows the map.
func (m Map) Render(c *ui.Ctx) ui.Component {
	th := ui.UseTheme(c)
	p := props{Center: m.Center, Zoom: m.Zoom, Markers: m.Markers, MyLocation: m.MyLocation}
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
	return ui.Native("googlemaps", p, m.Style).OnEvent(func(name, data string) {
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
	})
}
