package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/plugins/livephoto"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Moment lets someone share a Live Photo of their latte art, and plays it
// back.
type Moment struct{}

func pickMoment(ctx context.Context, _ struct{}) (livephoto.Photo, error) {
	p, err := livephoto.Pick(ctx)
	if plugin.IsCode(err, plugin.Cancelled) {
		return livephoto.Photo{}, nil
	}
	return p, err
}

func (Moment) Render(c *ui.Ctx) ui.Component {
	pick := ui.UseAction(c, pickMoment)
	plays, setPlays := ui.State(c, 0)
	p := pick.Result
	kids := []ui.Component{ui.Button{Label: "Choose a Live Photo", OnPress: func() { pick.Run(struct{}{}) }}}
	if pick.Err != nil {
		kids = append(kids, ui.Text(pick.Err.Error(), ui.Style{}))
	}
	if p.ID != "" {
		kids = append(kids,
			livephoto.View{ID: p.ID, Play: plays, Fill: true, Style: ui.Style{Width: ui.Pt(320), Height: ui.Pt(320 / p.Aspect()), Radius: 16},
				Fallback: ui.Text("Live Photos are on iPhone.", ui.Style{})},
			ui.Button{Label: "Play it", Variant: ui.Tonal, OnPress: func() { setPlays(plays + 1) }})
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12}, kids...)
}
