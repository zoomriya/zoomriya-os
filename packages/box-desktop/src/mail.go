// Sign-in codes by email (ZOOMRIYA ID's app.mail and app.second_step):
// the family's own email account that sends the codes, and the signed-in
// admin's own codes. Both are ZOOMRIYA ID's; Box Desktop passes them on as
// this admin, as People does.
package main

import (
	"net/http"
	"strings"
)

// mailAsk: /api/mail and /api/mail/test, on to ZOOMRIYA ID's /mail.
func (s *Server) mailAsk(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	path := "/mail" + strings.TrimPrefix(r.URL.Path, "/api/mail")
	var body any
	if r.Method == http.MethodPut || r.Method == http.MethodPost {
		var b map[string]any
		if p := readJSON(r, &b); p != nil {
			writeProblem(w, p)
			return
		}
		body = b
	}
	answer, p := s.ID.Ask(se.Login(), r.Method, path, body)
	if p != nil {
		writeProblem(w, p)
		return
	}
	writeJSON(w, http.StatusOK, answer)
}

// mySecondStep: whether the admin's own codes are on, and where they go.
func (s *Server) mySecondStep(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	answer, p := s.ID.Ask(se.Login(), "GET", "/auth/me", nil)
	if p != nil {
		writeProblem(w, p)
		return
	}
	me, _ := answer.(map[string]any)
	writeJSON(w, http.StatusOK, me["second_step"])
}

// mySecondStepChange: setup (password and address; a code goes there),
// finish (that code) or off (password).
func (s *Server) mySecondStepChange(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	paths := map[string]string{
		"setup":  "/auth/second-step/setup",
		"finish": "/auth/second-step/setup/finish",
		"off":    "/auth/me/second-step/off",
	}
	path, ok := paths[r.PathValue("step")]
	if !ok {
		writeProblem(w, problem(http.StatusNotFound, "not_found"))
		return
	}
	var b map[string]any
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	answer, p := s.ID.Ask(se.Login(), "POST", path, b)
	if p != nil {
		writeProblem(w, p)
		return
	}
	writeJSON(w, http.StatusOK, answer)
}
