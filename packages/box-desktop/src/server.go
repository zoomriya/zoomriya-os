// The routes and their guards (box-desktop.md §8): the box's own names only,
// the home network only, an exact Origin and the page's token for every
// change, a strict cookie, and a Content Security Policy.
package main

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Listener is the side a request came in on.
type Listener int

const (
	// ListenerHome: browsers on the home network, over HTTPS on 7443.
	ListenerHome Listener = iota
	// ListenerScreen: the box's own screen, over HTTPS on 127.0.0.1:7444
	// (box-desktop-screen opens https://<name>.local:7444).
	ListenerScreen
)

func (l Listener) String() string {
	if l == ListenerScreen {
		return "screen"
	}
	return "home"
}

type Server struct {
	Cfg      Config
	Certs    *Certificates
	Sessions *Sessions
	ID       *ZoomriyaID
	Agent    *Agent
	Store    *Store
	Journal  Journal
	Web      fs.FS
	Now      func() time.Time
	Hardware Hardware
	Places   *Places
	Progress *setupProgress
}

func NewServer(cfg Config) (*Server, error) {
	web, err := webFiles(cfg.Web)
	if err != nil {
		return nil, err
	}
	s := &Server{
		Cfg:      cfg,
		Certs:    NewCertificates(cfg.CertsDir),
		Sessions: NewSessions(),
		ID:       NewZoomriyaID(cfg.IDAPI),
		Agent:    &Agent{Socket: cfg.Agent},
		Store:    &Store{Folder: cfg.AdminDir},
		Web:      web,
		Now:      time.Now,
		Places:   &Places{Root: cfg.Root},
		Progress: &setupProgress{path: filepath.Join(strings.Split(cfg.StateDir, ":")[0], "setup.json")},
	}
	if cfg.Fake {
		s.Hardware = NewFakeHardware()
	} else {
		s.Hardware = NewBoxHardware()
	}
	// A sign-in that ran out also ends its ZOOMRIYA ID session.
	s.Sessions.Ended = func(se *Session) {
		if l := se.Login(); l != nil {
			go s.ID.SignOut(l)
		}
		// The next one at the box's screen starts with the box's keyboard and
		// a right-handed mouse.
		if se.Listener == ListenerScreen {
			go s.resetScreen()
		}
	}
	return s, nil
}

const (
	cookieHome   = "__Host-box-desktop"
	cookieScreen = "box-desktop"
	tokenHeader  = "X-Box-Desktop-Token"
	maxBody      = 1 << 20
)

func (s *Server) cookieName(l Listener) string {
	if s.Cfg.Insecure {
		return cookieScreen
	}
	return cookieHome
}

type handlerFunc func(w http.ResponseWriter, r *http.Request, l Listener, se *Session)

// Handler serves one side.
func (s *Server) Handler(l Listener) http.Handler {
	mux := http.NewServeMux()
	open := func(pattern string, h handlerFunc) { mux.HandleFunc(pattern, s.wrap(l, false, h)) }
	signed := func(pattern string, h handlerFunc) { mux.HandleFunc(pattern, s.wrap(l, true, h)) }

	open("GET /api/session", s.session)
	open("POST /api/sign-in", s.signIn)
	open("POST /api/sign-in/new-password", s.signInNewPassword)
	open("POST /api/sign-in/code", s.signInCode)
	open("POST /api/sign-in/second-step/setup", s.secondStepSetup)
	open("POST /api/sign-in/second-step/finish", s.secondStepFinish)
	open("POST /api/recover", s.recover)
	open("GET /api/box/picture", s.boxPicture)
	signed("POST /api/sign-out", s.signOut)
	signed("POST /api/lock", s.lock)
	signed("POST /api/unlock", s.unlock)
	signed("POST /api/agent", s.agent)
	signed("GET /api/prefs", s.prefsGet)
	signed("PUT /api/prefs", s.prefsPut)
	signed("PATCH /api/box", s.boxPatch)
	signed("GET /api/people", s.people)
	signed("GET /api/people/activity", s.activity)
	signed("POST /api/people", s.personAdd)
	signed("PATCH /api/people/{id}", s.personChange)
	signed("GET /api/events", s.events)
	open("GET /api/places", s.places)
	open("GET /api/places/subdivisions", s.subdivisions)
	open("GET /api/places/locale", s.locale)
	open("GET /api/places/zones", s.zonesIn)
	signed("GET /api/info", s.info)
	signed("GET /api/sound", s.soundGet)
	signed("POST /api/sound", s.soundPost)
	signed("GET /api/sound/level", s.soundLevel)
	signed("GET /api/display", s.displayGet)
	signed("POST /api/display", s.displayPost)
	signed("POST /api/screen/power", s.screenPower)
	signed("POST /api/box/picture", s.boxPictureSave)
	open("GET /api/setup", s.setupState)
	open("POST /api/setup/progress", s.setupProgressPost)
	open("POST /api/setup/agent", s.setupAgent)
	open("GET /api/setup/internet", s.setupInternet)
	open("POST /api/setup/second-step/start", s.setupSecondStepStart)
	open("POST /api/setup/second-step/finish", s.setupSecondStepFinish)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		s.headers(w, r, l)
		writeProblem(w, problem(http.StatusNotFound, "not_found"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !s.allowed(w, r, l) {
			return
		}
		s.headers(w, r, l)
		s.static(w, r)
	})
	return mux
}

// allowed: the box's own name, from the home network (or the box itself).
func (s *Server) allowed(w http.ResponseWriter, r *http.Request, l Listener) bool {
	if !s.hostAllowed(r.Host, l) {
		http.Error(w, "Not this box's name.", http.StatusMisdirectedRequest)
		return false
	}
	if !s.Cfg.Insecure && !fromHome(r.RemoteAddr) {
		http.Error(w, "Box Desktop opens from the home network only.", http.StatusForbidden)
		return false
	}
	return true
}

func (s *Server) hostAllowed(host string, l Listener) bool {
	if s.Cfg.Insecure {
		return true
	}
	name, port, err := net.SplitHostPort(host)
	if err != nil {
		name, port = host, "443"
	}
	addr := s.Cfg.TLSAddr
	if l == ListenerScreen {
		addr = s.Cfg.ScreenAddr
	}
	_, want, _ := net.SplitHostPort(addr)
	return port == want && s.Certs.Names(strings.Trim(name, "[]"))
}

// fromHome: an address on the home network: private, link-local or the box's own.
func fromHome(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return false
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	ip = ip.Unmap()
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()
}

func (s *Server) headers(w http.ResponseWriter, r *http.Request, l Listener) {
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; "+
		"style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self'; "+
		"font-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'; "+
		"frame-src 'self' https://"+s.frameHost(r)+" "+s.Cfg.AppBase)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	if !s.Cfg.Insecure {
		h.Set("Strict-Transport-Security", "max-age=31536000")
	}
}

// origin: what the page's own Origin is on this request.
func (s *Server) origin(r *http.Request, l Listener) string {
	scheme := "https"
	if s.Cfg.Insecure {
		scheme = "http"
	}
	return scheme + "://" + r.Host
}

func (s *Server) wrap(l Listener, needSignIn bool, h handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.allowed(w, r, l) {
			return
		}
		s.headers(w, r, l)
		w.Header().Set("Cache-Control", "no-store")
		change := r.Method != http.MethodGet && r.Method != http.MethodHead
		if change {
			// zoomriya.app isn't on the Public Suffix List: the cookie alone
			// isn't trusted, the Origin must be the page's own.
			if r.Header.Get("Origin") != s.origin(r, l) {
				writeProblem(w, problem(http.StatusForbidden, "wrong_origin"))
				return
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				writeProblem(w, problem(http.StatusUnsupportedMediaType, "json_only"))
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		}
		var se *Session
		if c, err := r.Cookie(s.cookieName(l)); err == nil {
			se = s.Sessions.Find(c.Value, l)
		}
		if needSignIn {
			if se == nil || se.Login() == nil {
				writeProblem(w, problem(http.StatusUnauthorized, "signed_out"))
				return
			}
			if change && !se.TokenMatches(r.Header.Get(tokenHeader)) {
				writeProblem(w, problem(http.StatusForbidden, "wrong_token"))
				return
			}
			// A locked desktop: its password or Switch account, and nothing else.
			if se.Locked() && !lockOpen[r.URL.Path] {
				writeProblem(w, problem(http.StatusForbidden, "screen_locked"))
				return
			}
		}
		h(w, r, l, se)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeProblem(w http.ResponseWriter, p *Problem) {
	writeJSON(w, p.Status, p)
}

func readJSON(r *http.Request, v any) *Problem {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return problem(http.StatusBadRequest, "bad_request")
	}
	return nil
}

// --- Signing in -------------------------------------------------------------

type sessionAnswer struct {
	SignedIn bool    `json:"signed_in"`
	Side     string  `json:"side"`
	Member   *Member `json:"member,omitempty"`
	Token    string  `json:"token,omitempty"`
	Prefs    Prefs   `json:"prefs,omitempty"`
	Box      Box     `json:"box"`
	// The box's language (our screens' code) and its OS's name, for the
	// sign-in screen as well.
	Language string `json:"language"`
	Locked   bool   `json:"locked,omitempty"`
	OS       string `json:"os"`
	// HasPicture: the family's own background is on the box.
	HasPicture bool `json:"has_picture"`
	// Setup: the box isn't set up yet; its own screen shows the first setup.
	Setup bool `json:"setup,omitempty"`
}

func (s *Server) answer(se *Session, l Listener) sessionAnswer {
	_, err := os.Stat(s.Store.Picture())
	a := sessionAnswer{Side: l.String(), Box: s.Store.Box(), Language: s.Places.BoxLanguage(),
		OS: s.Places.OSName(), HasPicture: err == nil, Setup: !s.SetUp()}
	if se != nil {
		if login := se.Login(); login != nil {
			m := login.Member
			a.SignedIn, a.Member, a.Token, a.Prefs = true, &m, se.Token, s.Store.Load(m.ID)
			a.Locked = se.Locked()
		}
	}
	return a
}

func (s *Server) session(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	writeJSON(w, http.StatusOK, s.answer(se, l))
}

// signedIn keeps the new sign-in, sets its cookie and answers as /api/session.
func (s *Server) signedIn(w http.ResponseWriter, r *http.Request, l Listener, login *Login, extra map[string]any) {
	s.Journal.SignIn(login.Member.Username, l, r.RemoteAddr, true, "")
	se := s.Sessions.Start(login, l, r.RemoteAddr)
	http.SetCookie(w, &http.Cookie{
		Name: s.cookieName(l), Value: se.ID, Path: "/",
		Secure: !s.Cfg.Insecure, HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
	// ZOOMRIYA ID's own, for the box's name: the apps in Box Desktop's
	// windows open signed in as this admin (cookies aren't kept apart by port).
	for _, c := range login.Cookies {
		http.SetCookie(w, &http.Cookie{Name: c.Name, Value: c.Value, Path: "/", MaxAge: c.MaxAge,
			Secure: !s.Cfg.Insecure, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	}
	if l == ListenerScreen {
		go s.applyScreen(s.Store.Load(login.Member.ID))
	}
	answer := map[string]any{}
	raw, _ := json.Marshal(s.answer(se, l))
	json.Unmarshal(raw, &answer)
	for k, v := range extra {
		answer[k] = v
	}
	writeJSON(w, http.StatusOK, answer)
}

// refused: the journal hears why, the page the code.
func (s *Server) refused(w http.ResponseWriter, r *http.Request, l Listener, username string, p *Problem) {
	switch p.Code {
	case "new_password_needed", "second_step_needed", "second_step_setup_needed":
		// Not refused: the next step.
	default:
		s.Journal.SignIn(username, l, r.RemoteAddr, false, p.Code)
	}
	writeProblem(w, p)
}

func (s *Server) signIn(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	var b struct{ Username, Password string }
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	login, p := s.ID.SignIn(b.Username, b.Password)
	if p != nil {
		s.Sessions.Pending(p.Ticket, b.Username)
		s.refused(w, r, l, b.Username, p)
		return
	}
	s.signedIn(w, r, l, login, nil)
}

func (s *Server) signInNewPassword(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	var b struct {
		Username, Password string
		NewPassword        string `json:"new_password"`
	}
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	login, p := s.ID.SignInWithNewPassword(b.Username, b.Password, b.NewPassword)
	if p != nil {
		s.Sessions.Pending(p.Ticket, b.Username)
		s.refused(w, r, l, b.Username, p)
		return
	}
	s.signedIn(w, r, l, login, nil)
}

func (s *Server) signInCode(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	var b struct{ Ticket, Code string }
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	login, p := s.ID.SignInWithCode(b.Ticket, b.Code)
	if p != nil {
		s.refused(w, r, l, s.Sessions.PendingUser(b.Ticket), p)
		return
	}
	s.signedIn(w, r, l, login, nil)
}

func (s *Server) secondStepSetup(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	var b struct{ Ticket string }
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	answer, p := s.ID.SecondStepStart(b.Ticket)
	if p != nil {
		writeProblem(w, p)
		return
	}
	writeJSON(w, http.StatusOK, answer)
}

func (s *Server) secondStepFinish(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	var b struct{ Ticket, Code string }
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	login, codes, p := s.ID.SecondStepFinish(b.Ticket, b.Code)
	if p != nil {
		s.refused(w, r, l, s.Sessions.PendingUser(b.Ticket), p)
		return
	}
	s.signedIn(w, r, l, login, map[string]any{"backup_codes": codes})
}

func (s *Server) signOut(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	s.Sessions.End(se)
	http.SetCookie(w, &http.Cookie{Name: s.cookieName(l), Value: "", Path: "/", MaxAge: -1,
		Secure: !s.Cfg.Insecure, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	for _, name := range []string{SessionCookie, TokenCookie} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1,
			Secure: !s.Cfg.Insecure, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// What a locked desktop may still ask.
var lockOpen = map[string]bool{"/api/unlock": true, "/api/sign-out": true, "/api/events": true}

// lock: the screen locked in this browser; it stays locked if the page is
// opened again, until the admin's password.
func (s *Server) lock(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	se.SetLocked(true)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) unlock(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	var b struct{ Password string }
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	if p := s.passwordAgain(se.Login(), &b.Password); p != nil {
		if p.Code == "not_your_password" || p.Code == "password_needed" {
			p.Code = "wrong_unlock"
		}
		writeProblem(w, p)
		return
	}
	se.SetLocked(false)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// recover: Can't sign in? The setup code from the sticker lets an admin back
// in, at the box's own screen only.
func (s *Server) recover(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	if l != ListenerScreen {
		writeProblem(w, problem(http.StatusForbidden, "at_the_box"))
		return
	}
	var b struct {
		SetupCode string `json:"setup_code"`
		Username  string `json:"username"`
		Password  string `json:"password"`
	}
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	fields := map[string]json.RawMessage{}
	for k, v := range map[string]string{"setup_code": b.SetupCode, "username": b.Username, "password": b.Password} {
		fields[k], _ = json.Marshal(v)
	}
	answer, p := s.Agent.Ask("box-screen", "admin-recover", fields)
	if p != nil {
		writeProblem(w, p)
		return
	}
	writeJSON(w, http.StatusOK, answer)
}

// --- The agent --------------------------------------------------------------

func (s *Server) agent(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	var b struct {
		Request  map[string]json.RawMessage `json:"request"`
		Password *string                    `json:"password"`
	}
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	var op string
	json.Unmarshal(b.Request["op"], &op)
	rule, ok := agentRules[op]
	if !ok || rule.SignedOut || (rule.Screen && l != ListenerScreen) {
		writeProblem(w, problem(http.StatusForbidden, "not_allowed"))
		return
	}
	login := se.Login()
	now := s.Now()
	if rule.needsPassword(b.Request) && !(op == "restart" && se.RestartAllowed(now)) {
		if p := s.passwordAgain(login, b.Password); p != nil {
			writeProblem(w, p)
			return
		}
	}
	answer, p := s.Agent.Ask(login.Member.Username, op, b.Request)
	if p != nil {
		writeProblem(w, p)
		return
	}
	// A restore or a reset is followed by a restart, not asked again.
	if op == "restore-point-restore" || op == "factory-reset" {
		se.AllowRestart(now)
	}
	writeJSON(w, http.StatusOK, answer)
}

// passwordAgain checks the admin's password with ZOOMRIYA ID, without
// signing in or out.
func (s *Server) passwordAgain(login *Login, password *string) *Problem {
	if password == nil || *password == "" {
		return problem(http.StatusForbidden, "password_needed")
	}
	ok, p := s.ID.PasswordIs(login, *password)
	if p != nil {
		return p
	}
	if !ok {
		return problem(http.StatusForbidden, "not_your_password")
	}
	return nil
}

// --- Each admin's settings, and the box's ---------------------------------

func (s *Server) prefsGet(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	writeJSON(w, http.StatusOK, s.Store.Load(se.Login().Member.ID))
}

func (s *Server) prefsPut(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	var b map[string]any
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	member := se.Login().Member.ID
	before := s.Store.Load(member)
	prefs, err := s.Store.Save(member, b)
	if err != nil {
		writeProblem(w, problem(http.StatusInternalServerError, "not_saved"))
		return
	}
	// Their keyboard and mouse change on the box's screen while they're
	// signed in there.
	if s.Sessions.AtScreen(member) && inputChanged(before, prefs) {
		go s.applyScreen(prefs)
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (s *Server) boxPatch(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	var b map[string]any
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	box, err := s.Store.SaveBox(b)
	if errors.Is(err, errNotAllowed) {
		writeProblem(w, problem(http.StatusBadRequest, "bad_request"))
		return
	} else if err != nil {
		writeProblem(w, problem(http.StatusInternalServerError, "not_saved"))
		return
	}
	writeJSON(w, http.StatusOK, box)
}

// boxPicture: the family's own background, shown before anyone signs in too.
func (s *Server) boxPicture(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	f, err := os.Open(s.Store.Picture())
	if err != nil {
		writeProblem(w, problem(http.StatusNotFound, "not_found"))
		return
	}
	defer f.Close()
	fi, _ := f.Stat()
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "box.jpg", fi.ModTime(), f)
}

// --- People (ZOOMRIYA ID's /users) ------------------------------------------

func (s *Server) people(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	answer, p := s.ID.Ask(se.Login(), "GET", "/users", nil)
	if p != nil {
		writeProblem(w, p)
		return
	}
	writeJSON(w, http.StatusOK, answer)
}

func (s *Server) activity(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	query := ""
	if before := r.URL.Query().Get("before"); before != "" {
		if _, err := strconv.Atoi(before); err != nil {
			writeProblem(w, problem(http.StatusBadRequest, "bad_request"))
			return
		}
		query = "?before=" + before
	}
	answer, p := s.ID.Ask(se.Login(), "GET", "/users/activity"+query, nil)
	if p != nil {
		writeProblem(w, p)
		return
	}
	writeJSON(w, http.StatusOK, answer)
}

func (s *Server) personAdd(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	var fields map[string]any
	if p := readJSON(r, &fields); p != nil {
		writeProblem(w, p)
		return
	}
	login := se.Login()
	answer, p := s.ID.Ask(login, "POST", "/users", fields)
	if p != nil {
		writeProblem(w, p)
		return
	}
	child := ""
	if _, ok := fields["birth_year"]; ok {
		child = ", a child"
	}
	s.Journal.Change(login.Member.Username, "people-add",
		"Added "+asString(fields["display_name"])+" (@"+asString(fields["username"])+")"+child)
	writeJSON(w, http.StatusOK, answer)
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func shownName(person map[string]any) string {
	if n := asString(person["display_name"]); n != "" {
		return n
	}
	return asString(person["username"])
}

// personChange: a change to someone's account. Making or removing an admin
// and turning an account off ask for the password again, and so does an
// admin changing their own password.
func (s *Server) personChange(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		writeProblem(w, problem(http.StatusBadRequest, "bad_request"))
		return
	}
	var b struct {
		Fields   map[string]any `json:"fields"`
		Password *string        `json:"password"`
	}
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	login := se.Login()
	_, role := b.Fields["role"]
	off := b.Fields["is_active"] == false
	_, newPassword := b.Fields["password"]
	if role || off || (newPassword && id == login.Member.ID) {
		if p := s.passwordAgain(login, b.Password); p != nil {
			writeProblem(w, p)
			return
		}
	}
	answer, p := s.ID.Ask(login, "PATCH", "/users/"+strconv.Itoa(id), b.Fields)
	if p != nil {
		writeProblem(w, p)
		return
	}
	person, _ := answer.(map[string]any)
	who := shownName(person)
	op, message := "people-change", "Changed "+who+"'s details (@"+asString(person["username"])+")"
	switch {
	case b.Fields["role"] == "admin":
		op, message = "people-role", "Made "+who+" an admin"
	case role:
		op, message = "people-role", "Removed "+who+" as admin"
	case off:
		op, message = "people-off", "Turned off "+who+"'s account"
	case b.Fields["is_active"] == true:
		op, message = "people-on", "Turned on "+who+"'s account"
	case newPassword:
		op, message = "people-password", "Set a new password for "+who
	}
	s.Journal.Change(login.Member.Username, op, message)
	// No longer an admin, or turned off: signed out of Box Desktop at once.
	if (role && b.Fields["role"] != "admin") || off {
		s.Sessions.EndMember(id)
	}
	writeJSON(w, http.StatusOK, answer)
}

// --- The page's event stream -------------------------------------------------

// events keeps the page told while it's open: a heartbeat, and the moment
// its sign-in ends. While a page is open, its sign-in isn't idle.
func (s *Server) events(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeProblem(w, problem(http.StatusInternalServerError, "no_stream"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	se.Stream(1)
	defer se.Stream(-1)
	io.WriteString(w, "retry: 3000\n\n")
	flusher.Flush()
	beat := time.NewTicker(20 * time.Second)
	defer beat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-se.Done:
			io.WriteString(w, "event: signed-out\ndata: {}\n\n")
			flusher.Flush()
			return
		case <-beat.C:
			io.WriteString(w, ": beat\n\n")
			flusher.Flush()
		}
	}
}

// --- The desktop's files -----------------------------------------------------

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	if fi, err := fs.Stat(s.Web, name); err != nil || fi.IsDir() {
		// The desktop is one page: anything else is its own.
		if path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		name = "index.html"
	}
	if strings.HasPrefix(name, "assets/") {
		// Vite names each file by its content.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeFileFS(w, r, s.Web, name)
}
