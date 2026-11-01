package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/trackingconsent"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Consent explains first, then asks.
type Consent struct{}

func ask(ctx context.Context, _ struct{}) (string, error) {
	s, err := trackingconsent.Request(ctx)
	if err != nil {
		return "", err
	}
	if s.Allowed() {
		return "Personal offers are on", nil
	}
	return "Everything still works, with general offers", nil
}

func (Consent) Render(c *ui.Ctx) ui.Component {
	consent := ui.UseAction(c, ask)
	status := consent.Result
	if consent.Err != nil {
		status = consent.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Text("We'd like to show offers you might enjoy.", ui.Style{FontSize: 20}),
		ui.Button{Label: "Continue", OnPress: func() { consent.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
