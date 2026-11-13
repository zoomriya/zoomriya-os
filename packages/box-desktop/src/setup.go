// The first setup (design §5), on the box's own screen: the same page, before
// the box is set up, as a wizard. Being in front of the box is what proves
// it's yours, so it's served on the screen's side only (127.0.0.1), with no
// sign-in, and only until the agent's setup-finish marks the box set up.
// After that, every request here is refused.
package main

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// setupRequests: what the setup asks the agent, each with its fields as the
// page sends them; "by" is always first-setup.
var setupRequests = map[string]bool{
	"setup-status": true, "keyboard": true, "keyboard-set": true, "network": true, "network-set": true,
	"network-keep": true, "region-set": true, "language-set": true, "time-zone-set": true,
	"first-admin": true, "setup-finish": true,
}

// internetCheck is reached to say whether the box is online; the setup goes
// on without it.
var internetCheck = "deb.debian.org:443"

// setupProgress: steps done, so a setup cut short (a power cut) goes on from
// where it was.
type setupProgress struct {
	mu   sync.Mutex
	path string
}

func (p *setupProgress) load() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	found := map[string]any{}
	if raw, err := os.ReadFile(p.path); err == nil {
		json.Unmarshal(raw, &found)
	}
	return found
}

func (p *setupProgress) save(changes map[string]any) map[string]any {
	found := p.load()
	p.mu.Lock()
	defer p.mu.Unlock()
	for k, v := range changes {
		if k == "language" || k == "keyboard" || k == "zone" {
			found[k] = v
		}
	}
	raw, _ := json.Marshal(found)
	os.MkdirAll(filepath.Dir(p.path), 0o700)
	os.WriteFile(p.path, raw, 0o600)
	return found
}

// SetUp: the box is set up (the agent's setup-finish wrote the mark).
func (s *Server) SetUp() bool {
	_, err := os.Stat(s.Cfg.SetupDone)
	return err == nil
}

// inSetup guards every setup request: the box's own screen, before it's set up.
func (s *Server) inSetup(w http.ResponseWriter, l Listener) bool {
	if l != ListenerScreen {
		writeProblem(w, problem(http.StatusForbidden, "at_the_box"))
		return false
	}
	if s.SetUp() {
		writeProblem(w, problem(http.StatusConflict, "set_up"))
		return false
	}
	return true
}

func (s *Server) setupState(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	if !s.inSetup(w, l) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"progress": s.Progress.load()})
}

func (s *Server) setupProgressPost(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	if !s.inSetup(w, l) {
		return
	}
	var b map[string]any
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	writeJSON(w, http.StatusOK, s.Progress.save(b))
}

// setupAgent: one of the setup's requests to the agent, as first-setup.
func (s *Server) setupAgent(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	if !s.inSetup(w, l) {
		return
	}
	var b struct {
		Request map[string]json.RawMessage `json:"request"`
	}
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	var op string
	json.Unmarshal(b.Request["op"], &op)
	if !setupRequests[op] {
		writeProblem(w, problem(http.StatusForbidden, "not_allowed"))
		return
	}
	answer, p := s.Agent.Ask("first-setup", op, b.Request)
	if p != nil {
		writeProblem(w, p)
		return
	}
	// The keyboard on the screen at once, so the admin's password is typed with it.
	if op == "keyboard-set" {
		var layout string
		json.Unmarshal(b.Request["layout"], &layout)
		if layout != "" {
			go s.Hardware.Keyboard(layout)
		}
	}
	writeJSON(w, http.StatusOK, answer)
}

// setupInternet: whether the box reaches the internet.
func (s *Server) setupInternet(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	if !s.inSetup(w, l) {
		return
	}
	conn, err := net.DialTimeout("tcp", internetCheck, 5*time.Second)
	if err == nil {
		conn.Close()
	}
	writeJSON(w, http.StatusOK, map[string]bool{"online": err == nil})
}

// setupSecondStepStart: the admin just made sets up the code from their phone
// (ZOOMRIYA ID's decision 27), with the password they just chose: its QR code.
func (s *Server) setupSecondStepStart(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	if !s.inSetup(w, l) {
		return
	}
	var b struct{ Username, Password string }
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	a, err := s.ID.open("POST", "/auth/login", map[string]string{"username": strings.TrimSpace(b.Username), "password": b.Password}, "", "")
	if err != nil {
		writeProblem(w, problem(http.StatusBadGateway, "id_down_later"))
		return
	}
	if a.Status == 403 && str(a.Body, "code") == "second_step_needed" {
		// Set up already (a setup that went on after a power cut).
		writeJSON(w, http.StatusOK, map[string]any{"already": true})
		return
	}
	if a.Status != 403 || str(a.Body, "code") != "second_step_setup_needed" {
		writeProblem(w, problem(http.StatusBadGateway, "id_start_failed"))
		return
	}
	ticket := str(a.Body, "ticket")
	setup, err := s.ID.open("POST", "/auth/second-step/setup", map[string]string{"ticket": ticket}, "", "")
	if err != nil || setup.Status != 200 {
		writeProblem(w, problem(http.StatusBadGateway, "id_start_failed"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ticket": ticket, "qr_svg": str(setup.Body, "qr_svg"), "key": str(setup.Body, "key")})
}

// setupSecondStepFinish: the app's first code; it's on. The backup codes, shown once.
func (s *Server) setupSecondStepFinish(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	if !s.inSetup(w, l) {
		return
	}
	var b struct{ Ticket, Code string }
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	a, err := s.ID.open("POST", "/auth/second-step/setup/finish", map[string]string{"ticket": b.Ticket, "code": strings.TrimSpace(b.Code)}, "", "")
	switch {
	case err != nil:
		writeProblem(w, problem(http.StatusBadGateway, "id_down_later"))
	case a.Status == 400:
		writeProblem(w, problem(400, "setup_wrong_code"))
	case a.Status == 401:
		writeProblem(w, problem(401, "setup_too_slow"))
	case a.Status != 200:
		writeProblem(w, problem(http.StatusBadGateway, "id_finish_failed"))
	default:
		writeJSON(w, http.StatusOK, map[string]any{"backup_codes": a.Body["backup_codes"]})
	}
}
