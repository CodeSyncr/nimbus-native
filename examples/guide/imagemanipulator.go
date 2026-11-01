package guide

import (
	"fmt"
	"image"
	"image/color"

	"github.com/CodeSyncr/nimbus-native/plugins/imagemanipulator"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Thumbnail shrinks and turns a picture, and shows what it cost. A real app
// starts from camera.Pick and imagemanipulator.Decode.
type Thumbnail struct{}

func sample() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 1600, 900))
	for y := 0; y < 900; y++ {
		for x := 0; x < 1600; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x / 7), uint8(y / 4), 160, 255})
		}
	}
	return img
}

func (Thumbnail) Render(c *ui.Ctx) ui.Component {
	line, setLine := ui.State(c, "")
	shrink := func() {
		go func() {
			img := imagemanipulator.Rotate(imagemanipulator.Fit(sample(), 400), 90)
			b, err := imagemanipulator.JPEG(img, 85)
			if err != nil {
				setLine(err.Error())
				return
			}
			setLine(fmt.Sprintf("%d×%d, %d KB", img.Bounds().Dx(), img.Bounds().Dy(), len(b)/1024))
		}()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Make a thumbnail", OnPress: shrink},
		ui.Text(line, ui.Style{}))
}
