package accessjwt

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVerifierChecksSignatureIssuerAudienceAndExpiry(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := map[string]any{"keys": []any{map[string]any{
		"kid": "test-key", "kty": "RSA",
		"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()
	v := New("https://team.cloudflareaccess.com", "expected-aud")
	v.certsURL = server.URL
	now := time.Now().Unix()
	good := map[string]any{"iss": v.issuer, "aud": []string{"expected-aud"}, "exp": now + 300, "iat": now - 10}
	if err := v.Verify(context.Background(), signedToken(t, key, good)); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	for name, change := range map[string]func(map[string]any){
		"issuer":   func(c map[string]any) { c["iss"] = "https://other.cloudflareaccess.com" },
		"audience": func(c map[string]any) { c["aud"] = "wrong-aud" },
		"expired":  func(c map[string]any) { c["exp"] = now - 1 },
	} {
		t.Run(name, func(t *testing.T) {
			claims := map[string]any{"iss": v.issuer, "aud": "expected-aud", "exp": now + 300, "iat": now - 10}
			change(claims)
			if err := v.Verify(context.Background(), signedToken(t, key, claims)); err == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Verify(context.Background(), signedToken(t, otherKey, good)); err == nil {
		t.Fatal("invalid signature accepted")
	}
}

func signedToken(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "test-key"})
	payload, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}
