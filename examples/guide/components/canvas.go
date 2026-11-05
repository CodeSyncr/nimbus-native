package components

import "github.com/CodeSyncr/nimbus-native/ui"

// Bakery draws a day at the bakery on one canvas: a ring that fills to the
// day's goal, a spinner, a card with a shadow, a photo and wrapped text, a
// heart that beats, and colors that blend. The phone runs the animations.
type Bakery struct{}

const heart = "M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"

func (Bakery) Render(c *ui.Ctx) ui.Component {
	th := ui.UseTheme(c)
	fill := ui.UseAnimation(c, ui.Animation{Ms: 900, Delay: 200, Ease: ui.Spring, Bounce: 0.2})
	spin := ui.UseAnimation(c, ui.Animation{Ms: 900, Ease: ui.Linear, Repeat: ui.Forever})
	beat := ui.UseAnimation(c, ui.Animation{Ms: 450, Repeat: ui.Forever, Reverse: true})
	teal, coral, ink := ui.Hex("#2A9D8F"), ui.Hex("#E76F51"), ui.Hex("#264653")

	return ui.Scroll(ui.Style{Grow: 1}, ui.SafeArea(ui.Style{Padding: ui.All(16)},
		ui.Canvas(340, 600, func(p *ui.Painter) {
			// The day's goal: a track, then a ring that fills to 72% around it.
			p.Arc(85, 85, 60, 0, 360)
			p.Stroke(14, ui.Hex("#2646531A"))
			p.Arc(85, 85, 60, -90, 360)
			p.Trim(ui.Fixed(0), fill.Range(0, 0.72))
			p.StrokeGradient(14, &ui.Gradient{Sweep: true, From: ui.Point{X: 85, Y: 85}, To: ui.Point{X: 85, Y: 0},
				Colors: []ui.Color{teal, coral, teal}})
			p.Text("72%", 25, 66, ui.Style{Width: ui.Pt(120), FontSize: 30, FontWeight: 700, TextAlign: ui.AlignCenter, Color: ink})
			p.Text("of today's bakes", 25, 102, ui.Style{Width: ui.Pt(120), FontSize: 12, TextAlign: ui.AlignCenter, Color: th.TextMuted})

			// A spinner, turned by the phone.
			p.Motion(ui.Motion{X: ui.Fixed(255), Y: ui.Fixed(85), Rotate: spin.Range(0, 360)})
			p.Arc(0, 0, 30, 0, 270)
			p.Stroke(6, th.Primary)
			p.Restore()

			// A card with a soft shadow, a photo clipped to its corners, and text
			// that wraps.
			p.Save()
			p.Shadow(0, 8, 24, ui.Hex("#26465333"))
			p.Rect(8, 190, 324, 130, 20)
			p.Fill(ui.Hex("#FFFFFF"))
			p.Restore()
			p.Save()
			p.Rect(24, 206, 98, 98, 14)
			p.Clip()
			p.Image("https://www.w3schools.com/css/img_5terre.jpg", 24, 206, 98, 98, ui.FitCover)
			p.Restore()
			p.Text("Cardamom bun", 138, 214, ui.Style{FontSize: 20, FontWeight: 700, Color: ink})
			p.Text("Baked this morning with butter, sugar and freshly ground cardamom.", 138, 244,
				ui.Style{Width: ui.Pt(180), FontSize: 14, Lines: 3, Color: ui.Hex("#5B6B73")})

			// A dashed route to a heart that beats.
			p.Dash(8, 8)
			p.LineStyle(ui.CapButt, ui.JoinRound)
			p.MoveTo(24, 400)
			p.QuadTo(120, 330, 200, 400)
			p.Stroke(3, ink)
			p.Dash()
			p.LineStyle(ui.CapRound, ui.JoinRound)
			p.Motion(ui.Motion{X: ui.Fixed(260), Y: ui.Fixed(395), Scale: beat.Range(2.6, 3.2)})
			p.Translate(-12, -12)
			p.Path(heart)
			p.Fill(coral)
			p.Restore()

			// A ring cut from two circles, by the even-odd rule.
			p.Circle(60, 500, 44)
			p.Circle(60, 500, 26)
			p.EvenOdd()
			p.FillGradient(&ui.Gradient{From: ui.Point{X: 16, Y: 456}, To: ui.Point{X: 104, Y: 544}, Colors: []ui.Color{teal, ink}})

			// Three colors that multiply where they overlap.
			p.Save()
			p.Blend(ui.BlendMultiply)
			for i, col := range []ui.Color{ui.Hex("#4CC9F0"), ui.Hex("#F72585"), ui.Hex("#FFD60A")} {
				x, y := []float32{170, 210, 190}[i], []float32{480, 480, 515}[i]
				p.Circle(x, y, 34)
				p.Fill(col)
			}
			p.Restore()

			// Stripes that fade out to the right: drawn in a layer, then erased
			// along a gradient.
			p.Layer(1)
			for i := 0; i < 6; i++ {
				p.Rect(250, 450+float32(i)*18, 80, 10, 5)
				p.Fill(teal)
			}
			p.Blend(ui.BlendErase)
			p.Rect(250, 450, 80, 110, 0)
			p.FillGradient(&ui.Gradient{From: ui.Point{X: 250, Y: 0}, To: ui.Point{X: 330, Y: 0},
				Colors: []ui.Color{ui.Hex("#00000000"), ui.Hex("#000000FF")}})
			p.Restore()
		}, ui.Style{Width: ui.Pct(100), Height: ui.Pt(600)})))
}
