// Package signin signs them in with Apple or Google, using the phone's own
// sheet. On the phone:
//
//	nimbus native plugin add sign-in
//
//	u, err := signin.Google(ctx, signin.Options{
//		IOSClient:     "123-ios.apps.googleusercontent.com",
//		AndroidClient: "123-web.apps.googleusercontent.com",
//	})
//
// Then send u.IDToken to your server, which checks it with Verify. The other
// fields are for showing their name: anyone can forge what a phone sends.
//
// On the web, the provider's page opens in a popup and comes back to the
// site's /_nn/oauth, such as https://cafe.example/_nn/oauth: register that
// as a redirect URI (Google) or return URL (Apple).
package signin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Options says who's asking.
type Options struct {
	// Nonce is a one-time value from your server. It comes back inside the
	// ID token, so a stolen token can't be replayed.
	Nonce string `json:"nonce,omitempty"`
	// IOSClient is Google's iOS OAuth client ID. Google only.
	IOSClient string `json:"iosClient,omitempty"`
	// AndroidClient is Google's Web OAuth client ID: that kind, not the
	// Android one, which only has to exist. The web uses it too. Google only.
	AndroidClient string `json:"androidClient,omitempty"`
	// AppleServicesID is Apple's Services ID, for Sign in with Apple on
	// the web. Apple only.
	AppleServicesID string `json:"appleServicesID,omitempty"`
}

// User is who signed in.
type User struct {
	Provider string `json:"provider"` // "apple" or "google"
	ID       string `json:"id"`       // theirs for life at that provider
	Email    string `json:"email"`    // their address; Apple may give a private relay
	Name     string `json:"name"`     // their name, when known
	IDToken  string `json:"idToken"`  // for your server to check with Verify
	// Code is Apple's one-time authorization code, for a server that wants
	// refresh tokens. Empty for Google.
	Code string `json:"code"`
}

// Apple shows Sign in with Apple, on iOS and on the web (with
// AppleServicesID). On Android the error has code plugin.Unavailable. Apple gives their name the first time only, and
// their email may be a private relay address.
//
//nimbus:param o the nonce; the client IDs are for Google
func Apple(ctx context.Context, o Options) (User, error) { return call(ctx, "apple", o) }

// Google shows Google's sign-in. On iOS it's Google's page in a secure
// browser sheet; on Android it's the phone's account sheet.
//
//nimbus:param o the client IDs and the nonce
func Google(ctx context.Context, o Options) (User, error) { return call(ctx, "google", o) }

func call(ctx context.Context, provider string, o Options) (User, error) {
	u, err := plugin.Call[User](ctx, "signin", provider, o)
	if err != nil {
		return u, err
	}
	u.Provider = provider
	fromToken(&u)
	return u, nil
}

// claims is the part of an ID token that matters here.
type claims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
}

// fromToken fills what the token knows and the host didn't say. Nothing is
// checked: this is only for display.
func fromToken(u *User) {
	parts := strings.Split(u.IDToken, ".")
	if len(parts) != 3 {
		return
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return
	}
	var c claims
	if json.Unmarshal(b, &c) != nil {
		return
	}
	u.ID, u.Email = c.Subject, c.Email
	if u.Name == "" {
		u.Name = c.Name
	}
}
