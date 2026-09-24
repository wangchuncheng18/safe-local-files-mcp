package accessjwt

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Verifier checks Cloudflare Access application JWTs before an MCP request
// reaches the filesystem. Signing keys are fetched only from the configured
// Cloudflare team domain and cached to keep normal requests off the network.
type Verifier struct {
	issuer   string
	audience string
	certsURL string
	client   *http.Client
	mu       sync.Mutex
	keys     map[string]*rsa.PublicKey
	expires  time.Time
}

func New(teamDomain, audience string) *Verifier {
	return &Verifier{
		issuer: teamDomain, audience: audience,
		certsURL: teamDomain + "/cdn-cgi/access/certs",
		client: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return errors.New("Access signing keys redirect refused")
			},
		},
	}
}

func (v *Verifier) Verify(ctx context.Context, token string) error {
	if len(token) == 0 || len(token) > 16384 {
		return errors.New("missing or oversized Access JWT")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("malformed Access JWT")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return errors.New("malformed Access JWT header")
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if json.Unmarshal(headerBytes, &header) != nil || header.Alg != "RS256" || header.Kid == "" {
		return errors.New("unsupported Access JWT header")
	}
	key, err := v.key(ctx, header.Kid)
	if err != nil {
		return err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return errors.New("malformed Access JWT signature")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature) != nil {
		return errors.New("invalid Access JWT signature")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return errors.New("malformed Access JWT payload")
	}
	var claims struct {
		Issuer    string          `json:"iss"`
		Audience  json.RawMessage `json:"aud"`
		Expiry    int64           `json:"exp"`
		NotBefore int64           `json:"nbf"`
		IssuedAt  int64           `json:"iat"`
	}
	if json.Unmarshal(payloadBytes, &claims) != nil {
		return errors.New("malformed Access JWT claims")
	}
	now := time.Now().Unix()
	if claims.Issuer != v.issuer || !hasAudience(claims.Audience, v.audience) || claims.Expiry <= now || (claims.NotBefore != 0 && claims.NotBefore > now+60) || (claims.IssuedAt != 0 && claims.IssuedAt > now+60) {
		return errors.New("invalid Access JWT claims")
	}
	return nil
}

func hasAudience(raw json.RawMessage, expected string) bool {
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return single == expected
	}
	var list []string
	if json.Unmarshal(raw, &list) != nil {
		return false
	}
	for _, value := range list {
		if value == expected {
			return true
		}
	}
	return false
}

func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if time.Now().Before(v.expires) {
		if key := v.keys[kid]; key != nil {
			return key, nil
		}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, v.certsURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := v.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch Access signing keys: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("Access signing keys unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20+1))
	if err != nil || len(data) > 1<<20 {
		return nil, errors.New("Access signing keys response invalid")
	}
	var set struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if json.Unmarshal(data, &set) != nil || len(set.Keys) == 0 || len(set.Keys) > 20 {
		return nil, errors.New("Access signing keys response invalid")
	}
	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, item := range set.Keys {
		if item.Kty != "RSA" || item.Kid == "" {
			continue
		}
		nBytes, nErr := base64.RawURLEncoding.DecodeString(item.N)
		eBytes, eErr := base64.RawURLEncoding.DecodeString(item.E)
		if nErr != nil || eErr != nil || len(nBytes) < 256 || len(nBytes) > 1024 || len(eBytes) == 0 || len(eBytes) > 4 {
			continue
		}
		exponent := new(big.Int).SetBytes(eBytes)
		if !exponent.IsInt64() || exponent.Int64() < 3 || exponent.Int64() > 1<<31-1 || exponent.Int64()%2 == 0 {
			continue
		}
		keys[item.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: int(exponent.Int64())}
	}
	if len(keys) == 0 {
		return nil, errors.New("Access signing keys response invalid")
	}
	v.keys = keys
	v.expires = time.Now().Add(15 * time.Minute)
	if key := keys[kid]; key != nil {
		return key, nil
	}
	return nil, errors.New("Access signing key not found")
}
