package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/plugins/signin"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Welcome offers both. Send u.IDToken to your server, and have it call
// signin.Verify: what a phone says proves nothing by itself.
type Welcome struct{}

func greet(u signin.User, err error) (string, error) {
	if plugin.IsCode(err, plugin.Cancelled) {
		return "", nil
	}
	return "Hello, " + u.Name, err
}

func withApple(ctx context.Context, _ struct{}) (string, error) {
	return greet(signin.Apple(ctx, signin.Options{}))
}

func withGoogle(ctx context.Context, _ struct{}) (string, error) {
	return greet(signin.Google(ctx, signin.Options{
		IOSClient:     "123-ios.apps.googleusercontent.com",
		AndroidClient: "123-web.apps.googleusercontent.com",
	}))
}

func (Welcome) Render(c *ui.Ctx) ui.Component {
	apple, google := ui.UseAction(c, withApple), ui.UseAction(c, withGoogle)
	status := apple.Result + google.Result
	if apple.Err != nil {
		status = apple.Err.Error()
	}
	if google.Err != nil {
		status = google.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Continue with Google", OnPress: func() { google.Run(struct{}{}) }},
		ui.Button{Label: "Continue with Apple", Variant: ui.Tonal, OnPress: func() { apple.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
