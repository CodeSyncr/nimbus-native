package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/appreview"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// RateUs asks for a rating after something went well. The system decides
// whether the prompt really shows, so nothing here depends on it.
type RateUs struct{}

func askForRating(ctx context.Context, _ struct{}) (string, error) {
	return "Thanks for your order!", appreview.Request(ctx)
}

func (RateUs) Render(c *ui.Ctx) ui.Component {
	ask := ui.UseAction(c, askForRating)
	status := ask.Result
	if ask.Err != nil {
		status = ask.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Finish order", OnPress: func() { ask.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
