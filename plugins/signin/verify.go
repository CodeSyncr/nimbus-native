package signin

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// keySets maps a token's issuer to where its public keys live. Only these
// issuers are believed.
var keySets = map[string]struct{ provider, url string }{
	"https://appleid.apple.com":   {"apple", "https://appleid.apple.com/auth/keys"},
	"https://accounts.google.com": {"google", "https://www.googleapis.com/oauth2/v3/certs"},
	"accounts.google.com":         {"google", "https://www.googleapis.com/oauth2/v3/certs"},
}

// ErrInvalid means the token isn't one to believe.
var ErrInvalid = errors.New("signin: invalid token")

// Verification says what a token must be.
type Verification struct {
	// Audiences are your client IDs: Apple's is the app's bundle ID (and
	// the Services ID, for the web), Google's are the ones in Options. A
	// token for another app is refused.
	Audiences []string
	// Nonce is the value you gave the phone. Empty skips the check.
	Nonce string
}

// Verify is for your server. It checks an ID token's signature with Apple's
// or Google's published keys, then who it was issued by and for, and that it
// hasn't expired. Only trust the User it returns.
//
//nimbus:param token the IDToken the phone sent
//nimbus:param v what the token must be
func Verify(ctx context.Context, token string, v Verification) (User, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return User{}, fmt.Errorf("%w: not a token", ErrInvalid)
	}
	var head struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	var body struct {
		claims
		Issuer   string          `json:"iss"`
		Audience json.RawMessage `json:"aud"`
		Expires  int64           `json:"exp"`
		Nonce    string          `json:"nonce"`
	}
	if err := decode(parts[0], &head); err != nil {
		return User{}, err
	}
	if err := decode(parts[1], &body); err != nil {
		return User{}, err
	}
	set, ok := keySets[body.Issuer]
	if !ok {
		return User{}, fmt.Errorf("%w: issuer %q", ErrInvalid, body.Issuer)
	}
	if head.Alg != "RS256" {
		return User{}, fmt.Errorf("%w: algorithm %q", ErrInvalid, head.Alg)
	}
	key, err := publicKey(ctx, set.url, head.Kid)
	if err != nil {
		return User{}, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return User{}, fmt.Errorf("%w: signature", ErrInvalid)
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig) != nil {
		return User{}, fmt.Errorf("%w: signature", ErrInvalid)
	}
	if !audience(body.Audience, v.Audiences) {
		return User{}, fmt.Errorf("%w: for another app", ErrInvalid)
	}
	if time.Now().Unix() >= body.Expires {
		return User{}, fmt.Errorf("%w: expired", ErrInvalid)
	}
	if v.Nonce != "" && body.Nonce != v.Nonce {
		return User{}, fmt.Errorf("%w: nonce", ErrInvalid)
	}
	return User{Provider: set.provider, ID: body.Subject, Email: body.Email, Name: body.Name, IDToken: token}, nil
}

func decode(part string, into any) error {
	b, err := base64.RawURLEncoding.DecodeString(part)
	if err == nil {
		err = json.Unmarshal(b, into)
	}
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return nil
}

// audience handles aud as either one string or a list.
func audience(raw json.RawMessage, allowed []string) bool {
	var list []string
	if json.Unmarshal(raw, &list) != nil {
		var one string
		if json.Unmarshal(raw, &one) != nil {
			return false
		}
		list = []string{one}
	}
	for _, a := range list {
		for _, b := range allowed {
			if a == b {
				return true
			}
		}
	}
	return false
}

var (
	keysMu sync.Mutex
	keys   = map[string]map[string]*rsa.PublicKey{}
)

// publicKey keeps the keys it has, and fetches again when it meets a new one:
// that's how a provider's rotation shows up.
func publicKey(ctx context.Context, url, kid string) (*rsa.PublicKey, error) {
	keysMu.Lock()
	defer keysMu.Unlock()
	if k := keys[url][kid]; k != nil {
		return k, nil
	}
	set, err := fetchKeys(ctx, url)
	if err != nil {
		return nil, err
	}
	keys[url] = set
	if k := set[kid]; k != nil {
		return k, nil
	}
	return nil, fmt.Errorf("%w: unknown key", ErrInvalid)
}

func fetchKeys(ctx context.Context, url string) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("signin: keys: %s", res.Status)
	}
	var doc struct {
		Keys []struct{ Kid, Kty, N, E string } `json:"keys"`
	}
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		return nil, err
	}
	out := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if k.Kty != "RSA" || err1 != nil || err2 != nil {
			continue
		}
		out[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	return out, nil
}
