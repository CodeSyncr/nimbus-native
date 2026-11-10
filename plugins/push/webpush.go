package push

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Keys is a VAPID key pair: it tells browsers' push services that a
// message comes from your server. Make one with NewVAPIDKeys, once, and keep
// it; a new pair orphans every browser that subscribed with the old one.
type Keys struct {
	Public  string // base64url; the browser subscribes with it
	Private string // base64url; keep it on your server
	Subject string // "mailto:you@example.com" or your site's https address, for push services to reach you
}

// VAPID is the key pair GetToken and SendWeb use, from NIMBUS_VAPID_PUBLIC,
// NIMBUS_VAPID_PRIVATE and NIMBUS_VAPID_SUBJECT unless you set it.
var VAPID = Keys{
	Public:  os.Getenv("NIMBUS_VAPID_PUBLIC"),
	Private: os.Getenv("NIMBUS_VAPID_PRIVATE"),
	Subject: os.Getenv("NIMBUS_VAPID_SUBJECT"),
}

// NewVAPIDKeys makes a key pair for the web. Run it once and store the pair
// as NIMBUS_VAPID_PUBLIC and NIMBUS_VAPID_PRIVATE.
func NewVAPIDKeys() (Keys, error) {
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return Keys{}, err
	}
	enc := base64.RawURLEncoding.EncodeToString
	return Keys{Public: enc(k.PublicKey().Bytes()), Private: enc(k.Bytes())}, nil
}

// Notification is what SendWeb shows.
type Notification struct {
	Title string            `json:"title"`          // its heading
	Body  string            `json:"body"`           // its text
	Data  map[string]string `json:"data,omitempty"` // Message.Data on arrival
	URL   string            `json:"url,omitempty"`  // the page a tap opens, such as "/orders/42"; "/" by default
	Tag   string            `json:"tag,omitempty"`  // a new one with the same tag replaces the old
	TTL   time.Duration     `json:"-"`              // how long the push service keeps trying (default a day)
}

// ErrGone is the browser having unsubscribed: delete the token.
var ErrGone = errors.New("push: subscription expired or unsubscribed")

// SendWeb sends a notification to a browser, by the Value of a "web" Token.
// It returns ErrGone when the subscription has ended.
//
//nimbus:param token the Value of the browser's Token
//nimbus:param n what to show
func SendWeb(ctx context.Context, token string, n Notification) error {
	var sub struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	if err := json.Unmarshal([]byte(token), &sub); err != nil || sub.Endpoint == "" {
		return errors.New("push: not a web token")
	}
	ua, err1 := unb64(sub.Keys.P256dh)
	secret, err2 := unb64(sub.Keys.Auth)
	if err1 != nil || err2 != nil || len(secret) != 16 {
		return errors.New("push: the web token's keys are malformed")
	}
	payload, _ := json.Marshal(n)
	body, err := encrypt(payload, ua, secret, nil, nil)
	if err != nil {
		return err
	}
	auth, err := vapidHeader(sub.Endpoint, VAPID, time.Now())
	if err != nil {
		return err
	}
	ttl := n.TTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", fmt.Sprint(int(ttl.Seconds())))
	req.Header.Set("Urgency", "high")
	req.Header.Set("Authorization", auth)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone:
		return ErrGone
	case res.StatusCode >= 300:
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("push: %s: %s", res.Status, strings.TrimSpace(string(msg)))
	}
	return nil
}

func unb64(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
}

// encrypt seals a message for one browser (RFC 8291, aes128gcm). salt and
// key are random unless a test sets them.
func encrypt(plain, uaPublic, authSecret, salt []byte, key *ecdh.PrivateKey) ([]byte, error) {
	ua, err := ecdh.P256().NewPublicKey(uaPublic)
	if err != nil {
		return nil, errors.New("push: the web token's key is malformed")
	}
	if key == nil {
		if key, err = ecdh.P256().GenerateKey(rand.Reader); err != nil {
			return nil, err
		}
	}
	if salt == nil {
		salt = make([]byte, 16)
		rand.Read(salt)
	}
	shared, err := key.ECDH(ua)
	if err != nil {
		return nil, err
	}
	asPublic := key.PublicKey().Bytes()
	info := append(append([]byte("WebPush: info\x00"), uaPublic...), asPublic...)
	ikm := hkdf(authSecret, shared, info, 32)
	cek := hkdf(salt, ikm, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := hkdf(salt, ikm, []byte("Content-Encoding: nonce\x00"), 12)
	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	sealed := gcm.Seal(nil, nonce, append(plain, 2), nil) // 2: the last record

	head := make([]byte, 0, 86+len(sealed))
	head = append(head, salt...)
	head = binary.BigEndian.AppendUint32(head, 4096)
	head = append(head, byte(len(asPublic)))
	head = append(head, asPublic...)
	return append(head, sealed...), nil
}

// hkdf is HKDF-SHA256 for outputs of one block.
func hkdf(salt, ikm, info []byte, n int) []byte {
	m := hmac.New(sha256.New, salt)
	m.Write(ikm)
	m = hmac.New(sha256.New, m.Sum(nil))
	m.Write(info)
	m.Write([]byte{1})
	return m.Sum(nil)[:n]
}

// vapidHeader signs a short-lived claim that the push service checks
// against the key the browser subscribed with (RFC 8292).
func vapidHeader(endpoint string, k Keys, now time.Time) (string, error) {
	if k.Public == "" || k.Private == "" {
		return "", errors.New("push: set NIMBUS_VAPID_PUBLIC and NIMBUS_VAPID_PRIVATE (push.NewVAPIDKeys makes a pair)")
	}
	if k.Subject == "" {
		return "", errors.New(`push: set NIMBUS_VAPID_SUBJECT, such as "mailto:you@example.com"`)
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	d, err := unb64(k.Private)
	if err != nil {
		return "", errors.New("push: NIMBUS_VAPID_PRIVATE is malformed")
	}
	pub, err := unb64(k.Public)
	if err != nil || len(pub) != 65 {
		return "", errors.New("push: NIMBUS_VAPID_PUBLIC is malformed")
	}
	priv := &ecdsa.PrivateKey{D: new(big.Int).SetBytes(d)}
	priv.PublicKey = ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(pub[1:33]), Y: new(big.Int).SetBytes(pub[33:])}

	enc := base64.RawURLEncoding.EncodeToString
	claims, _ := json.Marshal(map[string]any{"aud": u.Scheme + "://" + u.Host, "exp": now.Add(12 * time.Hour).Unix(), "sub": k.Subject})
	unsigned := enc([]byte(`{"typ":"JWT","alg":"ES256"}`)) + "." + enc(claims)
	h := sha256.Sum256([]byte(unsigned))
	r, s, err := ecdsa.Sign(rand.Reader, priv, h[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return "vapid t=" + unsigned + "." + enc(sig) + ", k=" + k.Public, nil
}
