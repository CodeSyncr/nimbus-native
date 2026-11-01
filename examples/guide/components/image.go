package components

import "github.com/CodeSyncr/nimbus-native/ui"

// Photo shows a picture filling its frame, and one fitted inside it.
type Photo struct{}

func (Photo) Render(c *ui.Ctx) ui.Component {
	url := "https://www.w3schools.com/css/img_5terre.jpg"
	return ui.SafeArea(ui.Style{Padding: ui.All(16), Gap: 12},
		ui.Image(url, ui.Style{Height: ui.Pt(180), Radius: 16}),
		ui.Image(url, ui.Style{Height: ui.Pt(120)}).Fit(ui.FitContain))
}
