package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// provider is a stand-in OAuth server that checks PKCE the way a real one
// does. browser plays the person in the sheet: it reads the sign-in page's
// address and says where the provider sends them back.
type provider struct {
	t         *testing.T
	challenge string
	secret    string // wanted at the token endpoint when set
	answer    func(q url.Values) url.Values
	form      url.Values // the last token request
	status    int
	body      any
}

func (p *provider) serve(t *testing.T) Endpoint {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept = %q", r.Header.Get("Accept"))
		}
		r.ParseForm()
		p.form = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		if p.status != 0 {
			w.WriteHeader(p.status)
		}
		if p.body != nil {
			json.NewEncoder(w).Encode(p.body)
			return
		}
		f := r.PostForm
		switch {
		case f.Get("grant_type") == "refresh_token" && f.Get("refresh_token") == "r1":
			json.NewEncoder(w).Encode(map[string]any{"access_token": "a2", "token_type": "Bearer", "expires_in": "60"})
		case f.Get("code") != "c1" || challenge(f.Get("code_verifier")) != p.challenge:
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
		case p.secret != "" && f.Get("client_secret") != p.secret:
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_client"})
		default:
			json.NewEncoder(w).Encode(map[string]any{"access_token": "a1", "token_type": "Bearer", "refresh_token": "r1",
				"id_token": "i1", "scope": "read", "expires_in": 3600})
		}
	}))
	t.Cleanup(s.Close)
	old := open
	t.Cleanup(func() { open = old })
	open = func(ctx context.Context, page, scheme string, ephemeral bool) (string, error) {
		u, err := url.Parse(page)
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		if scheme != "com.example.cafe" {
			t.Errorf("scheme = %q", scheme)
		}
		if q.Get("code_challenge_method") != "S256" || q.Get("response_type") != "code" || q.Get("client_id") != "app" {
			t.Errorf("page = %s", page)
		}
		p.challenge = q.Get("code_challenge")
		back := url.Values{"code": {"c1"}, "state": {q.Get("state")}}
		if p.answer != nil {
			back = p.answer(q)
		}
		return q.Get("redirect_uri") + "?" + back.Encode(), nil
	}
	return Endpoint{AuthURL: s.URL + "/authorize?tenant=x", TokenURL: s.URL + "/token"}
}

func config(e Endpoint) Config {
	return Config{Endpoint: e, ClientID: "app", RedirectURL: "com.example.cafe://auth", Scopes: []string{"openid", "read"},
		Params: map[string]string{"prompt": "select_account"}}
}

func TestSignIn(t *testing.T) {
	p := &provider{}
	c := config(p.serve(t))
	var page url.Values
	p.answer = func(q url.Values) url.Values {
		page = q
		return url.Values{"code": {"c1"}, "state": {q.Get("state")}}
	}
	tok, err := SignIn(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "a1" || tok.RefreshToken != "r1" || tok.IDToken != "i1" || tok.Scope != "read" {
		t.Fatalf("token = %+v", tok)
	}
	if d := time.Until(tok.Expiry); d < 59*time.Minute || d > time.Hour {
		t.Fatalf("expiry in %v", d)
	}
	if page.Get("scope") != "openid read" || page.Get("prompt") != "select_account" || page.Get("tenant") != "x" {
		t.Fatalf("page = %v", page)
	}
	if p.form.Get("client_secret") != "" || p.form.Get("redirect_uri") != c.RedirectURL {
		t.Fatalf("token request = %v", p.form)
	}
}

func TestRefusals(t *testing.T) {
	p := &provider{}
	c := config(p.serve(t))
	ctx := context.Background()
	cases := map[string]struct {
		answer func(q url.Values) url.Values
		code   string
	}{
		"turned down": {func(q url.Values) url.Values {
			return url.Values{"error": {"access_denied"}, "state": {q.Get("state")}}
		}, "access_denied"},
		"forged state": {func(q url.Values) url.Values { return url.Values{"code": {"c1"}, "state": {"other"}} }, "invalid_state"},
		"no state":     {func(q url.Values) url.Values { return url.Values{"code": {"c1"}} }, "invalid_state"},
		"no code":      {func(q url.Values) url.Values { return url.Values{"state": {q.Get("state")}} }, "invalid_response"},
		"wrong code":   {func(q url.Values) url.Values { return url.Values{"code": {"c2"}, "state": {q.Get("state")}} }, "invalid_grant"},
	}
	for name, tc := range cases {
		p.answer = tc.answer
		_, err := SignIn(ctx, c)
		var e *Error
		if !errors.As(err, &e) || e.Code != tc.code {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

// A code taken from the redirect is no use without the verifier.
func TestStolenCode(t *testing.T) {
	p := &provider{}
	c := config(p.serve(t))
	g, err := Authorize(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	g.Verifier = random()
	if _, err := Exchange(context.Background(), c, g); err == nil {
		t.Fatal("exchanged with the wrong verifier")
	}
}

// GitHub's way: the phone authorizes, the server exchanges with its secret.
func TestServerExchange(t *testing.T) {
	p := &provider{secret: "s3cret"}
	c := config(p.serve(t))
	g, err := Authorize(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(g) // to the server
	var got Grant
	json.Unmarshal(b, &got)
	if _, err := Exchange(context.Background(), c, got); err == nil {
		t.Fatal("exchanged without the secret")
	}
	c.ClientSecret = "s3cret"
	tok, err := Exchange(context.Background(), c, got)
	if err != nil || tok.AccessToken != "a1" {
		t.Fatalf("token = %+v, %v", tok, err)
	}
}

func TestRefresh(t *testing.T) {
	p := &provider{}
	c := config(p.serve(t))
	tok, err := Refresh(context.Background(), c, "r1")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "a2" || tok.RefreshToken != "r1" || tok.Expiry.IsZero() {
		t.Fatalf("token = %+v", tok)
	}
	if p.form.Get("scope") != "openid read" {
		t.Fatalf("form = %v", p.form)
	}
}

func TestBadAnswers(t *testing.T) {
	p := &provider{}
	c := config(p.serve(t))
	ctx := context.Background()
	p.status, p.body = http.StatusBadGateway, "down"
	if _, err := Refresh(ctx, c, "r1"); err == nil {
		t.Error("a 502 passed")
	}
	p.status, p.body = 0, map[string]string{"token_type": "Bearer"}
	var e *Error
	if _, err := Refresh(ctx, c, "r1"); !errors.As(err, &e) || e.Code != "invalid_response" {
		t.Errorf("no access token: err = %v", err)
	}
}

func TestConfigChecks(t *testing.T) {
	ctx := context.Background()
	if _, err := Authorize(ctx, Config{}); err == nil {
		t.Error("empty config passed")
	}
	if _, err := Authorize(ctx, Config{Endpoint: GitHub, ClientID: "x", RedirectURL: "no-scheme"}); err == nil {
		t.Error("redirect without a scheme passed")
	}
}

func TestFragment(t *testing.T) {
	code, err := callback("com.example.cafe://auth#code=c9&state=s", "s")
	if err != nil || code != "c9" {
		t.Fatalf("code = %q, %v", code, err)
	}
}

func TestMicrosoft(t *testing.T) {
	e := Microsoft("common")
	if e.AuthURL != "https://login.microsoftonline.com/common/oauth2/v2.0/authorize" ||
		e.TokenURL != "https://login.microsoftonline.com/common/oauth2/v2.0/token" {
		t.Fatalf("%+v", e)
	}
}
