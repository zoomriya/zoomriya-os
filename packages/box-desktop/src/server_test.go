package main

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeID is ZOOMRIYA ID with two people: ada, an admin (password "right"),
// and kid, who isn't. "temp" is a temporary password; "twostep" asks for a code.
func fakeID(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	body := func(r *http.Request) map[string]any {
		var b map[string]any
		json.NewDecoder(r.Body).Decode(&b)
		return b
	}
	answer := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(v)
	}
	login := func(w http.ResponseWriter, user string) {
		http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "idsession-" + user})
		answer(w, 200, map[string]any{"access_token": "token-" + user})
	}
	mux.HandleFunc("POST /id/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		b := body(r)
		switch {
		case b["username"] == "ada" && b["password"] == "right", b["username"] == "kid" && b["password"] == "right":
			login(w, b["username"].(string))
		case b["password"] == "temp":
			answer(w, 403, map[string]any{"code": "new_password_needed"})
		case b["password"] == "twostep":
			answer(w, 403, map[string]any{"code": "second_step_needed", "ticket": "T1"})
		case b["password"] == "locked":
			w.Header().Set("Retry-After", "900")
			answer(w, 429, map[string]any{})
		default:
			answer(w, 401, map[string]any{"detail": "no"})
		}
	})
	mux.HandleFunc("POST /id/api/v1/auth/login/second-step", func(w http.ResponseWriter, r *http.Request) {
		if body(r)["code"] == "123456" {
			login(w, "ada")
			return
		}
		answer(w, 400, map[string]any{})
	})
	mux.HandleFunc("GET /id/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Authorization") {
		case "Bearer token-ada":
			answer(w, 200, map[string]any{"id": 1, "username": "ada", "display_name": "Ada", "role": "admin"})
		case "Bearer token-kid":
			answer(w, 200, map[string]any{"id": 2, "username": "kid", "role": "family"})
		default:
			answer(w, 401, map[string]any{})
		}
	})
	mux.HandleFunc("POST /id/api/v1/auth/me/confirm-password", func(w http.ResponseWriter, r *http.Request) {
		if body(r)["password"] == "right" {
			answer(w, 200, map[string]any{})
			return
		}
		answer(w, 401, map[string]any{})
	})
	mux.HandleFunc("POST /id/api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) { answer(w, 200, nil) })
	mux.HandleFunc("GET /id/api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		answer(w, 200, []any{map[string]any{"id": 1, "username": "ada"}})
	})
	mux.HandleFunc("PATCH /id/api/v1/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		answer(w, 200, map[string]any{"id": 3, "username": "bo", "display_name": "Bo"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// fakeAgent answers every request ok, and keeps what it was asked.
type fakeAgent struct {
	mu   sync.Mutex
	seen []map[string]any
}

func (f *fakeAgent) last() map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.seen) == 0 {
		return nil
	}
	return f.seen[len(f.seen)-1]
}

func startAgent(t *testing.T, socket string) *fakeAgent {
	t.Helper()
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	f := &fakeAgent{}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			line, _ := bufio.NewReader(conn).ReadBytes('\n')
			var req map[string]any
			json.Unmarshal(line, &req)
			f.mu.Lock()
			f.seen = append(f.seen, req)
			f.mu.Unlock()
			conn.Write([]byte(`{"ok":true,"op":"` + req["op"].(string) + `"}` + "\n"))
			conn.Close()
		}
	}()
	return f
}

type testBox struct {
	t      *testing.T
	srv    *Server
	agent  *fakeAgent
	lines  []map[string]string
	home   http.Handler
	screen http.Handler
	cookie string
	token  string
	side   Listener
	remote string
	host   string
	origin string
}

func newBox(t *testing.T) *testBox {
	t.Helper()
	dir := t.TempDir()
	id := fakeID(t)
	socket := filepath.Join(dir, "agent.sock")
	b := &testBox{t: t}
	b.agent = startAgent(t, socket)
	srv, err := NewServer(Config{
		TLSAddr: ":7443", ScreenAddr: "127.0.0.1:7444", CertsDir: filepath.Join(dir, "certs"),
		AdminDir: filepath.Join(dir, "admin"), IDAPI: id.URL + "/id/api/v1", Agent: socket,
		SetupDone: filepath.Join(dir, "setup-done"), StateDir: filepath.Join(dir, "state"),
	})
	if err != nil {
		t.Fatal(err)
	}
	srv.Journal = Journal{Lines: &b.lines}
	srv.Hardware = NewFakeHardware()
	srv.Certs.names = map[string]bool{"maplehouse.zoomriya.app": true}
	b.srv = srv
	b.home, b.screen = srv.Handler(ListenerHome), srv.Handler(ListenerScreen)
	b.atHome()
	return b
}

func (b *testBox) atHome() {
	b.side, b.remote, b.host = ListenerHome, "192.168.1.20:5555", "maplehouse.zoomriya.app:7443"
	b.origin = "https://" + b.host
}

func (b *testBox) atScreen() {
	b.side, b.remote, b.host = ListenerScreen, "127.0.0.1:5555", "maplehouse.zoomriya.app:7444"
	b.origin = "https://" + b.host
}

func (b *testBox) do(method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	b.t.Helper()
	var rd *strings.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = strings.NewReader(string(raw))
	} else {
		rd = strings.NewReader("")
	}
	r := httptest.NewRequest(method, path, rd)
	r.Host, r.RemoteAddr = b.host, b.remote
	if method != "GET" {
		r.Header.Set("Origin", b.origin)
		r.Header.Set("Content-Type", "application/json")
	}
	if b.cookie != "" {
		r.Header.Set("Cookie", b.cookie)
	}
	if b.token != "" {
		r.Header.Set(tokenHeader, b.token)
	}
	w := httptest.NewRecorder()
	if b.side == ListenerScreen {
		b.screen.ServeHTTP(w, r)
	} else {
		b.home.ServeHTTP(w, r)
	}
	var answer map[string]any
	json.Unmarshal(w.Body.Bytes(), &answer)
	return w, answer
}

func (b *testBox) signIn() {
	b.t.Helper()
	w, a := b.do("POST", "/api/sign-in", map[string]string{"username": "ada", "password": "right"})
	if w.Code != 200 || a["signed_in"] != true {
		b.t.Fatalf("sign-in: %d %v", w.Code, a)
	}
	b.cookie = strings.SplitN(w.Header().Get("Set-Cookie"), ";", 2)[0]
	b.token = a["token"].(string)
}

func TestSignInSetsAStrictCookie(t *testing.T) {
	b := newBox(t)
	w, a := b.do("POST", "/api/sign-in", map[string]string{"username": "ada", "password": "right"})
	if w.Code != 200 || a["member"].(map[string]any)["name"] != "Ada" {
		t.Fatalf("%d %v", w.Code, a)
	}
	c := w.Header().Get("Set-Cookie")
	for _, want := range []string{"__Host-box-desktop=", "Path=/", "HttpOnly", "Secure", "SameSite=Strict"} {
		if !strings.Contains(c, want) {
			t.Errorf("cookie %q lacks %q", c, want)
		}
	}
	if len(b.lines) != 1 || b.lines[0]["MESSAGE"] != "ada signed in from 192.168.1.20" {
		t.Errorf("journal: %v", b.lines)
	}
	if strings.Contains(w.Body.String(), "token-ada") || strings.Contains(w.Body.String(), "idsession") {
		t.Error("ZOOMRIYA ID's token reached the page's script")
	}
	// ZOOMRIYA ID's session cookie, for the apps in Box Desktop's windows.
	handed := false
	for _, c := range w.Result().Cookies() {
		if c.Name == SessionCookie && c.Value == "idsession-ada" && c.HttpOnly && c.Secure {
			handed = true
		}
	}
	if !handed {
		t.Errorf("ZOOMRIYA ID's session cookie wasn't passed on: %v", w.Header()["Set-Cookie"])
	}
}

func TestSignInRefusals(t *testing.T) {
	b := newBox(t)
	for password, code := range map[string]string{
		"wrong": "wrong_password", "temp": "new_password_needed", "twostep": "second_step_needed", "locked": "locked",
	} {
		_, a := b.do("POST", "/api/sign-in", map[string]string{"username": "ada", "password": password})
		if a["code"] != code {
			t.Errorf("%s: %v, want %s", password, a, code)
		}
	}
	_, a := b.do("POST", "/api/sign-in", map[string]string{"username": "kid", "password": "right"})
	if a["code"] != "not_admin" {
		t.Errorf("not an admin: %v", a)
	}
}

func TestSecondStep(t *testing.T) {
	b := newBox(t)
	_, a := b.do("POST", "/api/sign-in", map[string]string{"username": "ada", "password": "twostep"})
	if a["ticket"] != "T1" {
		t.Fatalf("%v", a)
	}
	_, a = b.do("POST", "/api/sign-in/code", map[string]string{"ticket": "T1", "code": "000000"})
	if a["code"] != "wrong_code" {
		t.Errorf("%v", a)
	}
	if last := b.lines[len(b.lines)-1]; last["ZOOMRIYA_BY"] != "ada" {
		t.Errorf("the refusal's journal line: %v", last)
	}
	w, a := b.do("POST", "/api/sign-in/code", map[string]string{"ticket": "T1", "code": "123456"})
	if w.Code != 200 || a["signed_in"] != true {
		t.Errorf("%d %v", w.Code, a)
	}
}

func TestGuards(t *testing.T) {
	b := newBox(t)
	b.signIn()
	ask := map[string]any{"request": map[string]any{"op": "status"}}

	if w, _ := b.do("POST", "/api/agent", ask); w.Code != 200 {
		t.Fatalf("signed in: %d", w.Code)
	}
	b.origin = "https://evil.example"
	if w, a := b.do("POST", "/api/agent", ask); a["code"] != "wrong_origin" {
		t.Errorf("another site: %d %v", w.Code, a)
	}
	b.atHome()
	token := b.token
	b.token = ""
	if _, a := b.do("POST", "/api/agent", ask); a["code"] != "wrong_token" {
		t.Errorf("no token: %v", a)
	}
	b.token = token
	b.host = "evil.example:7443"
	if w, _ := b.do("GET", "/api/session", nil); w.Code != http.StatusMisdirectedRequest {
		t.Errorf("another name (DNS rebinding): %d", w.Code)
	}
	b.atHome()
	b.remote = "203.0.113.9:4000"
	if w, _ := b.do("GET", "/api/session", nil); w.Code != http.StatusForbidden {
		t.Errorf("from the internet: %d", w.Code)
	}
	b.atHome()
	b.host = "maplehouse.zoomriya.app:7444"
	if w, _ := b.do("GET", "/api/session", nil); w.Code != http.StatusMisdirectedRequest {
		t.Errorf("the screen's port on the home side: %d", w.Code)
	}
	b.atHome()
	// The home network's cookie means nothing on the box's screen's side.
	b.atScreen()
	if _, a := b.do("GET", "/api/session", nil); a["signed_in"] != false {
		t.Errorf("cookie crossed sides: %v", a)
	}
	w, _ := b.do("GET", "/", nil)
	if csp := w.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP: %q", csp)
	}
}

func TestAgentRules(t *testing.T) {
	b := newBox(t)
	b.signIn()
	// The page can't say who asked, nor ask for anything off the list.
	b.do("POST", "/api/agent", map[string]any{"request": map[string]any{"op": "status", "by": "mallory"}})
	if got := b.agent.last()["by"]; got != "ada" {
		t.Errorf("by: %v", got)
	}
	for _, op := range []string{"first-admin", "setup-finish", "admin-recover", "nonsense"} {
		if _, a := b.do("POST", "/api/agent", map[string]any{"request": map[string]any{"op": op}}); a["code"] != "not_allowed" {
			t.Errorf("%s: %v", op, a)
		}
	}
	// Risky ones ask for the password again.
	if _, a := b.do("POST", "/api/agent", map[string]any{"request": map[string]any{"op": "shutdown"}}); a["code"] != "password_needed" {
		t.Errorf("shutdown: %v", a)
	}
	wrong := "nope"
	if _, a := b.do("POST", "/api/agent", map[string]any{"request": map[string]any{"op": "shutdown"}, "password": wrong}); a["code"] != "not_your_password" {
		t.Errorf("shutdown, wrong password: %v", a)
	}
	if _, a := b.do("POST", "/api/agent", map[string]any{"request": map[string]any{"op": "advanced-access", "action": "on"}}); a["code"] != "password_needed" {
		t.Errorf("advanced access on: %v", a)
	}
	if w, _ := b.do("POST", "/api/agent", map[string]any{"request": map[string]any{"op": "advanced-access", "action": "off"}}); w.Code != 200 {
		t.Errorf("advanced access off needs no password: %d", w.Code)
	}
	// A restore, then its restart without asking again, once.
	if w, _ := b.do("POST", "/api/agent", map[string]any{"request": map[string]any{"op": "restore-point-restore", "number": 3}, "password": "right"}); w.Code != 200 {
		t.Fatalf("restore: %d", w.Code)
	}
	if w, _ := b.do("POST", "/api/agent", map[string]any{"request": map[string]any{"op": "restart"}}); w.Code != 200 {
		t.Errorf("restart after restore: %d", w.Code)
	}
	if _, a := b.do("POST", "/api/agent", map[string]any{"request": map[string]any{"op": "restart"}}); a["code"] != "password_needed" {
		t.Errorf("a second restart: %v", a)
	}
}

func TestRecoverAtTheBoxOnly(t *testing.T) {
	b := newBox(t)
	fields := map[string]string{"setup_code": "K7QM-4XRT", "username": "ada", "password": "new"}
	if _, a := b.do("POST", "/api/recover", fields); a["code"] != "at_the_box" {
		t.Errorf("home network: %v", a)
	}
	b.atScreen()
	if w, _ := b.do("POST", "/api/recover", fields); w.Code != 200 || b.agent.last()["op"] != "admin-recover" {
		t.Errorf("box's screen: %d %v", w.Code, b.agent.last())
	}
}

func TestPeopleRoleNeedsPasswordAndSignsOut(t *testing.T) {
	b := newBox(t)
	b.signIn()
	change := map[string]any{"fields": map[string]any{"role": "family"}}
	if _, a := b.do("PATCH", "/api/people/3", change); a["code"] != "password_needed" {
		t.Errorf("%v", a)
	}
	change["password"] = "right"
	if w, _ := b.do("PATCH", "/api/people/3", change); w.Code != 200 {
		t.Fatalf("%d", w.Code)
	}
	last := b.lines[len(b.lines)-1]
	if last["MESSAGE"] != "Removed Bo as admin" || last["ZOOMRIYA_OP"] != "people-role" {
		t.Errorf("journal: %v", last)
	}
	// Removing yourself as admin signs you out here.
	change["fields"] = map[string]any{"is_active": false}
	b.do("PATCH", "/api/people/1", change)
	if _, a := b.do("GET", "/api/session", nil); a["signed_in"] != false {
		t.Errorf("still signed in after turning off their own account: %v", a)
	}
}

func TestSessionsRunOut(t *testing.T) {
	b := newBox(t)
	now := time.Now()
	b.srv.Sessions.Now = func() time.Time { return now }
	b.signIn()
	now = now.Add(31 * time.Minute)
	if _, a := b.do("GET", "/api/session", nil); a["signed_in"] != false {
		t.Errorf("idle 31 minutes, still signed in")
	}
}

func TestScreenOnlyHere(t *testing.T) {
	b := newBox(t)
	b.signIn()
	if _, a := b.do("POST", "/api/screen/power", map[string]bool{"on": false}); a["code"] != "at_the_box" {
		t.Errorf("screen power from the home network: %v", a)
	}
	b.cookie, b.token = "", ""
	b.atScreen()
	b.signIn()
	if w, _ := b.do("POST", "/api/screen/power", map[string]bool{"on": false}); w.Code != 200 {
		t.Errorf("screen power at the box: %d", w.Code)
	}
	if !b.srv.Sessions.AtScreen(1) {
		t.Error("Ada isn't seen at the screen")
	}
}

func TestDisplayOnlyKnownModes(t *testing.T) {
	b := newBox(t)
	b.signIn()
	bad := map[string]any{"name": "HDMI-A-1", "mode": map[string]int{"width": 7, "height": 7, "refresh": 1},
		"scale": 1, "transform": "normal"}
	if _, a := b.do("POST", "/api/display", bad); a["code"] != "bad_request" {
		t.Errorf("an unknown mode: %v", a)
	}
	good := map[string]any{"name": "HDMI-A-1", "mode": map[string]int{"width": 1920, "height": 1080, "refresh": 60000},
		"scale": 1.25, "transform": "90"}
	if w, _ := b.do("POST", "/api/display", good); w.Code != 200 {
		t.Errorf("a listed mode: %d", w.Code)
	}
}

func TestLockHoldsUntilThePassword(t *testing.T) {
	b := newBox(t)
	b.signIn()
	b.do("POST", "/api/lock", map[string]any{})
	if _, a := b.do("POST", "/api/agent", map[string]any{"request": map[string]any{"op": "status"}}); a["code"] != "screen_locked" {
		t.Errorf("locked, the agent: %v", a)
	}
	if _, a := b.do("GET", "/api/session", nil); a["locked"] != true {
		t.Errorf("opened again, not locked: %v", a)
	}
	if _, a := b.do("POST", "/api/unlock", map[string]string{"password": "nope"}); a["code"] != "wrong_unlock" {
		t.Errorf("wrong password: %v", a)
	}
	if w, _ := b.do("POST", "/api/unlock", map[string]string{"password": "right"}); w.Code != 200 {
		t.Errorf("unlock: %d", w.Code)
	}
	if w, _ := b.do("POST", "/api/agent", map[string]any{"request": map[string]any{"op": "status"}}); w.Code != 200 {
		t.Errorf("unlocked, the agent: %d", w.Code)
	}
}

func TestFirstSetupAtTheBoxOnlyAndOnce(t *testing.T) {
	b := newBox(t)
	ask := map[string]any{"request": map[string]any{"op": "keyboard-set", "layout": "fr", "by": "mallory"}}
	if _, a := b.do("POST", "/api/setup/agent", ask); a["code"] != "at_the_box" {
		t.Errorf("from the home network: %v", a)
	}
	b.atScreen()
	if _, a := b.do("GET", "/api/session", nil); a["setup"] != true {
		t.Errorf("not set up, the session doesn't say so: %v", a)
	}
	if w, _ := b.do("POST", "/api/setup/agent", ask); w.Code != 200 || b.agent.last()["by"] != "first-setup" {
		t.Errorf("at the box: %d %v", w.Code, b.agent.last())
	}
	if _, a := b.do("POST", "/api/setup/agent", map[string]any{"request": map[string]any{"op": "factory-reset"}}); a["code"] != "not_allowed" {
		t.Errorf("a request the setup never makes: %v", a)
	}
	if _, a := b.do("POST", "/api/setup/progress", map[string]any{"language": "fr", "evil": 1}); a["language"] != "fr" || a["evil"] != nil {
		t.Errorf("progress: %v", a)
	}
	// Set up: the setup's door closes for good.
	os.WriteFile(b.srv.Cfg.SetupDone, nil, 0o644)
	if _, a := b.do("POST", "/api/setup/agent", ask); a["code"] != "set_up" {
		t.Errorf("after setup: %v", a)
	}
}

func TestPrefsChecked(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Folder: dir}
	prefs, err := s.Save(7, map[string]any{
		"theme": "neon", "accent": "teal", "dock": []any{"photos", "photos", "BAD", "vault"},
		"shortcuts": []any{"storage", "app:photos", "rm -rf"}, "mouse": map[string]any{"speed": 5.0},
		"evil": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if prefs["theme"] != "midnight" || prefs["accent"] != "teal" || len(prefs["dock"].([]any)) != 2 ||
		len(prefs["shortcuts"].([]any)) != 2 || prefs["mouse"].(map[string]any)["speed"] != 1.0 || prefs["evil"] != nil {
		t.Errorf("%v", prefs)
	}
	// The same file name as the native desktop's: sha256 of the member id.
	if _, err := os.Stat(filepath.Join(dir, "7902699be42c8a8e46fbbb4501726517e86b22c56a189f7625a6da49081b2451.json")); err != nil {
		t.Error(err)
	}
	fi, _ := os.Stat(s.path(7))
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", fi.Mode())
	}
	if _, err := s.SaveBox(map[string]any{"lock_after": 3.0}); err == nil {
		t.Error("lock_after 3 accepted")
	}
	if box, _ := s.SaveBox(map[string]any{"background": "ocean", "lock_after": 15.0}); box.Background != "ocean" || box.LockAfter != 15 {
		t.Errorf("%v", box)
	}
}
