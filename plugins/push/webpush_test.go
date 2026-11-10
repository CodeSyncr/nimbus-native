package push

import (
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"
)

func b64(t *testing.T, s string) []byte {
	b, err := unb64(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The example in RFC 8291, section 5.
func TestEncryptRFC8291(t *testing.T) {
	as, err := ecdh.P256().NewPrivateKey(b64(t, "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := encrypt([]byte("When I grow up, I want to be a watermelon"),
		b64(t, "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4"),
		b64(t, "BTBZMqHH6r4Tts7J_aSIgg"), b64(t, "DGv6ra1nlYgDCS1FRnbzlw"), as)
	if err != nil {
		t.Fatal(err)
	}
	want := "DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPTpK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN"
	if s := base64.RawURLEncoding.EncodeToString(got); s != want {
		t.Fatalf("got  %s\nwant %s", s, want)
	}
}

func TestVAPIDHeader(t *testing.T) {
	k, err := NewVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	k.Subject = "mailto:ops@shop.example"
	now := time.Unix(1_800_000_000, 0)
	h, err := vapidHeader("https://fcm.googleapis.com/fcm/send/abc", k, now)
	if err != nil {
		t.Fatal(err)
	}
	jwt, key, ok := strings.Cut(strings.TrimPrefix(h, "vapid t="), ", k=")
	if !ok || key != k.Public {
		t.Fatalf("header %q", h)
	}
	parts := strings.Split(jwt, ".")
	var claims struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}
	json.Unmarshal(b64(t, parts[1]), &claims)
	if claims.Aud != "https://fcm.googleapis.com" || claims.Sub != k.Subject || claims.Exp != now.Add(12*time.Hour).Unix() {
		t.Fatalf("claims %+v", claims)
	}
	pub := b64(t, k.Public)
	sig := b64(t, parts[2])
	d := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	pk := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(pub[1:33]), Y: new(big.Int).SetBytes(pub[33:])}
	if !ecdsa.Verify(pk, d[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		t.Fatal("signature doesn't verify")
	}
}

func TestSendWebNeedsKeys(t *testing.T) {
	old := VAPID
	t.Cleanup(func() { VAPID = old })
	VAPID = Keys{}
	tok := `{"endpoint":"https://push.example/x","keys":{"p256dh":"BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4","auth":"BTBZMqHH6r4Tts7J_aSIgg"}}`
	if err := SendWeb(context.Background(), tok, Notification{Title: "x"}); err == nil || !strings.Contains(err.Error(), "NIMBUS_VAPID_PUBLIC") {
		t.Fatalf("err %v", err)
	}
}
