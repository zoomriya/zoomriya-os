// setup-status, region-set, time-zone-set, first-admin and setup-finish
// (design §5): the
// first setup, on the box's own screen. The setup app asks for them; once
// setup is finished, first-admin and setup-finish are refused.
package main

import (
	"encoding/json"
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
	// The state or province within it, when one was chosen (optional): its
	// ISO 3166-2 code, such as CA-ON, from iso-codes' list.
	subdivisionFile  = "etc/zoomriya/subdivision"
	subdivisionsFile = "usr/share/iso-codes/json/iso_3166-2.json"
	// ZOOMRIYA ID's own command for its first account (zoomriya-id package).
	idPython = "/opt/zoomriya/id/venv/bin/python"
	// first_admin's exit status when ZOOMRIYA ID already has accounts.
	idAccountsExist = 3
	minPassword     = 8
)

var regionCode = regexp.MustCompile(`^[A-Z]{2}$`)

var subdivisionCode = regexp.MustCompile(`^[A-Z]{2}-[A-Z0-9]{1,3}$`)

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

// subdivision is the box's state or province, such as CA-ON; empty when none
// was chosen.
func (a *Agent) subdivision() string {
	data, err := os.ReadFile(a.path(subdivisionFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// knownSubdivision: the code is one of iso-codes' top-level subdivisions (no
// parent: Canada's provinces, France's regions, not its departments).
func (a *Agent) knownSubdivision(code string) bool {
	data, err := os.ReadFile(a.path(subdivisionsFile))
	if err != nil {
		return false
	}
	var list map[string][]struct {
		Code   string `json:"code"`
		Parent string `json:"parent"`
	}
	if json.Unmarshal(data, &list) != nil {
		return false
	}
	for _, s := range list["3166-2"] {
		if s.Code == code && s.Parent == "" {
			return true
		}
	}
	return false
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
		"language": a.language(), "region": a.region(), "subdivision": a.subdivision(),
		"zone": a.timeZone(), "clock_synced": synced,
	}, nil
}

// regionSet sets the country or region, and its state or province when one is
// given; a new country without one clears the old one's.
func (a *Agent) regionSet(code, sub *string) (Response, error) {
	if code == nil || !regionCode.MatchString(*code) || !a.knownRegion(*code) {
		return nil, refuse("Which country or region? Choose one from the list.")
	}
	chosen := ""
	if sub != nil && *sub != "" {
		if !subdivisionCode.MatchString(*sub) || !strings.HasPrefix(*sub, *code+"-") || !a.knownSubdivision(*sub) {
			return nil, refuse("Which state or province? Choose one from the list.")
		}
		chosen = *sub
	}
	file := a.path(regionFile)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(file, []byte(*code+"\n"), 0o644); err != nil {
		return nil, err
	}
	if chosen == "" {
		if err := os.Remove(a.path(subdivisionFile)); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	} else if err := os.WriteFile(a.path(subdivisionFile), []byte(chosen+"\n"), 0o644); err != nil {
		return nil, err
	}
	return Response{"region": *code, "subdivision": chosen}, nil
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
