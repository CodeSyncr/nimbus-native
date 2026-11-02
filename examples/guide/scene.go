package guide

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"github.com/CodeSyncr/nimbus-native/plugins/scene"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Showroom shows a gift box on a table, in shadow, to turn with a finger.
// For a model from Blender or a store, scene.LoadGLTF reads its .glb.
type Showroom struct{}

// wrapping is the gift paper: stripes, made here so the example needs no file.
var wrapping = func() *scene.Texture {
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			c := color.RGBA{13, 115, 119, 255}
			if (x+y)/16%2 == 0 {
				c = color.RGBA{244, 225, 193, 255}
			}
			img.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	t, _ := scene.NewTexture(b.Bytes()) // a PNG made here is always valid
	return t
}()

func (Showroom) Render(c *ui.Ctx) ui.Component {
	picked, setPicked := ui.State(c, "Drag to turn it, pinch to zoom")
	return ui.SafeArea(ui.Style{Grow: 1},
		scene.View{
			Camera:  scene.Camera{Position: scene.V(0, 1.8, 4), Target: scene.V(0, 0.3, 0)},
			Orbit:   true,
			Shadows: true,
			Lights:  []scene.Light{{Direction: scene.V(-0.4, -1, -0.5), Intensity: 0.9}},
			Nodes: []scene.Node{
				{ID: "table", Mesh: scene.Box(3, 0.1, 2), Color: ui.Hex("#8a5a3b")},
				{ID: "gift", Mesh: scene.Box(0.8, 0.8, 0.8), Texture: wrapping, Shine: 0.3,
					Position: scene.V(-0.5, 0.45, 0), Spin: scene.Spin{Speed: 20},
					Children: []scene.Node{{ID: "ribbon", Mesh: scene.Box(0.82, 0.1, 0.82), Color: ui.Hex("#c0392b"), Position: scene.V(0, 0.15, 0)}}},
				{ID: "cup", Mesh: scene.Sphere(0.3), Color: ui.Hex("#f4f1ea"), Shine: 0.8, Position: scene.V(0.8, 0.35, 0.2)},
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
