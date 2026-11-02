package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugin"
	"github.com/CodeSyncr/nimbus-native/plugins/oauth"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// WorkAccount signs in with Microsoft. Keep the refresh token in securestore,
// and send the ID token to your server to check, as you would signin's.
type WorkAccount struct{}

var microsoft = oauth.Config{
	Endpoint:    oauth.Microsoft("common"),
	ClientID:    "00000000-0000-0000-0000-000000000000",
	RedirectURL: "space.nimbusgo.cafe://auth",
	Scopes:      []string{"openid", "profile", "offline_access", "User.Read"},
}

func withMicrosoft(ctx context.Context, _ struct{}) (string, error) {
	t, err := oauth.SignIn(ctx, microsoft)
	if plugin.IsCode(err, plugin.Cancelled) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return "Signed in until " + t.Expiry.Format("3:04 pm"), nil
}

func (WorkAccount) Render(c *ui.Ctx) ui.Component {
	signIn := ui.UseAction(c, withMicrosoft)
	status := signIn.Result
	if signIn.Err != nil {
		status = signIn.Err.Error()
	}
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Sign in with Microsoft", OnPress: func() { signIn.Run(struct{}{}) }},
		ui.Text(status, ui.Style{}))
}
