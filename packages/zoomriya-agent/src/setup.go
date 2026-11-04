// setup-status, region-set, time-zone-set, first-admin and setup-finish
// (design §5): the
// first setup, on the box's own screen. The setup app asks for them; once
// setup is finished, first-admin and setup-finish are refused.
package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// setupDoneFile marks the first setup finished; Erase everything takes
	// it away with the rest of the OS's changes.
	setupDoneFile = "var/lib/zoomriya-os/setup-done"
	zoneinfoDir   = "usr/share/zoneinfo"
	// The countries and regions people choose from (tzdata's list), and the
	// box's choice: its two-letter code.
	countriesFile = "usr/share/zoneinfo/iso3166.tab"
	regionFile    = "etc/zoomriya/region"
	// ZOOMRIYA ID's own command for its first account (zoomriya-id package).
	idPython = "/opt/zoomriya/id/venv/bin/python"
	// first_admin's exit status when ZOOMRIYA ID already has accounts.
	idAccountsExist = 3
	minPassword     = 8
)

var regionCode = regexp.MustCompile(`^[A-Z]{2}$`)

var zoneName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+-]*(/[A-Za-z0-9_+-]+){0,2}$`)

func (a *Agent) setupFinished() bool {
	_, err := os.Stat(a.path(setupDoneFile))
	return err == nil
}

// timeZone is the box's zone, from /etc/localtime's link into the database.
func (a *Agent) timeZone() string {
	target, err := os.Readlink(a.path("etc/localtime"))
	if err != nil {
		return "UTC"
	}
	if _, zone, found := strings.Cut(target, zoneinfoDir+"/"); found {
		return zone
	}
	return "UTC"
}

// region is the box's country or region, a code such as CA; empty until the
// first setup asks.
func (a *Agent) region() string {
	data, err := os.ReadFile(a.path(regionFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// knownRegion: the code is one of tzdata's countries and regions.
func (a *Agent) knownRegion(code string) bool {
	data, err := os.ReadFile(a.path(countriesFile))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if known, _, _ := strings.Cut(line, "\t"); known == code {
			return true
		}
	}
	return false
}

// hasAdmin: ZOOMRIYA ID has an active admin (one made before a power cut
// counts: the setup goes on from there).
func (a *Agent) hasAdmin() bool {
	out, err := a.Run("runuser", "-u", "postgres", "--", "psql", "-qtA", "zoomriya_id",
		"-c", "SELECT count(*) FROM users WHERE role = 'admin' AND is_active")
	count := strings.TrimSpace(string(out))
	return err == nil && count != "" && count != "0"
}

func (a *Agent) setupStatus() (Response, error) {
	synced := false
	if out, err := a.Run("timedatectl", "show", "--property=NTPSynchronized", "--value"); err == nil {
		synced = strings.TrimSpace(string(out)) == "yes"
	}
	return Response{
		"finished": a.setupFinished(), "has_admin": a.hasAdmin(),
		"language": a.language(), "region": a.region(), "zone": a.timeZone(), "clock_synced": synced,
	}, nil
}

func (a *Agent) regionSet(code *string) (Response, error) {
	if code == nil || !regionCode.MatchString(*code) || !a.knownRegion(*code) {
		return nil, refuse("Which country or region? Choose one from the list.")
	}
	file := a.path(regionFile)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(file, []byte(*code+"\n"), 0o644); err != nil {
		return nil, err
	}
	return Response{"region": *code}, nil
}

func (a *Agent) timeZoneSet(zone *string) (Response, error) {
	if zone == nil || !zoneName.MatchString(*zone) {
		return nil, refuse("Which time zone? Choose one from the list.")
	}
	if info, err := os.Stat(a.path(filepath.Join(zoneinfoDir, *zone))); err != nil || info.IsDir() {
		return nil, refuse("The box doesn't know a time zone called %s.", *zone)
	}
	if _, err := a.Run("timedatectl", "set-timezone", *zone); err != nil {
		return nil, err
	}
	return Response{"zone": *zone}, nil
}

func (a *Agent) firstAdmin(username, password *string) (Response, error) {
	if a.setupFinished() {
		return nil, refuse("The box is already set up.")
	}
	if username == nil || password == nil {
		return nil, refuse("The admin needs a username and a password.")
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
	// As ZOOMRIYA ID's own account, with its settings; the password goes on
	// standard input, so it never shows in the list of processes.
	_, err := runInput(*password, "systemd-run", "--quiet", "--pipe", "--wait", "--collect",
		"--property=User=zoomriya-id", "--property=EnvironmentFile=/etc/zoomriya/id.env",
		"--property=WorkingDirectory=/opt/zoomriya/id/backend",
		"--", idPython, "-m", "app.first_admin", name)
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == idAccountsExist {
		return nil, refuse("ZOOMRIYA ID already has an admin on this box.")
	}
	if err != nil {
		return nil, err
	}
	return Response{"username": name}, nil
}

func (a *Agent) setupFinish() (Response, error) {
	if a.setupFinished() {
		return nil, refuse("The box is already set up.")
	}
	if !a.hasAdmin() {
		return nil, refuse("Make the box's admin first.")
	}
	file := a.path(setupDoneFile)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		return nil, err
	}
	return Response{"finished": true}, nil
}

func runWithInput(input, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(input)
	return cmd.Output()
}
