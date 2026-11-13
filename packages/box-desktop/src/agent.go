// Asking zoomriya-agent (design §7): one JSON line out, one back, over its
// socket. Box Desktop passes on only the requests on its own list, in the
// signed-in admin's name; the agent checks each one's fields itself and
// refuses any it doesn't know.
package main

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"
	"time"
)

// Rule says what Box Desktop lets through for one request.
type Rule struct {
	// Password: the admin's password again first (box-desktop.md §7).
	// PasswordActions: only for these actions (advanced-access's "on").
	Password        bool
	PasswordActions []string
	// Screen: from the box's own screen only.
	Screen bool
	// SignedOut: before anyone has signed in (Can't sign in?).
	SignedOut bool
}

// agentRules is every request the native desktop makes, and how.
var agentRules = map[string]Rule{
	"status":                {},
	"restore-point-list":    {},
	"restore-point-restore": {Password: true},
	"restart":               {Password: true},
	"shutdown":              {Password: true},
	"maintenance-hour":      {},
	"disk-health":           {},
	"factory-reset":         {Password: true},
	"network":               {},
	"network-set":           {},
	"network-keep":          {},
	"resources":             {},
	"logs":                  {},
	"security":              {},
	"security-check-now":    {},
	"outbound":              {},
	"notifications":         {},
	"keyboard":              {},
	"keyboard-set":          {},
	"storage":               {},
	"storage-check":         {},
	"setup-status":          {},
	"language-set":          {},
	"region-set":            {},
	"time-zone-set":         {},
	"display-set":           {},
	"internet":              {},
	"internet-set":          {},
	"held":                  {},
	"held-release":          {Password: true},
	"diagnostics":           {},
	"advanced-access":       {PasswordActions: []string{"on", "key-add"}},
	"apps":                  {PasswordActions: []string{"restore", "remove", "install"}},
	"pictures":              {},
	"backups":               {},
	// Can't sign in?: the setup code from the sticker lets an admin back in.
	// At the box only: over the home network, anyone who has seen the code
	// could use it.
	"admin-recover": {Screen: true, SignedOut: true},
}

// needsPassword: does this request, with these fields, ask for the password again?
func (r Rule) needsPassword(fields map[string]json.RawMessage) bool {
	if r.Password {
		return true
	}
	if len(r.PasswordActions) == 0 {
		return false
	}
	var action string
	json.Unmarshal(fields["action"], &action)
	for _, a := range r.PasswordActions {
		if a == action {
			return true
		}
	}
	return false
}

// Fields the page may never set: who asked is the server's to say, and the
// first setup's own.
var agentFixedFields = map[string]bool{"op": true, "by": true}

type Agent struct {
	Socket string
}

// Ask sends one request and gives the agent's answer. A refusal is a Problem
// carrying the agent's own sentence.
func (a *Agent) Ask(by, op string, fields map[string]json.RawMessage) (map[string]any, *Problem) {
	request := map[string]json.RawMessage{}
	for k, v := range fields {
		if !agentFixedFields[k] {
			request[k] = v
		}
	}
	request["op"], _ = json.Marshal(op)
	request["by"], _ = json.Marshal(by)
	line, err := json.Marshal(request)
	if err != nil {
		return nil, problem(http.StatusBadRequest, "bad_request")
	}
	conn, err := net.DialTimeout("unix", a.Socket, 5*time.Second)
	if err != nil {
		return nil, problem(http.StatusBadGateway, "box_down")
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(120 * time.Second))
	if _, err := conn.Write(append(line, '\n')); err != nil {
		return nil, problem(http.StatusBadGateway, "box_down")
	}
	reader := bufio.NewReaderSize(conn, 1<<16)
	raw, err := reader.ReadBytes('\n')
	if err != nil && len(raw) == 0 {
		return nil, problem(http.StatusBadGateway, "box_down")
	}
	var answer map[string]any
	if json.Unmarshal(raw, &answer) != nil {
		return nil, problem(http.StatusBadGateway, "box_unreadable")
	}
	if ok, _ := answer["ok"].(bool); !ok {
		p := problem(http.StatusConflict, "box_refused")
		p.Error, _ = answer["error"].(string)
		return nil, p
	}
	return answer, nil
}
