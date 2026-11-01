package components

import (
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Tile follows a finger, and says which way it was flicked.
type Tile struct{}

func (Tile) Render(c *ui.Ctx) ui.Component {
	th := ui.UseTheme(c)
	x, setX := ui.State(c, float32(0))
	from := ui.Ref(c, float32(0))
	flick, setFlick := ui.State(c, "Flick me")
	names := map[ui.Swipe]string{ui.SwipeLeft: "Left", ui.SwipeRight: "Right", ui.SwipeUp: "Up", ui.SwipeDown: "Down"}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 24},
		ui.View(ui.Style{Height: ui.Pt(80)},
			ui.View(ui.Style{Absolute: true, Left: ui.Pt(x), Width: ui.Pt(80), Height: ui.Pt(80), Radius: 18, Background: th.Primary}).OnPanX(func(p ui.Pan) {
				if p.Phase == ui.Began {
					*from = x
				}
				setX(*from + p.X)
			})),
		ui.View(ui.Style{Height: ui.Pt(100), Radius: 18, Background: th.Surface, AlignItems: ui.AlignCenter, Justify: ui.JustifyCenter},
			ui.Text(flick, ui.Style{})).OnSwipe(func(s ui.Swipe) { setFlick(names[s]) }))
}
