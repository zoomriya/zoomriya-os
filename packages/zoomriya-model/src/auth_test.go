package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// testID is a stand-in ZOOMRIYA ID: a key, its JWKS, and tokens it signs.
type testID struct {
	private ed25519.PrivateKey
	server  *httptest.Server
	fetches int
}

func newTestID(t *testing.T) *testID {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := &testID{private: private}
	id.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		id.fetches++
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "OKP", "crv": "Ed25519", "kid": "k1", "alg": "EdDSA", "use": "sig",
			"x": base64.RawURLEncoding.EncodeToString(public),
		}}})
	}))
	t.Cleanup(id.server.Close)
	return id
}

func (id *testID) token(claims map[string]any, header map[string]any) string {
	if header == nil {
		header = map[string]any{"alg": "EdDSA", "typ": "JWT", "kid": "k1"}
	}
	full := map[string]any{
		"iss": "zoomriya-id", "aud": "zoomriya", "sub": "7",
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(), "role": "member",
	}
	for k, v := range claims {
		if v == nil {
			delete(full, k)
		} else {
			full[k] = v
		}
	}
	part := func(v any) string {
		raw, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	signed := part(header) + "." + part(full)
	return signed + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(id.private, []byte(signed)))
}

func (id *testID) auth() *Auth { return NewAuth(id.server.URL) }

func TestVerifyAcceptsZoomriyaIDTokens(t *testing.T) {
	id := newTestID(t)
	auth := id.auth()
	claims, err := auth.Verify(id.token(map[string]any{"role": "Admin"}, nil))
	if err != nil {
		t.Fatal(err)
	}
	if !claims.Admin() {
		t.Error("an admin's token isn't seen as an admin's")
	}
	if _, err := auth.Verify(id.token(map[string]any{"aud": []any{"other", "zoomriya"}}, nil)); err != nil {
		t.Errorf("an audience list with the apps' audience: %v", err)
	}
	member, _ := auth.Verify(id.token(nil, nil))
	if member.Admin() {
		t.Error("a member is seen as an admin")
	}
	if id.fetches != 1 {
		t.Errorf("the keys were fetched %d times, want once", id.fetches)
	}
}

func TestVerifyRefusesOtherTokens(t *testing.T) {
	id := newTestID(t)
	auth := id.auth()
	other := &testID{}
	_, other.private, _ = ed25519.GenerateKey(rand.Reader)
	cases := map[string]string{
		"expired":        id.token(map[string]any{"exp": time.Now().Add(-time.Second).Unix()}, nil),
		"other issuer":   id.token(map[string]any{"iss": "someone"}, nil),
		"other audience": id.token(map[string]any{"aud": "zoomriya-id-session"}, nil),
		"no subject":     id.token(map[string]any{"sub": nil}, nil),
		"no iat":         id.token(map[string]any{"iat": nil}, nil),
		"not EdDSA":      id.token(nil, map[string]any{"alg": "HS256", "kid": "k1"}),
		"unknown key":    id.token(nil, map[string]any{"alg": "EdDSA", "kid": "k2"}),
		"forged":         other.token(nil, nil),
		"not a token":    "abc",
	}
	for name, token := range cases {
		if _, err := auth.Verify(token); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if id.fetches > 2 {
		t.Errorf("bad tokens fetched the keys %d times", id.fetches)
	}
}

func TestTokenFromHeaderOrCookie(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer abc")
	if Token(r) != "abc" {
		t.Error("bearer token not read")
	}
	r = httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: "zoomriya.auth.token", Value: "xyz"})
	if Token(r) != "xyz" {
		t.Error("cookie token not read")
	}
}
