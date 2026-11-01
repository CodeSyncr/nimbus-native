package components

import (
	"fmt"

	"github.com/CodeSyncr/nimbus-native/ui"
)

// Offers shows one page at a time.
type Offers struct{}

func (Offers) Render(c *ui.Ctx) ui.Component {
	th := ui.UseTheme(c)
	at, setAt := ui.State(c, 0)
	pages := []ui.Component{}
	for _, name := range []string{"Free cookie", "Two for one", "Monsoon Club"} {
		pages = append(pages, ui.View(ui.Style{Grow: 1, Margin: ui.All(16), Radius: 20, Background: th.Surface, AlignItems: ui.AlignCenter, Justify: ui.JustifyCenter},
			ui.Text(name, ui.Style{FontSize: 20})))
	}
	return ui.SafeArea(ui.Style{},
		ui.Carousel{Pages: pages, Height: 220, OnChange: setAt},
		ui.Text(fmt.Sprintf("Page %d of 3", at+1), ui.Style{Padding: ui.All(16)}))
}
