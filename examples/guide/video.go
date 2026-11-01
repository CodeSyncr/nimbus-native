package guide

import (
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugins/video"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Tour plays a clip, with a button of your own.
type Tour struct{}

func (Tour) Render(c *ui.Ctx) ui.Component {
	paused, setPaused := ui.State(c, false)
	length, setLength := ui.State(c, 0.0)
	label := "Pause"
	if paused {
		label = "Play"
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		video.Player{
			URL:     "https://www.w3schools.com/html/mov_bbb.mp4",
			Paused:  paused,
			Muted:   true,
			Loop:    true,
			OnReady: setLength,
			Style:   ui.Style{Height: ui.Pt(200)},
		},
		ui.Text(fmt.Sprintf("%.0f seconds", length), ui.Style{}),
		ui.Button{Label: label, OnPress: func() { setPaused(!paused) }})
}
