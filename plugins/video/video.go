// Package video plays video inside your screen, from a URL, with the
// system's own player.
//
//	nimbus native plugin add video
//
//	video.Player{URL: "https://example.com/tour.mp4", Style: ui.Style{Height: ui.Pt(220)}}
package video

import (
	"encoding/json"

	"github.com/CodeSyncr/nimbus-native/ui"
)

// Player is a video.
type Player struct {
	URL      string
	Paused   bool // setting it pauses and resumes from Go
	Loop     bool
	Muted    bool
	Controls bool // the system's controls

	OnReady func(seconds float64) // it can play, and how long it is
	OnEnd   func()
	Style   ui.Style
}

// Render shows the player.
func (v Player) Render(c *ui.Ctx) ui.Component {
	p := map[string]any{"url": v.URL, "paused": v.Paused, "loop": v.Loop, "muted": v.Muted, "controls": v.Controls}
	return ui.Native("video", p, v.Style).OnEvent(func(name, data string) {
		switch {
		case name == "end" && v.OnEnd != nil:
			v.OnEnd()
		case name == "ready" && v.OnReady != nil:
			var e struct {
				Seconds float64 `json:"seconds"`
			}
			if json.Unmarshal([]byte(data), &e) == nil {
				v.OnReady(e.Seconds)
			}
		}
	})
}
