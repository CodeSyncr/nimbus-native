// Package oauth signs them in with any OAuth 2 provider: GitHub, Microsoft,
// Slack, your own. The provider's page opens in the system's secure browser
// sheet, and PKCE stands in for a client secret, so none ships in the app.
// For Apple and Google, signin uses the phone's own sheets.
//
//	nimbus native plugin add oauth -scheme com.example.cafe
//
//	t, err := oauth.SignIn(ctx, oauth.Config{
//		Endpoint:    oauth.Microsoft("common"),
//		ClientID:    "your-app-id",
//		RedirectURL: "com.example.cafe://auth",
//		Scopes:      []string{"openid", "profile", "offline_access", "User.Read"},
//	})
//
// Some providers, GitHub among them, still want a secret for the exchange.
// For those, Authorize gets the code on the phone and your server calls
// Exchange with the secret.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/CodeSyncr/nimbus-native/plugin"
)

// Endpoint is where a provider signs people in.
type Endpoint struct {
	AuthURL  string // the page that asks them
	TokenURL string // where a code becomes a token
}

// GitHub's endpoints. Its OAuth apps want the client secret for the
// exchange, so use Authorize on the phone and Exchange on your server.
var GitHub = Endpoint{
	AuthURL:  "https://github.com/login/oauth/authorize",
	TokenURL: "https://github.com/login/oauth/access_token",
}

// Microsoft is Microsoft's identity platform. tenant is "common" for any
// account, "organizations" or "consumers", or your directory's ID.
//
//nimbus:param tenant whose accounts may sign in
func Microsoft(tenant string) Endpoint {
	base := "https://login.microsoftonline.com/" + url.PathEscape(tenant) + "/oauth2/v2.0/"
	return Endpoint{AuthURL: base + "authorize", TokenURL: base + "token"}
}

// Config says which provider, and who's asking.
type Config struct {
	// Endpoint is the provider's: GitHub, Microsoft(tenant), or one you fill.
	Endpoint Endpoint
	// ClientID is your app's ID at the provider.
	ClientID string
	// RedirectURL is where the provider sends them back, such as
	// "com.example.cafe://auth". Its scheme is the one you gave plugin add,
	// and it must be registered at the provider exactly as written.
	RedirectURL string
	// Scopes are what you ask for.
	Scopes []string
	// Params go on the sign-in page's address as they are, such as
	// {"prompt": "select_account"}.
	Params map[string]string
	// Ephemeral keeps the browser's cookies out of it, so they sign in
	// afresh and iOS doesn't first ask to use the provider's site. iOS only.
	Ephemeral bool
	// ClientSecret is for your server's Exchange and Refresh only. Never
	// put it in the app: anyone can read what's in there.
	ClientSecret string
}

// Grant is what the provider sent back: a one-time code, and the verifier
// that proves this app asked for it. Exchange turns it into a Token.
type Grant struct {
	Code        string `json:"code"`        // the provider's one-time code
	Verifier    string `json:"verifier"`    // the PKCE secret only this sign-in knew
	RedirectURL string `json:"redirectURL"` // which the exchange must repeat
}

// Token is what the provider hands out.
type Token struct {
	AccessToken  string    `json:"access_token"`            // for the provider's API
	TokenType    string    `json:"token_type"`              // usually "Bearer"
	RefreshToken string    `json:"refresh_token,omitempty"` // for Refresh; only if the provider gives one
	IDToken      string    `json:"id_token,omitempty"`      // with the "openid" scope, from providers that do OpenID
	Scope        string    `json:"scope,omitempty"`         // what was granted, which can be less than asked
	Expiry       time.Time `json:"expiry"`                  // zero when the provider didn't say
}

// Error is the provider saying no. Code is OAuth's: "access_denied" when they
// turned it down, "invalid_grant" for a used or expired code, and others.
type Error struct {
	Code        string // OAuth's error code
	Description string // the provider's words, when it gave any
}

func (e *Error) Error() string {
	if e.Description == "" {
		return "oauth: " + e.Code
	}
	return "oauth: " + e.Code + ": " + e.Description
}

// SignIn shows the provider's page and returns their token. Backing out
// gives a plugin error with code plugin.Cancelled.
//
//nimbus:param c the provider and your app's registration there
func SignIn(ctx context.Context, c Config) (Token, error) {
	g, err := Authorize(ctx, c)
	if err != nil {
		return Token{}, err
	}
	return Exchange(ctx, c, g)
}

// Authorize shows the provider's page and returns the code it sent back,
// without exchanging it. Send the Grant to your server when the exchange
// needs a secret.
//
//nimbus:param c the provider and your app's registration there
func Authorize(ctx context.Context, c Config) (Grant, error) {
	if c.Endpoint.AuthURL == "" || c.ClientID == "" || c.RedirectURL == "" {
		return Grant{}, fmt.Errorf("oauth: Config needs Endpoint, ClientID and RedirectURL")
	}
	redirect, err := url.Parse(c.RedirectURL)
	if err != nil || redirect.Scheme == "" {
		return Grant{}, fmt.Errorf("oauth: RedirectURL %q has no scheme", c.RedirectURL)
	}
	verifier, state := random(), random()
	page, err := authURL(c, state, challenge(verifier))
	if err != nil {
		return Grant{}, err
	}
	back, err := open(ctx, page, redirect.Scheme, c.Ephemeral)
	if err != nil {
		return Grant{}, err
	}
	code, err := callback(back, state)
	if err != nil {
		return Grant{}, err
	}
	return Grant{Code: code, Verifier: verifier, RedirectURL: c.RedirectURL}, nil
}

// Exchange turns a Grant into a Token. On the phone it runs inside SignIn;
// on your server, set ClientSecret.
//
//nimbus:param c the provider and your app's registration there
//nimbus:param g what Authorize returned
func Exchange(ctx context.Context, c Config, g Grant) (Token, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {g.Code},
		"redirect_uri":  {g.RedirectURL},
		"code_verifier": {g.Verifier},
	}
	return token(ctx, c, form)
}

// Refresh gets a new access token with a refresh token, without showing
// them anything. The answer may carry a new refresh token: keep that one.
//
//nimbus:param c the provider and your app's registration there
//nimbus:param refreshToken Token.RefreshToken from before
func Refresh(ctx context.Context, c Config, refreshToken string) (Token, error) {
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}}
	if len(c.Scopes) > 0 {
		form.Set("scope", strings.Join(c.Scopes, " "))
	}
	t, err := token(ctx, c, form)
	if err == nil && t.RefreshToken == "" {
		t.RefreshToken = refreshToken
	}
	return t, err
}

// open shows the page in the secure browser sheet and returns the address
// the provider sent them back to. Tests replace it.
var open = func(ctx context.Context, page, scheme string, ephemeral bool) (string, error) {
	r, err := plugin.Call[struct {
		URL string `json:"url"`
	}](ctx, "oauth", "open", map[string]any{"url": page, "scheme": scheme, "ephemeral": ephemeral})
	return r.URL, err
}

func random() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand doesn't fail on the systems Go supports
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func authURL(c Config, state, challenge string) (string, error) {
	u, err := url.Parse(c.Endpoint.AuthURL)
	if err != nil {
		return "", fmt.Errorf("oauth: AuthURL: %w", err)
	}
	q := u.Query()
	for k, v := range c.Params {
		q.Set(k, v)
	}
	q.Set("response_type", "code")
	q.Set("client_id", c.ClientID)
	q.Set("redirect_uri", c.RedirectURL)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	if len(c.Scopes) > 0 {
		q.Set("scope", strings.Join(c.Scopes, " "))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// callback reads the address they came back to. A state that isn't ours
// means the answer is for some other request, so it's refused.
func callback(back, state string) (string, error) {
	u, err := url.Parse(back)
	if err != nil {
		return "", fmt.Errorf("oauth: came back to %q: %w", back, err)
	}
	q := u.Query()
	if q.Get("code") == "" && q.Get("error") == "" && u.Fragment != "" {
		q, _ = url.ParseQuery(u.Fragment) // a provider set to answer in the fragment
	}
	if q.Get("state") != state {
		return "", &Error{Code: "invalid_state", Description: "the answer wasn't for this sign-in"}
	}
	if e := q.Get("error"); e != "" {
		return "", &Error{Code: e, Description: q.Get("error_description")}
	}
	code := q.Get("code")
	if code == "" {
		return "", &Error{Code: "invalid_response", Description: "the provider sent no code"}
	}
	return code, nil
}

func token(ctx context.Context, c Config, form url.Values) (Token, error) {
	if c.Endpoint.TokenURL == "" {
		return Token{}, fmt.Errorf("oauth: Config needs Endpoint.TokenURL")
	}
	form.Set("client_id", c.ClientID)
	if c.ClientSecret != "" {
		form.Set("client_secret", c.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json") // GitHub answers in a form otherwise
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Token{}, err
	}
	defer res.Body.Close()
	var body struct {
		Token
		ExpiresIn   json.RawMessage `json:"expires_in"`
		Err         string          `json:"error"`
		Description string          `json:"error_description"`
	}
	decodeErr := json.NewDecoder(res.Body).Decode(&body)
	if body.Err != "" { // GitHub says no with a 200
		return Token{}, &Error{Code: body.Err, Description: body.Description}
	}
	if res.StatusCode != http.StatusOK {
		return Token{}, fmt.Errorf("oauth: token: %s", res.Status)
	}
	if decodeErr != nil {
		return Token{}, fmt.Errorf("oauth: token: %w", decodeErr)
	}
	if body.AccessToken == "" {
		return Token{}, &Error{Code: "invalid_response", Description: "the provider sent no access token"}
	}
	t := body.Token
	if s := strings.Trim(string(body.ExpiresIn), `"`); s != "" && s != "null" {
		if n, err := strconv.ParseFloat(s, 64); err == nil && n > 0 {
			t.Expiry = time.Now().Add(time.Duration(n) * time.Second)
		}
	}
	return t, nil
}
