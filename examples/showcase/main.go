// showcase is every component on a screen of its own, the source of the
// guide's pictures. SHOWCASE=<name> opens one directly.
package main

import (
	"fmt"
	"os"

	"github.com/CodeSyncr/nimbus-native/ui"
)

func init() { ui.Mount(App{}) }

// main runs only under `nimbus native dev`; on a device the host owns the process.
func main() { ui.Main() }

// App opens the screen SHOWCASE names.
type App struct{}

func (App) Render(c *ui.Ctx) *ui.Element {
	switch os.Getenv("SHOWCASE") {
	default:
		return ui.C(gestures{})
	}
}

// gestures drags a tile, flicks a deck and pinches a square.
type gestures struct{}

func (gestures) Render(c *ui.Ctx) *ui.Element {
	th := ui.UseTheme(c)
	x, setX := ui.State(c, float32(0))
	y, setY := ui.State(c, float32(0))
	from := ui.Ref(c, [2]float32{})
	swipes, setSwipes := ui.State(c, "none yet")
	scale, setScale := ui.State(c, float32(1))
	return ui.SafeArea(ui.Style{Grow: 1, Padding: ui.All(24), Gap: 20, Background: th.Background},
		ui.Text("Drag", ui.Style{FontWeight: 600}),
		ui.View(ui.Style{Height: ui.Pt(160)},
			ui.View(ui.Style{Absolute: true, Left: ui.Pt(x), Top: ui.Pt(y), Width: ui.Pt(80), Height: ui.Pt(80), Radius: 18,
				Background: th.Primary}).OnPan(func(p ui.Pan) {
				if p.Phase == ui.Began {
					*from = [2]float32{x, y}
				}
				setX(from[0] + p.X)
				setY(from[1] + p.Y)
			}).TestID("tile")),
		ui.Text("Swipe: "+swipes, ui.Style{FontWeight: 600}).TestID("swiped"),
		ui.View(ui.Style{Height: ui.Pt(90), Radius: 18, Background: th.Surface, Justify: ui.JustifyCenter, AlignItems: ui.AlignCenter},
			ui.Text("Flick me", ui.Style{Color: th.TextMuted})).OnSwipe(func(d ui.Swipe) {
			setSwipes(map[ui.Swipe]string{ui.SwipeLeft: "left", ui.SwipeRight: "right", ui.SwipeUp: "up", ui.SwipeDown: "down"}[d])
		}),
		ui.Text(fmt.Sprintf("Pinch: %.2f", scale), ui.Style{FontWeight: 600}).TestID("pinched"),
		ui.View(ui.Style{Height: ui.Pt(90), Radius: 18, Background: th.Surface}).OnPinch(func(p ui.Pinch) { setScale(p.Scale) }),
	)
}
