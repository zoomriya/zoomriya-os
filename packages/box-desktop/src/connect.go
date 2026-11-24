// Control Panel › Devices and Remote access: the tools' requests go on to
// connectd's own API, on its socket (/run/zoomriya-connect/api.sock), which
// only box-desktopd's group may open. Only the routes the tools use pass,
// and only for a signed-in admin, as everything in Box Desktop. Removing a
// device and changing the family's own name ask for the admin's password
// again. Each change goes in the journal's admin actions. No ZOOMRIYA server
// is involved (zoomriya-connect, docs/direct-plus.md §6, §8 A, §11).
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
	regexp.MustCompile(`^POST /v1/pairing$`),
	regexp.MustCompile(`^DELETE /v1/pairing$`),
	regexp.MustCompile(`^DELETE /v1/devices/[0-9a-f]{1,32}$`),
	regexp.MustCompile(`^POST /v1/devices/[0-9a-f]{1,32}/name$`),
	regexp.MustCompile(`^PUT /v1/away$`),
	regexp.MustCompile(`^(GET|PUT|DELETE) /v1/names$`),
	regexp.MustCompile(`^GET /v1/guests$`),
}

// What each change is called in the journal's admin actions.
func connectAction(method, path string, body []byte) (op, message string) {
	var b map[string]any
	_ = json.Unmarshal(body, &b)
	str := func(k string) string { s, _ := b[k].(string); return clean(s) }
	switch {
	case path == "/v1/pairing" && method == "POST":
		if b["laptop"] == true {
			return "device-pair", "Showed a pairing code for a laptop"
		}
		return "device-pair", "Showed a pairing code for a phone"
	case method == "DELETE" && strings.HasPrefix(path, "/v1/devices/"):
		return "device-remove", "Removed a paired device"
	case path == "/v1/names" && method == "PUT":
		return "own-name", "Set the family's own name to " + str("domain")
	case path == "/v1/names" && method == "DELETE":
		return "own-name", "Removed the family's own name"
	case path == "/v1/away":
		if b["on"] == true {
			return "remote-access", "Turned on remote access away from home"
		}
		return "remote-access", "Turned off remote access away from home"
	case strings.HasSuffix(path, "/name"):
		return "device-rename", "Renamed a paired device to " + str("name")
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
	// Removing a device, and the family's DNS key: the password again.
	if (r.Method == "DELETE" && strings.HasPrefix(path, "/v1/devices/")) ||
		(path == "/v1/names" && r.Method != "GET") {
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
		// connectd isn't running (starting, or stopped).
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
