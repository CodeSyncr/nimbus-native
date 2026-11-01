package guide

import (
	"context"

	"github.com/CodeSyncr/nimbus-native/plugins/securestore"
	"github.com/CodeSyncr/nimbus-native/ui"
)

// Session keeps a sign-in token in the Keychain or Keystore.
type Session struct{}

func signIn(ctx context.Context, token string) (string, error) {
	if err := securestore.Set(ctx, "session", token); err != nil {
		return "", err
	}
	saved, ok, err := securestore.Get(ctx, "session")
	if err != nil || !ok {
		return "Not saved", err
	}
	return "Saved " + saved, nil
}

func signOut(ctx context.Context, _ struct{}) (string, error) {
	return "Signed out", securestore.Delete(ctx, "session")
}

func (Session) Render(c *ui.Ctx) ui.Component {
	in, out := ui.UseAction(c, signIn), ui.UseAction(c, signOut)
	return ui.SafeArea(ui.Style{Padding: ui.All(24), Gap: 12},
		ui.Button{Label: "Sign in", OnPress: func() { in.Run("token-1042") }},
		ui.Button{Label: "Sign out", Variant: ui.Tonal, OnPress: func() { out.Run(struct{}{}) }},
		ui.Text(in.Result, ui.Style{}),
		ui.Text(out.Result, ui.Style{}))
}
