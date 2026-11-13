// Signing in with the box's ZOOMRIYA ID, as every app does (box-desktop.md
// §7), and People's requests (ZOOMRIYA ID's own /users). The same requests as
// the native desktop's signin.py; its words are the page's, so a refusal is a
// code here, which the page turns into the same sentence in the admin's
// language.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SessionCookie and TokenCookie are ZOOMRIYA ID's cookies
// (zoomriya_identity.identity's SESSION_COOKIE_NAME and AUTH_COOKIE_NAME).
const (
	SessionCookie = "zoomriya.auth.session"
	TokenCookie   = "zoomriya.auth.token"
)

// Problem is a refusal for the page to word: Code says which, Ticket goes
// with the second step, Detail is ZOOMRIYA ID's own words (English) where it
// gives them.
type Problem struct {
	Status int    `json:"-"`
	Code   string `json:"code"`
	Ticket string `json:"ticket,omitempty"`
	Detail string `json:"detail,omitempty"`
	// Error is the agent's own sentence, already meant for an admin.
	Error string `json:"error,omitempty"`
}

func problem(status int, code string) *Problem { return &Problem{Status: status, Code: code} }

// Member is a signed-in admin, as ZOOMRIYA ID knows them.
type Member struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

// Login is what a sign-in gives: the member, their access token, and the
// login's session cookie that renews it.
type Login struct {
	Member  Member
	Token   string
	Session string
	// Cookies: ZOOMRIYA ID's own, as it set them at this sign-in. The
	// browser gets them too, so the apps in Box Desktop's windows open
	// signed in, as the native desktop's app windows did.
	Cookies []*http.Cookie
}

type ZoomriyaID struct {
	API    string
	Client *http.Client
}

func NewZoomriyaID(api string) *ZoomriyaID {
	return &ZoomriyaID{API: strings.TrimRight(api, "/"), Client: &http.Client{Timeout: 15 * time.Second}}
}

type idAnswer struct {
	Status  int
	Body    map[string]any
	List    []any
	Header  http.Header
	Cookies []*http.Cookie
}

// errUnreachable: ZOOMRIYA ID didn't answer at all.
var errUnreachable = errors.New("ZOOMRIYA ID isn't responding")

func (z *ZoomriyaID) open(method, path string, body any, token, cookie string) (*idAnswer, error) {
	var data io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		data = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, z.API+path, data)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: SessionCookie, Value: cookie})
	}
	resp, err := z.Client.Do(req)
	if err != nil {
		return nil, errUnreachable
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	a := &idAnswer{Status: resp.StatusCode, Header: resp.Header, Cookies: resp.Cookies()}
	if len(bytes.TrimSpace(raw)) > 0 {
		if raw[0] == '[' {
			json.Unmarshal(raw, &a.List)
		} else {
			json.Unmarshal(raw, &a.Body)
		}
	}
	return a, nil
}

func str(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// signingIn: one step of a sign-in, its refusals as signin.py words them.
func (z *ZoomriyaID) signingIn(method, path string, body any, token string) (*idAnswer, *Problem) {
	a, err := z.open(method, path, body, token, "")
	if err != nil {
		return nil, problem(http.StatusBadGateway, "id_down")
	}
	if a.Status < 300 {
		return a, nil
	}
	secondStep := strings.Contains(path, "second-step")
	switch {
	case a.Status == 403 && str(a.Body, "code") == "new_password_needed":
		return nil, problem(403, "new_password_needed")
	case a.Status == 403 && str(a.Body, "code") == "second_step_needed":
		return nil, &Problem{Status: 403, Code: "second_step_needed", Ticket: str(a.Body, "ticket")}
	case a.Status == 403 && str(a.Body, "code") == "second_step_setup_needed":
		return nil, &Problem{Status: 403, Code: "second_step_setup_needed", Ticket: str(a.Body, "ticket")}
	case a.Status == 400 && secondStep:
		return nil, problem(400, "wrong_code")
	case a.Status == 401 && secondStep:
		return nil, problem(401, "too_slow")
	case a.Status == 401:
		return nil, problem(401, "wrong_password")
	case a.Status == 400 && strings.HasSuffix(path, "/new-password"):
		return nil, problem(400, "password_in_use")
	case a.Status == 429:
		// ZOOMRIYA ID locks one person's sign-in for 15 minutes after five
		// wrong passwords, wherever they were typed.
		if wait, _ := strconv.Atoi(a.Header.Get("Retry-After")); wait > 60 {
			return nil, problem(429, "locked")
		}
		return nil, problem(429, "too_many")
	}
	return nil, problem(http.StatusBadGateway, "id_failed")
}

func (z *ZoomriyaID) SignIn(username, password string) (*Login, *Problem) {
	a, p := z.signingIn("POST", "/auth/login",
		map[string]string{"username": strings.TrimSpace(username), "password": password}, "")
	if p != nil {
		return nil, p
	}
	return z.admin(a)
}

// SignInWithNewPassword: the first sign-in with a temporary password; their
// own replaces it.
func (z *ZoomriyaID) SignInWithNewPassword(username, password, newPassword string) (*Login, *Problem) {
	a, p := z.signingIn("POST", "/auth/login/new-password", map[string]string{
		"username": strings.TrimSpace(username), "password": password, "new_password": newPassword}, "")
	if p != nil {
		return nil, p
	}
	return z.admin(a)
}

// SignInWithCode: the second step, the authenticator app's code or a backup code.
func (z *ZoomriyaID) SignInWithCode(ticket, code string) (*Login, *Problem) {
	a, p := z.signingIn("POST", "/auth/login/second-step",
		map[string]string{"ticket": ticket, "code": strings.TrimSpace(code)}, "")
	if p != nil {
		return nil, p
	}
	return z.admin(a)
}

// SecondStepStart: a new authenticator app, its QR code (SVG) and the key to type.
func (z *ZoomriyaID) SecondStepStart(ticket string) (map[string]any, *Problem) {
	a, p := z.signingIn("POST", "/auth/second-step/setup", map[string]string{"ticket": ticket}, "")
	if p != nil {
		return nil, p
	}
	return a.Body, nil
}

// SecondStepFinish: the app's first code; signed in, and the backup codes,
// shown once.
func (z *ZoomriyaID) SecondStepFinish(ticket, code string) (*Login, []string, *Problem) {
	a, p := z.signingIn("POST", "/auth/second-step/setup/finish",
		map[string]string{"ticket": ticket, "code": strings.TrimSpace(code)}, "")
	if p != nil {
		return nil, nil, p
	}
	login, p := z.admin(a)
	if p != nil {
		return nil, nil, p
	}
	var codes []string
	if list, ok := a.Body["backup_codes"].([]any); ok {
		for _, c := range list {
			if s, ok := c.(string); ok {
				codes = append(codes, s)
			}
		}
	}
	return login, codes, nil
}

// admin: who signed in, if they're an admin; Box Desktop is for the box's
// admins only.
func (z *ZoomriyaID) admin(a *idAnswer) (*Login, *Problem) {
	token := str(a.Body, "access_token")
	me, p := z.signingIn("GET", "/auth/me", nil, token)
	if p != nil {
		return nil, p
	}
	if str(me.Body, "role") != "admin" {
		z.open("POST", "/auth/logout", nil, token, "")
		return nil, problem(403, "not_admin")
	}
	id, _ := me.Body["id"].(float64)
	name := str(me.Body, "display_name")
	if name == "" {
		name = str(me.Body, "username")
	}
	login := &Login{Member: Member{ID: int(id), Username: str(me.Body, "username"), Name: name}, Token: token}
	for _, c := range a.Cookies {
		if c.Name == SessionCookie {
			login.Session = c.Value
		}
		if c.Name == SessionCookie || c.Name == TokenCookie {
			login.Cookies = append(login.Cookies, c)
		}
	}
	return login, nil
}

// PasswordIs: is this the signed-in admin's password? For "your password
// again": nothing is signed in or out, and no code is asked.
func (z *ZoomriyaID) PasswordIs(login *Login, password string) (bool, *Problem) {
	for attempt := 1; attempt <= 2; attempt++ {
		a, err := z.open("POST", "/auth/me/confirm-password", map[string]string{"password": password}, login.Token, "")
		if err != nil {
			return false, problem(http.StatusBadGateway, "id_down")
		}
		switch {
		case a.Status < 300:
			return true, nil
		case a.Status == 401 && attempt == 1 && z.Renew(login):
			continue
		case a.Status == 401:
			return false, nil
		case a.Status == 429:
			return false, problem(429, "too_many")
		default:
			return false, problem(http.StatusBadGateway, "check_failed")
		}
	}
	return false, nil
}

func (z *ZoomriyaID) SignOut(login *Login) {
	z.open("POST", "/auth/logout", nil, login.Token, "")
}

// Renew: a new access token with the login's session cookie.
func (z *ZoomriyaID) Renew(login *Login) bool {
	if login.Session == "" {
		return false
	}
	a, err := z.open("POST", "/auth/refresh", nil, "", login.Session)
	if err != nil || a.Status >= 300 {
		return false
	}
	if t := str(a.Body, "access_token"); t != "" {
		login.Token = t
		return true
	}
	return false
}

// Ask is a /users request as this admin, renewing the token once if it ran
// out. The answer is ZOOMRIYA ID's JSON, passed on as it is.
func (z *ZoomriyaID) Ask(login *Login, method, path string, body any) (any, *Problem) {
	for attempt := 1; attempt <= 2; attempt++ {
		a, err := z.open(method, path, body, login.Token, "")
		if err != nil {
			return nil, problem(http.StatusBadGateway, "id_down")
		}
		switch {
		case a.Status < 300:
			if a.List != nil {
				return a.List, nil
			}
			if a.Body == nil {
				return map[string]any{}, nil
			}
			return a.Body, nil
		case a.Status == 401 && attempt == 1 && z.Renew(login):
			continue
		case a.Status == 401:
			return nil, problem(401, "sign_in_again")
		case a.Status == 403:
			return nil, problem(403, "admins_only")
		case a.Status == 409:
			return nil, problem(409, "username_taken")
		case a.Status == 400:
			p := problem(400, "not_accepted")
			p.Detail = str(a.Body, "detail")
			return nil, p
		default:
			return nil, problem(http.StatusBadGateway, "change_failed")
		}
	}
	return nil, problem(401, "sign_in_again")
}
