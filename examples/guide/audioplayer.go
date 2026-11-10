package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugins/audioplayer"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Podcast plays a show, with a bar that follows it.
type Podcast struct{}

func (Podcast) Render(c *ui.Ctx) ui.Component {
	player, setPlayer := ui.State(c, (*audioplayer.Player)(nil))
	s, setStatus := ui.State(c, audioplayer.Status{})
	problem, setProblem := ui.State(c, "")
	ui.Effect(c, func(ctx context.Context) func() {
		var open *audioplayer.Player
		go func() {
			p, err := audioplayer.Open(ctx, audioplayer.Source{URL: "https://www.w3schools.com/html/horse.mp3"})
			if err != nil {
				setProblem(err.Error())
				return
			}
			open = p
			setPlayer(p)
			p.Watch(ctx, setStatus)
		}()
		return func() {
			if open != nil {
				go open.Close(context.WithoutCancel(ctx))
			}
		}
	}, ui.Once)
	if problem != "" {
		return ui.SafeArea(ui.Style{Padding: ui.All(24)}, ui.Text(problem, ui.Style{}))
	}
	if player == nil {
		return ui.SafeArea(ui.Style{Padding: ui.All(24)}, ui.Text("Loading…", ui.Style{}))
	}
	bg := c.Context()
	done := 0.0
	if s.Duration > 0 {
		done = s.Position / s.Duration
	}
	toggle, label := func() { go player.Play(bg) }, "Play"
	if s.Playing {
		toggle, label = func() { go player.Pause(bg) }, "Pause"
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.ProgressBar{Value: done},
		ui.Text(fmt.Sprintf("%.1f of %.1f seconds", s.Position, s.Duration), ui.Style{}),
		ui.Button{Label: label, OnPress: toggle},
		ui.Button{Label: "Start again", Variant: ui.Tonal, OnPress: func() { go player.Seek(bg, 0) }},
		ui.Button{Label: "Half speed", Variant: ui.Tonal, OnPress: func() { go player.SetRate(bg, 0.5) }})
}
