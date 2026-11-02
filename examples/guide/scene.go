package guide

import (
	"github.com/CodeSyncr/nimbus-native/plugins/scene"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Showroom turns a cup on a table, and says what was tapped.
type Showroom struct{}

func (Showroom) Render(c *ui.Ctx) ui.Component {
	picked, setPicked := ui.State(c, "Tap the cup")
	return ui.SafeArea(ui.Style{Grow: 1},
		scene.View{
			Camera: scene.Camera{Position: scene.V(0, 1.6, 4), Target: scene.V(0, 0.4, 0)},
			Lights: []scene.Light{{Direction: scene.V(-0.4, -1, -0.5)}},
			Nodes: []scene.Node{
				{ID: "table", Mesh: scene.Box(3, 0.1, 2), Color: ui.Hex("#8a5a3b")},
				{ID: "cup", Mesh: scene.Box(0.7, 0.8, 0.7), Color: ui.Hex("#0d7377"), Shine: 0.6,
					Position: scene.V(0, 0.45, 0), Spin: scene.Spin{Speed: 40},
					Children: []scene.Node{{ID: "coffee", Mesh: scene.Sphere(0.25), Color: ui.Hex("#4b2e1e"), Position: scene.V(0, 0.45, 0)}}},
			},
			OnTap: func(id string) {
				if id == "" {
					id = "nothing"
				}
				setPicked("Tapped " + id)
			},
			Style: ui.Style{Grow: 1},
		},
		ui.Text(picked, ui.Style{Padding: ui.All(16)}))
}
