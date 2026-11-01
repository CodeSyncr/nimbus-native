package guide

import (
	"context"
	"fmt"

	"github.com/CodeSyncr/nimbus-native/plugins/cameraview"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Viewfinder shows the camera with buttons of its own over it.
type Viewfinder struct{}

func shoot(ctx context.Context, _ struct{}) (string, error) {
	p, err := cameraview.Take(ctx, cameraview.Options{MaxSize: 1600})
	return fmt.Sprintf("%d×%d, %d KB", p.Width, p.Height, len(p.Data)/1024), err
}

func (Viewfinder) Render(c *ui.Ctx) ui.Component {
	allowed, setAllowed := ui.State(c, false)
	front, setFront := ui.State(c, false)
	torch, setTorch := ui.State(c, false)
	problem, setProblem := ui.State(c, "")
	take := ui.UseAction(c, shoot)
	ui.Effect(c, func(ctx context.Context) func() {
		go func() {
			ok, err := cameraview.Request(ctx)
			if err != nil || !ok {
				setProblem("The camera is off")
				return
			}
			setAllowed(true)
		}()
		return nil
	}, ui.Once)
	status := take.Result
	if take.Err != nil {
		status = take.Err.Error()
	}
	if problem != "" {
		status = problem
	}
	camera := ui.Component(ui.View(ui.Style{Grow: 1, Background: ui.Hex("#000000")}))
	if allowed {
		camera = cameraview.View{Front: front, Torch: torch, Style: ui.Style{Grow: 1},
			OnError: func(code string) { setProblem("No camera here: " + code) }}
	}
	return ui.SafeArea(ui.Style{Grow: 1, Gap: 12, Padding: ui.All(12)},
		camera,
		ui.Text(status, ui.Style{}),
		ui.View(ui.Style{Direction: ui.Row, Gap: 8},
			ui.Button{Label: "Take", Loading: take.Running, OnPress: func() { take.Run(struct{}{}) }},
			ui.Button{Label: "Flip", Variant: ui.Tonal, OnPress: func() {
				setProblem("")
				setFront(!front)
			}},
			ui.Button{Label: "Light", Variant: ui.Tonal, OnPress: func() { setTorch(!torch) }}))
}
