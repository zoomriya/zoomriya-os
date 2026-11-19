// ZOOMRIYA Connect in Box Desktop: the tool's requests go on to connectd's
// own API, on its socket (/run/zoomriya-connect/api.sock), which only
// box-desktopd's group may open. Only the routes the tool uses pass, and
// only for a signed-in admin, as everything in Box Desktop. Each change goes
// in the journal's admin actions.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// connectRoutes: what the tool may ask connectd, as "METHOD path".
var connectRoutes = []*regexp.Regexp{
	regexp.MustCompile(`^GET /v1/status$`),
	regexp.MustCompile(`^GET /v1/devices$`),
	regexp.MustCompile(`^POST /v1/(setup|on|off|release|pairing|server|relay)$`),
	regexp.MustCompile(`^DELETE /v1/pairing$`),
	regexp.MustCompile(`^DELETE /v1/devices/[0-9a-f]{1,32}$`),
	regexp.MustCompile(`^POST /v1/devices/[0-9a-f]{1,32}/name$`),
}

// connectRisky: changes that ask for the admin's password again, as Box
// Desktop's other risky changes do, with removing a device. The password is checked here and never
// passed on to connectd.
var connectRisky = map[string]bool{
	"POST /v1/release": true,
	"POST /v1/server":  true,
	"POST /v1/off":     true,
}

// What each change is called in the journal's admin actions.
func connectAction(method, path string, body []byte) (op, message string) {
	var b map[string]any
	_ = json.Unmarshal(body, &b)
	str := func(k string) string { s, _ := b[k].(string); return clean(s) }
	switch {
	case path == "/v1/setup":
		return "connect-name", "Named the box " + str("name") + " for ZOOMRIYA Connect"
	case path == "/v1/on":
		return "connect-on", "Turned ZOOMRIYA Connect on"
	case path == "/v1/off":
		return "connect-off", "Turned ZOOMRIYA Connect off"
	case path == "/v1/release":
		return "connect-release", "Gave up the box's ZOOMRIYA Connect name"
	case path == "/v1/pairing" && method == "POST":
		if b["admin"] == true {
			return "connect-pair", "Showed a pairing code for an admin's phone"
		}
		return "connect-pair", "Showed a pairing code for a phone"
	case path == "/v1/server":
		if u := str("url"); u != "" {
			return "connect-server", "Switched ZOOMRIYA Connect to the server " + u
		}
		return "connect-server", "Switched ZOOMRIYA Connect back to ZOOMRIYA's server"
	case path == "/v1/relay":
		if b["off"] == true {
			return "connect-relay", "Turned ZOOMRIYA Connect's relay off"
		}
		return "connect-relay", "Turned ZOOMRIYA Connect's relay on"
	case method == "DELETE" && strings.HasPrefix(path, "/v1/devices/"):
		return "connect-remove", "Removed a paired device"
	case strings.HasSuffix(path, "/name"):
		return "connect-rename", "Renamed a paired device to " + str("name")
	}
	return "", ""
}

// connect passes /api/connect/... on to connectd.
func (s *Server) connect(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	path := "/" + strings.TrimPrefix(r.PathValue("path"), "/")
	allowed := false
	for _, re := range connectRoutes {
		if re.MatchString(r.Method + " " + path) {
			allowed = true
			break
		}
	}
	if !allowed {
		writeProblem(w, problem(http.StatusNotFound, "not_found"))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil {
		writeProblem(w, problem(http.StatusBadRequest, "bad_request"))
		return
	}
	if connectRisky[r.Method+" "+path] || (r.Method == "DELETE" && strings.HasPrefix(path, "/v1/devices/")) {
		var fields map[string]any
		_ = json.Unmarshal(body, &fields)
		password, _ := fields["password"].(string)
		if p := s.passwordAgain(se.Login(), &password); p != nil {
			writeProblem(w, p)
			return
		}
		delete(fields, "password")
		if fields == nil {
			fields = map[string]any{}
		}
		body, _ = json.Marshal(fields)
	}
	client := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", s.Cfg.Connect)
			},
		},
	}
	req, _ := http.NewRequestWithContext(r.Context(), r.Method, "http://connectd"+path, bytes.NewReader(body))
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := client.Do(req)
	if err != nil {
		// connectd isn't running (not installed, or starting).
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "connect_unavailable"})
		return
	}
	defer res.Body.Close()
	answer, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if r.Method != "GET" && res.StatusCode < 300 {
		if op, message := connectAction(r.Method, path, body); op != "" {
			s.Journal.Change(se.Login().Member.Username, op, message)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(res.StatusCode)
	w.Write(answer)
}
