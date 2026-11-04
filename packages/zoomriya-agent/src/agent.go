// The requests the agent accepts (design §7). Each takes fixed arguments and
// never a command line; everything it runs is a fixed program with arguments
// it builds itself.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Request is one line from Box Desktop or the first setup. By is the ZOOMRIYA
// ID member who asked, for the audit log; Box Desktop has already signed them
// in as an admin. Unknown fields are refused.
type Request struct {
	Op     string  `json:"op"`
	By     string  `json:"by"`
	Number *int    `json:"number,omitempty"`
	Hour   *string `json:"hour,omitempty"`
	// factory-reset: "system" or "everything", and the setup code for the
	// latter.
	Level     *string `json:"level,omitempty"`
	SetupCode *string `json:"setup_code,omitempty"`
	// storage-check: "start", "pause" or "resume".
	Action *string `json:"action,omitempty"`
	// logs: "events", "audit" or "system"; the page before Cursor; for the
	// system log, the level ("errors", "warnings", "everything") and a search.
	Kind     *string `json:"kind,omitempty"`
	Cursor   *string `json:"cursor,omitempty"`
	Priority *string `json:"priority,omitempty"`
	Search   *string `json:"search,omitempty"`
	// network-set: the Ethernet port, "dhcp" or "fixed", and the fixed
	// settings.
	Link    *string  `json:"link,omitempty"`
	Mode    *string  `json:"mode,omitempty"`
	Address *string  `json:"address,omitempty"`
	Gateway *string  `json:"gateway,omitempty"`
	DNS     []string `json:"dns,omitempty"`
	// keyboard-set: one of the layouts keyboard lists.
	Layout *string `json:"layout,omitempty"`
	// language-set: one of Debian's locales, such as ta_LK or en_CA.UTF-8.
	Locale *string `json:"locale,omitempty"`
	// region-set: the box's country or region, a two-letter code such as CA.
	Region *string `json:"region,omitempty"`
	// time-zone-set: a zone from the time zone database, such as America/Toronto.
	Zone *string `json:"zone,omitempty"`
	// display-set: the box's screen (sway's output name), its resolution
	// ("1920x1080"), refresh rate in mHz, scale and transform ("normal", "90",
	// "180", "270"); or Action "forget", for the monitor's own best.
	Output    *string  `json:"output,omitempty"`
	Size      *string  `json:"size,omitempty"`
	Refresh   *int     `json:"refresh,omitempty"`
	Scale     *float64 `json:"scale,omitempty"`
	Transform *string  `json:"transform,omitempty"`
	// first-admin: the box's first ZOOMRIYA ID account. The password is
	// passed on to ZOOMRIYA ID and never logged.
	Username *string `json:"username,omitempty"`
	Password *string `json:"password,omitempty"`
}

// Response is one line back: ok, and either the request's result or an error
// meant for an admin to read.
type Response map[string]any

// Agent answers requests. Root is "/" on the box; tests point it at a folder
// of fake files and replace Run.
type Agent struct {
	Root string
	Run  func(name string, args ...string) ([]byte, error)
	Now  func() time.Time
	// Sleep waits between two readings (resources); tests replace it.
	Sleep func(time.Duration)
	// RunInput runs a command with input on its standard input (first-admin's
	// password); tests replace it.
	RunInput func(input, name string, args ...string) ([]byte, error)
}

const (
	restoreSnapshot = "/usr/lib/zoomriya-base/restore-snapshot"
	// restorePending tells status a restore waits for a restart.
	restorePending = "run/zoomriya/restore-pending"
	// resetPending tells status which reset waits for a restart.
	resetPending = "run/zoomriya/reset-pending"
	// How many restore points status shows.
	statusRestorePoints = 5
)

// readOnly requests change nothing; the audit log keeps them at debug level,
// so the changes stand out (Box Desktop asks for some every few seconds).
var readOnly = map[string]bool{
	"status": true, "restore-point-list": true, "disk-health": true,
	"storage": true, "network": true, "resources": true, "logs": true,
	"security": true, "notifications": true, "keyboard": true,
	"setup-status": true,
}

func NewAgent() *Agent {
	return &Agent{Root: "/", Run: run, Now: time.Now}
}

func run(name string, args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// errRequest is a refusal the admin can act on, as opposed to a failure.
type errRequest struct{ msg string }

func (e errRequest) Error() string { return e.msg }

func refuse(format string, args ...any) error {
	return errRequest{fmt.Sprintf(format, args...)}
}

// Handle answers one request.
func (a *Agent) Handle(req Request) (Response, error) {
	switch req.Op {
	case "status":
		return a.status()
	case "restore-point-list":
		points, err := a.restorePoints()
		if err != nil {
			return nil, err
		}
		return Response{"restore_points": points}, nil
	case "restore-point-restore":
		return a.restore(req.Number)
	case "restart":
		return a.restart()
	case "shutdown":
		return a.shutdown()
	case "maintenance-hour":
		return a.setMaintenanceHour(req.Hour)
	case "disk-health":
		return a.diskHealth()
	case "factory-reset":
		return a.factoryReset(req.Level, req.SetupCode)
	case "network":
		return a.network()
	case "resources":
		return a.resources()
	case "logs":
		return a.logs(req.Kind, req.Cursor, req.Search, req.Priority)
	case "network-set":
		return a.networkSet(req.Link, req.Mode, req.Address, req.Gateway, req.DNS)
	case "network-keep":
		return a.networkKeep()
	case "security":
		return a.security()
	case "notifications":
		return a.notifications()
	case "keyboard":
		return a.keyboard()
	case "keyboard-set":
		return a.keyboardSet(req.Layout)
	case "storage":
		return a.storage()
	case "storage-check":
		return a.storageCheck(req.Action)
	case "setup-status":
		return a.setupStatus()
	case "language-set":
		return a.languageSet(req.Locale)
	case "region-set":
		return a.regionSet(req.Region)
	case "time-zone-set":
		return a.timeZoneSet(req.Zone)
	case "display-set":
		return a.displaySet(req)
	case "first-admin":
		return a.firstAdmin(req.Username, req.Password)
	case "setup-finish":
		return a.setupFinish()
	case "":
		return nil, refuse("No request given.")
	default:
		return nil, refuse("Unknown request: %q.", req.Op)
	}
}

func (a *Agent) path(rel string) string {
	return filepath.Join(a.Root, rel)
}

func (a *Agent) readTrimmed(rel string) string {
	data, err := os.ReadFile(a.path(rel))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// modified is a file's change time, or nil when it doesn't exist.
func (a *Agent) modified(rel string) any {
	info, err := os.Stat(a.path(rel))
	if err != nil {
		return nil
	}
	return info.ModTime().UTC().Format(time.RFC3339)
}

// osRelease reads os-release's KEY=value lines, unquoting the values.
func (a *Agent) osRelease() map[string]string {
	fields := map[string]string{}
	for _, line := range strings.Split(a.readTrimmed("usr/lib/os-release"), "\n") {
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		fields[key] = value
	}
	return fields
}

func (a *Agent) status() (Response, error) {
	release := a.osRelease()

	base, err := a.Run("dpkg-query", "-W", "-f=${Version}", "zoomriya-base")
	if err != nil {
		base = nil
	}

	var uptime float64
	if fields := strings.Fields(a.readTrimmed("proc/uptime")); len(fields) > 0 {
		uptime, _ = strconv.ParseFloat(fields[0], 64)
	}

	reasons := a.restartReasons()
	var planned any
	if len(reasons) > 0 {
		planned = a.plannedRestart().Format(time.RFC3339)
	}

	points, err := a.restorePoints()
	if err != nil {
		points = []RestorePoint{}
	}
	if len(points) > statusRestorePoints {
		points = points[:statusRestorePoints]
	}

	return Response{
		"os":                   release["PRETTY_NAME"],
		"image_version":        release["IMAGE_VERSION"],
		"debian":               a.readTrimmed("etc/debian_version"),
		"base":                 strings.TrimSpace(string(base)),
		"kernel":               a.readTrimmed("proc/sys/kernel/osrelease"),
		"model":                a.model(),
		"serial":               a.serial(),
		"uptime_seconds":       int64(uptime),
		"restart_needed":       len(reasons) > 0,
		"restart_reasons":      reasons,
		"restart_planned":      planned,
		"maintenance_hour":     a.maintenanceHour(),
		"last_security_update": a.modified("var/log/unattended-upgrades/unattended-upgrades-dpkg.log"),
		"last_update_check":    a.modified("var/lib/apt/periodic/unattended-upgrades-stamp"),
		"restore_points":       points,
	}, nil
}

// RestorePoint is one snapper snapshot of @, newest first in lists. Kind tells
// Box Desktop how to word it.
type RestorePoint struct {
	Number      int    `json:"number"`
	Date        string `json:"date"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
	Important   bool   `json:"important"`
}

// snapperList is the part of `snapper --jsonout list` the agent reads.
type snapperList struct {
	Root []struct {
		Number      int               `json:"number"`
		Type        string            `json:"type"`
		Date        string            `json:"date"`
		Description string            `json:"description"`
		Userdata    map[string]string `json:"userdata"`
	} `json:"root"`
}

func (a *Agent) restorePoints() ([]RestorePoint, error) {
	out, err := a.Run("snapper", "--jsonout", "-c", "root", "list", "--disable-used-space")
	if err != nil {
		return nil, err
	}
	var list snapperList
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("snapper's list: %w", err)
	}

	points := []RestorePoint{}
	for _, s := range list.Root {
		if s.Number == 0 { // the running OS, not a restore point
			continue
		}
		kind := "other"
		switch {
		case s.Description == "factory":
			kind = "factory"
		case s.Description == "apt" && s.Type == "pre":
			kind = "before-update"
		case s.Description == "apt" && s.Type == "post":
			kind = "after-update"
		case strings.HasPrefix(s.Description, "Before restoring "):
			kind = "before-restore"
		}
		points = append(points, RestorePoint{
			Number:      s.Number,
			Date:        s.Date,
			Description: s.Description,
			Kind:        kind,
			Important:   s.Userdata["important"] == "yes",
		})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Number > points[j].Number })
	return points, nil
}

func (a *Agent) restore(number *int) (Response, error) {
	if number == nil {
		return nil, refuse("Which restore point? None was given.")
	}
	points, err := a.restorePoints()
	if err != nil {
		return nil, err
	}
	found := false
	for _, p := range points {
		found = found || p.Number == *number
	}
	if !found {
		return nil, refuse("There's no restore point %d.", *number)
	}

	if _, err := a.Run(restoreSnapshot, strconv.Itoa(*number)); err != nil {
		return nil, err
	}
	pending := a.path(restorePending)
	if err := os.MkdirAll(filepath.Dir(pending), 0o755); err == nil {
		_ = os.WriteFile(pending, []byte(strconv.Itoa(*number)+"\n"), 0o644)
	}
	return Response{"restored": *number, "restart_needed": true}, nil
}

// answer turns Handle's result into the line sent back.
func answer(resp Response, err error) Response {
	var refusal errRequest
	switch {
	case err == nil:
		resp["ok"] = true
		return resp
	case errors.As(err, &refusal):
		return Response{"ok": false, "error": refusal.msg}
	default:
		return Response{"ok": false, "error": "The box couldn't do that: " + err.Error()}
	}
}
