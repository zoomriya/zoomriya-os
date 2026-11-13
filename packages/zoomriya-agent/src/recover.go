// An admin let back in at the box (design §5.3): someone at the box's own
// screen who can't sign in (a forgotten password, or a lost phone and backup
// codes) types the box's setup code, an admin's username and a new password.
// Holding the box is what proves it's theirs: the code is on its sticker, or was
// written down at the first setup. ZOOMRIYA ID gives that admin the new
// password, clears their second step (they set up a new one at their next
// sign-in) and signs them out everywhere.
//
// Five wrong codes in an hour and it waits; every try is in the audit log, and
// the bell tells the box's admins either way.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	recoverTries  = "var/lib/zoomriya-agent/recover-tries.json"
	recoverMax    = 5
	recoverWindow = time.Hour
	idNoSuchAdmin = 3
)

// The wrong codes typed lately.
type triesState struct {
	Wrong []time.Time `json:"wrong"`
}

func (a *Agent) recentWrongCodes() triesState {
	var state triesState
	if data, err := os.ReadFile(a.path(recoverTries)); err == nil {
		_ = json.Unmarshal(data, &state)
	}
	since := a.Now().Add(-recoverWindow)
	kept := state.Wrong[:0]
	for _, at := range state.Wrong {
		if at.After(since) {
			kept = append(kept, at)
		}
	}
	state.Wrong = kept
	return state
}

func (a *Agent) saveTries(state triesState) {
	data, _ := json.Marshal(state)
	if err := os.MkdirAll(filepath.Dir(a.path(recoverTries)), 0o700); err == nil {
		_ = os.WriteFile(a.path(recoverTries), data, 0o600)
	}
}

func (a *Agent) adminRecover(code, username, password *string) (Response, error) {
	if !a.setupFinished() {
		return nil, refuse("Finish setting up the box first.")
	}
	tries := a.recentWrongCodes()
	if len(tries.Wrong) >= recoverMax {
		return nil, refuse("Too many wrong setup codes. Try again in an hour.")
	}
	if code == nil || !a.setupCodeMatches(*code) {
		tries.Wrong = append(tries.Wrong, a.Now())
		a.saveTries(tries)
		a.notify("signin", "warning", "Someone at the box typed a wrong setup code to let an admin back in.")
		return nil, refuse("That isn't the box's setup code. It's on the sticker, or the one written down at the first setup.")
	}
	if username == nil || password == nil {
		return nil, refuse("Which admin, and their new password?")
	}
	name := strings.TrimSpace(*username)
	if name == "" || utf8.RuneCountInString(name) > 128 || strings.ContainsAny(name, "\r\n") {
		return nil, refuse("The username must be 1 to 128 characters.")
	}
	if n := utf8.RuneCountInString(*password); n < minPassword || n > 256 || strings.ContainsAny(*password, "\r\n") {
		return nil, refuse("The password must be %d to 256 characters.", minPassword)
	}
	runInput := a.RunInput
	if runInput == nil {
		runInput = runWithInput
	}
	// As ZOOMRIYA ID's own account, as the first admin is made; the password on
	// standard input, never in the list of processes.
	_, err := runInput(*password, "systemd-run", "--quiet", "--pipe", "--wait", "--collect",
		"--property=User=zoomriya-id", "--property=EnvironmentFile=/etc/zoomriya/id.env",
		"--property=WorkingDirectory=/opt/zoomriya/id/backend",
		"--", idPython, "-m", "app.recover_admin", name)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == idNoSuchAdmin {
		return nil, refuse("There's no admin called %s on this box.", name)
	}
	if err != nil {
		return nil, err
	}
	a.saveTries(triesState{})
	a.notify("signin", "warning", fmt.Sprintf(
		"%s was let back in at the box with the setup code: a new password, and a new second sign-in step at their next sign-in. If that wasn't them, make sure the box is somewhere safe.", name))
	return Response{"username": name}, nil
}
