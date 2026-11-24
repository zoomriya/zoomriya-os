// Who is asking: a signed-in member, or an admin, by their ZOOMRIYA ID
// token (EdDSA, checked with the keys ZOOMRIYA ID publishes). The apps call
// /complete as the member, with the member's own token; Box Desktop calls it
// as the signed-in admin. The service keeps no accounts: the token's
// signature and claims are enough.
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// The token rules of zoomriya_identity.identity (ZOOMRIYA ID).
const (
	tokenIssuer   = "zoomriya-id"
	tokenAudience = "zoomriya"
	tokenCookie   = "zoomriya.auth.token"
)

var errBadToken = errors.New("not a valid ZOOMRIYA ID token")

// Claims is a checked token's claims.
type Claims map[string]any

// Admin: the token says so (ZOOMRIYA ID's role claim).
func (c Claims) Admin() bool {
	role, _ := c["role"].(string)
	return strings.EqualFold(role, "admin")
}

// Auth checks tokens with ZOOMRIYA ID's public keys, fetched once and kept.
// An unknown key fetches them again (ZOOMRIYA ID rotated its key), at most
// once every 30 seconds, so a stream of bad tokens can't hammer it.
type Auth struct {
	URL    string
	Client *http.Client
	// Now: the clock, for tests.
	Now func() time.Time

	mu      sync.Mutex
	keys    map[string]ed25519.PublicKey
	fetched time.Time
}

func NewAuth(jwksURL string) *Auth {
	return &Auth{URL: jwksURL, Client: &http.Client{Timeout: 5 * time.Second}, Now: time.Now}
}

func (a *Auth) key(kid string) (ed25519.PublicKey, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if key, ok := a.keys[kid]; ok {
		return key, nil
	}
	if !a.fetched.IsZero() && a.Now().Sub(a.fetched) < 30*time.Second {
		return nil, errBadToken
	}
	keys, err := a.fetch()
	if err != nil {
		return nil, err
	}
	a.keys, a.fetched = keys, a.Now()
	if key, ok := keys[kid]; ok {
		return key, nil
	}
	return nil, errBadToken
}

func (a *Auth) fetch() (map[string]ed25519.PublicKey, error) {
	response, err := a.Client.Get(a.URL)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ZOOMRIYA ID's keys: %s", response.Status)
	}
	var document struct {
		Keys []struct {
			Kty, Crv, Kid, X string
		} `json:"keys"`
	}
	if err := json.NewDecoder(response.Body).Decode(&document); err != nil {
		return nil, err
	}
	keys := map[string]ed25519.PublicKey{}
	for _, k := range document.Keys {
		if k.Kty != "OKP" || k.Crv != "Ed25519" {
			continue
		}
		raw, err := base64.RawURLEncoding.DecodeString(k.X)
		if err == nil && len(raw) == ed25519.PublicKeySize {
			keys[k.Kid] = ed25519.PublicKey(raw)
		}
	}
	return keys, nil
}

// Verify checks a token and returns its claims: EdDSA, a known key, ZOOMRIYA
// ID's issuer, the apps' audience, not expired, with exp, iat, iss, aud and
// sub all present.
func (a *Auth) Verify(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errBadToken
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := decodePart(parts[0], &header); err != nil || header.Alg != "EdDSA" {
		return nil, errBadToken
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errBadToken
	}
	key, err := a.key(header.Kid)
	if err != nil {
		return nil, err
	}
	if !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), signature) {
		return nil, errBadToken
	}
	var claims Claims
	if err := decodePart(parts[1], &claims); err != nil {
		return nil, errBadToken
	}
	for _, name := range []string{"exp", "iat", "iss", "aud", "sub"} {
		if _, ok := claims[name]; !ok {
			return nil, errBadToken
		}
	}
	exp, ok := claims["exp"].(float64)
	if !ok || float64(a.Now().Unix()) >= exp {
		return nil, errBadToken
	}
	if iss, _ := claims["iss"].(string); iss != tokenIssuer || !hasAudience(claims["aud"]) {
		return nil, errBadToken
	}
	return claims, nil
}

func hasAudience(aud any) bool {
	switch v := aud.(type) {
	case string:
		return v == tokenAudience
	case []any:
		for _, one := range v {
			if one == tokenAudience {
				return true
			}
		}
	}
	return false
}

func decodePart(part string, into any) error {
	raw, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, into)
}

// Token: "Authorization: Bearer", or ZOOMRIYA ID's cookie.
func Token(r *http.Request) string {
	if header := r.Header.Get("Authorization"); len(header) > 7 && strings.EqualFold(header[:7], "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	if cookie, err := r.Cookie(tokenCookie); err == nil {
		return cookie.Value
	}
	return ""
}
