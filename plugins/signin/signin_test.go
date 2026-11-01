package signin

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func sign(t *testing.T, k *rsa.PrivateKey, head, body map[string]any) string {
	t.Helper()
	h, _ := json.Marshal(head)
	b, _ := json.Marshal(body)
	msg := b64(h) + "." + b64(b)
	sum := sha256.Sum256([]byte(msg))
	sig, err := rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return msg + "." + b64(sig)
}

// provider stands in for Google: a key, published on a test server.
func provider(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "k1", "kty": "RSA", "n": b64(k.N.Bytes()), "e": b64(big.NewInt(int64(k.E)).Bytes()),
		}}})
	}))
	t.Cleanup(s.Close)
	old := keySets["https://accounts.google.com"]
	keySets["https://accounts.google.com"] = struct{ provider, url string }{"google", s.URL}
	t.Cleanup(func() { keySets["https://accounts.google.com"] = old; keys = map[string]map[string]*rsa.PublicKey{} })
	return k
}

func TestVerify(t *testing.T) {
	k := provider(t)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	head := map[string]any{"alg": "RS256", "kid": "k1"}
	good := func() map[string]any {
		return map[string]any{"iss": "https://accounts.google.com", "aud": "app", "sub": "42", "email": "a@b.co",
			"name": "Asha", "nonce": "n", "exp": time.Now().Add(time.Hour).Unix()}
	}
	v := Verification{Audiences: []string{"app"}, Nonce: "n"}
	ctx := context.Background()

	u, err := Verify(ctx, sign(t, k, head, good()), v)
	if err != nil {
		t.Fatal(err)
	}
	if u.Provider != "google" || u.ID != "42" || u.Email != "a@b.co" || u.Name != "Asha" {
		t.Fatalf("user = %+v", u)
	}

	list := good()
	list["aud"] = []string{"x", "app"}
	if _, err := Verify(ctx, sign(t, k, head, list), v); err != nil {
		t.Fatalf("aud list: %v", err)
	}

	bad := map[string]func() string{
		"forged": func() string { return sign(t, other, head, good()) },
		"other app": func() string {
			b := good()
			b["aud"] = "evil"
			return sign(t, k, head, b)
		},
		"expired": func() string {
			b := good()
			b["exp"] = time.Now().Add(-time.Minute).Unix()
			return sign(t, k, head, b)
		},
		"replayed": func() string {
			b := good()
			b["nonce"] = "old"
			return sign(t, k, head, b)
		},
		"strange issuer": func() string {
			b := good()
			b["iss"] = "https://evil.example"
			return sign(t, k, head, b)
		},
		"unsigned": func() string { return sign(t, k, map[string]any{"alg": "none", "kid": "k1"}, good()) },
		"junk":     func() string { return "a.b" },
	}
	for name, token := range bad {
		if _, err := Verify(ctx, token(), v); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestFromToken(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"sub": "9", "email": "x@y.z", "name": "From Token"})
	u := User{IDToken: "h." + b64(body) + ".s", Name: "From Host"}
	fromToken(&u)
	if u.ID != "9" || u.Email != "x@y.z" || u.Name != "From Host" {
		t.Fatalf("user = %+v", u)
	}
}
