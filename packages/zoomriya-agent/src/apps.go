// App Center (design §6.9, box-desktop.md §4): ZOOMRIYA's apps, installed by
// an admin from a USB stick, for now. Only packages signed by ZOOMRIYA's app
// key (zoomriya-base's app-keys) install; anything else is refused before
// anything changes. Installing runs as its own unit (`zoomriya-agent app-job`),
// as it can take minutes: copy from the stick, check every file, import the
// image, migrate, start, check it runs, and go back to the release before if
// it doesn't. The current and the previous release of each app are kept, for
// Restore previous version.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"zoomriya.com/os/zoomriya-agent/zapp"
)

const (
	appKeys       = "usr/share/zoomriya-base/app-keys"
	appsState     = "var/lib/zoomriya-agent/apps.json"
	appsDir       = "var/lib/zoomriya-agent/apps" // <id>/<version>/units/, and incoming/
	appJobFile    = "run/zoomriya/app-job.json"
	appJobUnit    = "zoomriya-app-job"
	usbAppsFolder = "zoomriya/apps"
	machinesDir   = "var/lib/machines"
	unitDir       = "etc/systemd/system"
	appDataRoot   = "data/apps"
	// How long a started release has to be running before it counts as working.
	appSettle = 8 * time.Second
	// Room an install needs beyond the image: the package's copy and the import.
	appSpare = 512 << 20
)

var packageFile = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,100}\.zapp$`)

type Release struct {
	Version   string   `json:"version"`
	Installed string   `json:"installed"`
	By        string   `json:"by"`
	Units     []string `json:"units"`
	Start     []string `json:"start"`
	Migrate   string   `json:"migrate,omitempty"`
}

type InstalledApp struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Active      string             `json:"active"`
	Previous    string             `json:"previous"`
	Releases    map[string]Release `json:"releases"`
	Data        bool               `json:"data"`
	Databases   []string           `json:"databases"`
}

type AppJob struct {
	Action  string `json:"action"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Step    string `json:"step"`
	By      string `json:"by"`
	Started string `json:"started"`
	Done    bool   `json:"done"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

func (a *Agent) appKeys() int { return len(a.trustedKeys()) }

func (a *Agent) loadApps() map[string]*InstalledApp {
	apps := map[string]*InstalledApp{}
	if data, err := os.ReadFile(a.path(appsState)); err == nil {
		_ = json.Unmarshal(data, &apps)
	}
	return apps
}

func (a *Agent) saveApps(apps map[string]*InstalledApp) error {
	data, _ := json.MarshalIndent(apps, "", "  ")
	return writeFileAtomic(a.path(appsState), data, 0o600)
}

func instances(templates []string, version string) []string {
	out := []string{}
	for _, t := range templates {
		out = append(out, strings.Replace(t, "@.", "@"+version+".", 1))
	}
	return out
}

// appsRequest: no action lists the apps and any job; "usb" lists the packages
// on the USB sticks plugged in; "install" (Device, Name: the package's file)
// starts installing one; "restore" (Name: the app) goes back to the release
// before; "remove" (Name, and Include "data" to delete its data too).
func (a *Agent) appsRequest(req Request) (Response, error) {
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return strings.TrimSpace(*p)
	}
	switch str(req.Action) {
	case "":
		return a.appsList(), nil
	case "usb":
		found, err := a.usbApps()
		if err != nil {
			return nil, err
		}
		return Response{"found": found, "keys": a.appKeys()}, nil
	case "install":
		if !packageFile.MatchString(str(req.Name)) || str(req.Device) == "" {
			return nil, refuse("Which app package, on which USB stick?")
		}
		if job := a.runningJob(); job != nil {
			return nil, refuse("App Center is busy with %s. Try again when it's done.", job.Name)
		}
		job := AppJob{Action: "install", Name: str(req.Name), Step: "Starting", By: req.By,
			Started: a.now().UTC().Format(time.RFC3339)}
		if err := a.writeJob(job); err != nil {
			return nil, err
		}
		if _, err := a.Run("systemd-run", "--quiet", "--collect", "--unit="+appJobUnit, "--property=Type=oneshot",
			"--description=Install a ZOOMRIYA app", "--", agentPath, "app-job", "install",
			str(req.Device), str(req.Name), req.By); err != nil {
			_ = os.Remove(a.path(appJobFile))
			return nil, err
		}
		return a.appsList(), nil
	case "restore":
		if job := a.runningJob(); job != nil {
			return nil, refuse("App Center is busy with %s. Try again when it's done.", job.Name)
		}
		if err := a.restoreApp(str(req.Name), req.By); err != nil {
			return nil, err
		}
		return a.appsList(), nil
	case "remove":
		if job := a.runningJob(); job != nil {
			return nil, refuse("App Center is busy with %s. Try again when it's done.", job.Name)
		}
		if err := a.removeApp(str(req.Name), req.By, contains(req.Include, "data")); err != nil {
			return nil, err
		}
		return a.appsList(), nil
	}
	return nil, refuse("List, install, restore or remove an app?")
}

type AppStatus struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Previous    string `json:"previous"`
	Installed   string `json:"installed"`
	By          string `json:"by"`
	Health      string `json:"health"` // "running", "starting", "stopped" or "failed"
	Data        bool   `json:"data"`
	// Blocked: the version running is on the revocation list; update it.
	Blocked bool `json:"blocked"`
	// PreviousBlocked: Restore would go back to a blocked release.
	PreviousBlocked bool `json:"previous_blocked"`
}

func (a *Agent) appsList() Response {
	apps := a.loadApps()
	revoked := a.revoked()
	list := []AppStatus{}
	for _, app := range apps {
		r := app.Releases[app.Active]
		list = append(list, AppStatus{ID: app.ID, Name: app.Name, Description: app.Description, Version: app.Active,
			Previous: app.Previous, Installed: r.Installed, By: r.By, Health: a.health(r), Data: app.Data,
			Blocked: revoked.blocked(app.ID, app.Active), PreviousBlocked: revoked.blocked(app.ID, app.Previous)})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	resp := Response{"apps": list, "job": nil, "keys": a.appKeys()}
	if job := a.readJob(); job != nil {
		resp["job"] = job
	}
	return resp
}

func (a *Agent) health(r Release) string {
	states := map[string]bool{}
	for _, unit := range instances(r.Start, r.Version) {
		out, _ := a.Run("systemctl", "is-active", unit)
		states[strings.TrimSpace(string(out))] = true
	}
	switch {
	case states["failed"]:
		return "failed"
	case states["activating"] || states["reloading"]:
		return "starting"
	case len(states) == 1 && states["active"]:
		return "running"
	}
	return "stopped"
}

// --- the job ----------------------------------------------------------------------

func (a *Agent) readJob() *AppJob {
	data, err := os.ReadFile(a.path(appJobFile))
	if err != nil {
		return nil
	}
	var job AppJob
	if json.Unmarshal(data, &job) != nil {
		return nil
	}
	return &job
}

func (a *Agent) writeJob(job AppJob) error {
	data, _ := json.Marshal(job)
	return writeFileAtomic(a.path(appJobFile), data, 0o644)
}

// runningJob: a job not done whose unit still runs (one that died is over).
func (a *Agent) runningJob() *AppJob {
	job := a.readJob()
	if job == nil || job.Done {
		return nil
	}
	out, _ := a.Run("systemctl", "is-active", appJobUnit+".service")
	if state := strings.TrimSpace(string(out)); state == "active" || state == "activating" {
		return job
	}
	if job.Step == "Starting" {
		return job // just asked; its unit may not be up yet
	}
	job.Done, job.Error = true, "It stopped before it finished."
	_ = a.writeJob(*job)
	return nil
}

// appJob is `zoomriya-agent app-job install DEVICE FILE BY`.
func appJob(args []string) int {
	a := NewAgent()
	audit := NewJournal()
	if len(args) != 4 || args[0] != "install" {
		return 2
	}
	name, err := a.installApp(args[1], args[2], args[3])
	result := "installed " + name
	if err != nil {
		result = err.Error()
	}
	audit.Record(AuditEntry{Op: "app-install", By: args[3], Detail: args[2], Result: result})
	if err != nil {
		return 1
	}
	return 0
}

// installApp installs a package from a stick, step by step, writing each step
// to the job file Box Desktop reads.
func (a *Agent) installApp(device, file, by string) (string, error) {
	job := AppJob{Action: "install", Name: file, By: by, Started: a.now().UTC().Format(time.RFC3339)}
	step := func(s string) {
		job.Step = s
		_ = a.writeJob(job)
	}
	finish := func(err error) error {
		job.Done, job.OK = true, err == nil
		if err != nil {
			job.Error = err.Error()
			var r errRequest
			if !errors.As(err, &r) {
				job.Error = "The app couldn't be installed: " + err.Error()
			}
			a.notify("apps", "error", fmt.Sprintf("%s wasn't installed. %s", job.Name, job.Error))
		} else {
			a.notify("apps", "info", fmt.Sprintf("%s %s is installed and running.", job.Name, job.Version))
		}
		_ = a.writeJob(job)
		return err
	}
	keys := a.trustedKeys()
	if len(keys) == 0 {
		return "", finish(refuse("Apps can't be installed until the box is updated."))
	}

	incoming := a.path(filepath.Join(appsDir, "incoming"))
	_ = os.RemoveAll(incoming)
	if err := os.MkdirAll(incoming, 0o700); err != nil {
		return "", finish(err)
	}
	defer os.RemoveAll(incoming)

	// Copied off the stick first, so what's checked is what's installed.
	step("Copying from the USB stick")
	copied := filepath.Join(incoming, "package.zapp")
	err := a.withStick(device, false, func(dir string) error {
		src := filepath.Join(dir, usbAppsFolder, file)
		info, err := os.Lstat(src)
		if err != nil || !info.Mode().IsRegular() {
			return refuse("%s isn't on the USB stick any more.", file)
		}
		return copyDurably(src, copied)
	})
	if err != nil {
		return "", finish(err)
	}

	step("Checking it's a ZOOMRIYA app")
	f, err := os.Open(copied)
	if err != nil {
		return "", finish(err)
	}
	m, err := zapp.Extract(f, keys, filepath.Join(incoming, "files"))
	f.Close()
	_ = os.Remove(copied)
	if err != nil {
		if errors.Is(err, zapp.ErrNotZoomriya) {
			return "", finish(refuse("%s isn't a ZOOMRIYA app, so it can't be installed.", file))
		}
		return "", finish(refuse("%s can't be installed: %s.", file, err.Error()))
	}
	job.ID, job.Name, job.Version = m.ID, m.Name, m.Version
	if err := a.canInstall(m); err != nil {
		return "", finish(err)
	}

	step("Adding it to the box")
	machine := m.Machine()
	if _, err := a.Run("machinectl", "import-tar", "--read-only",
		filepath.Join(incoming, "files", filepath.FromSlash(m.Image)), machine); err != nil {
		return "", finish(err)
	}
	release := Release{Version: m.Version, Installed: a.now().UTC().Format(time.RFC3339), By: by,
		Start: m.Start, Migrate: m.Migrate}
	keep := a.path(filepath.Join(appsDir, m.ID, m.Version, "units"))
	_ = os.RemoveAll(keep)
	for _, u := range m.Units {
		base := filepath.Base(u)
		if err := copyDurably(filepath.Join(incoming, "files", filepath.FromSlash(u)), filepath.Join(keep, base)); err != nil {
			return "", finish(err)
		}
		release.Units = append(release.Units, base)
	}

	apps := a.loadApps()
	app := apps[m.ID]
	if app == nil {
		app = &InstalledApp{ID: m.ID, Releases: map[string]Release{}}
	}
	app.Name, app.Description, app.Data, app.Databases = m.Name, m.Description, m.Data, m.Databases
	if m.Data {
		_ = os.MkdirAll(a.path(filepath.Join(appDataRoot, m.ID)), 0o750)
	}

	step("Starting it")
	before := app.Active
	if err := a.switchRelease(app, before, release); err != nil {
		_, _ = a.Run("machinectl", "remove", machine)
		_ = os.RemoveAll(a.path(filepath.Join(appsDir, m.ID, m.Version)))
		if before == "" {
			_ = os.RemoveAll(a.path(filepath.Join(appsDir, m.ID)))
		}
		return "", finish(err)
	}
	app.Releases[m.Version] = release
	app.Previous, app.Active = before, m.Version
	_ = a.noteHighest(m.ID, m.Version)
	a.pruneReleases(app)
	apps[m.ID] = app
	if err := a.saveApps(apps); err != nil {
		return "", finish(err)
	}
	return m.Name + " " + m.Version, finish(nil)
}

// canInstall: new to the box, or a release it hasn't got, on a ZOOMRIYA OS
// new enough, with the room for it.
func (a *Agent) canInstall(m zapp.Manifest) error {
	if have, _ := strconv.Atoi(a.osRelease()["VERSION_ID"]); have > 0 && m.MinOS > have {
		return refuse("%s %s needs ZOOMRIYA OS %d or newer. Update the box first.", m.Name, m.Version, m.MinOS)
	}
	if app := a.loadApps()[m.ID]; app != nil {
		if _, have := app.Releases[m.Version]; have {
			return refuse("%s %s is already on the box.", m.Name, m.Version)
		}
	}
	if _, _, why := a.stickRefusal(m); why != "" {
		return refuse("%s %s can't be installed. %s", m.Name, m.Version, why)
	}
	need := uint64(m.Files[m.Image].Bytes) + appSpare
	var fs syscall.Statfs_t
	if err := os.MkdirAll(a.path(machinesDir), 0o700); err == nil && syscall.Statfs(a.path(machinesDir), &fs) == nil {
		if free := fs.Bavail * uint64(fs.Bsize); free < need {
			return refuse("There isn't room for %s: it needs %d MB, and %d MB is free.", m.Name, need>>20, free>>20)
		}
	}
	return nil
}

// switchRelease stops from (if any) and starts to, migrating first; if to
// doesn't run, it's stopped and from started again.
func (a *Agent) switchRelease(app *InstalledApp, from string, to Release) error {
	old, hadOld := app.Releases[from]
	if hadOld {
		a.stopRelease(old)
	}
	if err := a.startRelease(app.ID, to); err != nil {
		a.stopRelease(to)
		if hadOld {
			if a.startRelease(app.ID, old) != nil {
				return fmt.Errorf("%s: and %s didn't start again either", err.Error(), old.Version)
			}
			return refuse("%s %s didn't start, so the box went back to %s. %s", app.Name, to.Version, old.Version, err.Error())
		}
		return err
	}
	return nil
}

// startRelease puts the release's units in place, migrates and starts it,
// and checks it's still running a few seconds later.
func (a *Agent) startRelease(id string, r Release) error {
	keep := a.path(filepath.Join(appsDir, id, r.Version, "units"))
	for _, u := range r.Units {
		if err := copyDurably(filepath.Join(keep, u), a.path(filepath.Join(unitDir, u))); err != nil {
			return err
		}
	}
	if _, err := a.Run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if r.Migrate != "" {
		if _, err := a.Run("systemctl", "start", instances([]string{r.Migrate}, r.Version)[0]); err != nil {
			return refuse("Its database update didn't work.")
		}
	}
	units := instances(r.Start, r.Version)
	if _, err := a.Run("systemctl", append([]string{"start"}, units...)...); err != nil {
		return refuse("It didn't start.")
	}
	a.sleep(appSettle)
	for _, u := range units {
		if out, _ := a.Run("systemctl", "is-active", u); strings.TrimSpace(string(out)) != "active" {
			return refuse("It stopped soon after starting.")
		}
	}
	_, _ = a.Run("systemctl", append([]string{"enable"}, units...)...)
	return nil
}

func (a *Agent) stopRelease(r Release) {
	units := instances(r.Start, r.Version)
	_, _ = a.Run("systemctl", append([]string{"disable", "--now"}, units...)...)
}

// pruneReleases keeps the active release and the one before.
func (a *Agent) pruneReleases(app *InstalledApp) {
	for v := range app.Releases {
		if v != app.Active && v != app.Previous {
			_, _ = a.Run("machinectl", "remove", zapp.Machine(app.ID, v))
			_ = os.RemoveAll(a.path(filepath.Join(appsDir, app.ID, v)))
			delete(app.Releases, v)
		}
	}
}

// restoreApp goes back to the release before (design §6.2's Restore previous
// version), and keeps the newer one to go forward to.
func (a *Agent) restoreApp(id, by string) error {
	apps := a.loadApps()
	app := apps[id]
	if app == nil {
		return refuse("There's no such app.")
	}
	prev, ok := app.Releases[app.Previous]
	if !ok {
		return refuse("%s has no earlier version on the box.", app.Name)
	}
	if a.revoked().blocked(id, prev.Version) {
		return refuse("%s %s has a known problem, so the box won't go back to it.", app.Name, prev.Version)
	}
	if err := a.switchRelease(app, app.Active, prev); err != nil {
		return err
	}
	app.Active, app.Previous = app.Previous, app.Active
	if err := a.saveApps(apps); err != nil {
		return err
	}
	a.notify("apps", "info", fmt.Sprintf("%s went back to %s, as %s asked.", app.Name, app.Active, by))
	return nil
}

// removeApp stops the app and removes its releases and units; with data, its
// folder under /data/apps and its databases too.
func (a *Agent) removeApp(id, by string, data bool) error {
	apps := a.loadApps()
	app := apps[id]
	if app == nil {
		return refuse("There's no such app.")
	}
	units := map[string]bool{}
	for _, r := range app.Releases {
		a.stopRelease(r)
		for _, u := range r.Units {
			units[u] = true
		}
	}
	for u := range units {
		if err := os.Remove(a.path(filepath.Join(unitDir, u))); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	_, _ = a.Run("systemctl", "daemon-reload")
	for v := range app.Releases {
		_, _ = a.Run("machinectl", "remove", zapp.Machine(id, v))
	}
	if err := os.RemoveAll(a.path(filepath.Join(appsDir, id))); err != nil {
		return err
	}
	if data {
		if app.Data {
			if err := os.RemoveAll(a.path(filepath.Join(appDataRoot, id))); err != nil {
				return err
			}
		}
		for _, db := range app.Databases {
			if _, err := a.Run("runuser", "-u", "postgres", "--", "dropdb", "--if-exists", db); err != nil {
				return err
			}
		}
	}
	delete(apps, id)
	if err := a.saveApps(apps); err != nil {
		return err
	}
	kept := " Its data is kept."
	if data {
		kept = " Its data was deleted."
	}
	a.notify("apps", "info", fmt.Sprintf("%s was removed by %s.%s", app.Name, by, kept))
	return nil
}

// --- what's on the sticks -----------------------------------------------------------

type FoundApp struct {
	Device    string `json:"device"`
	Stick     string `json:"stick"`
	File      string `json:"file"`
	Size      int64  `json:"size_bytes"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Version   string `json:"version,omitempty"`
	About     string `json:"description,omitempty"`
	OK        bool   `json:"ok"`
	Problem   string `json:"problem,omitempty"`
	Installed string `json:"installed,omitempty"`
	// "new", "update" or "same", against what's installed; "older" than the
	// newest the box has had, or "blocked" by the revocation list.
	Kind   string `json:"kind,omitempty"`
	Newest string `json:"newest,omitempty"`
}

// usbApps reads the packages in zoomriya/apps/ on each stick: only their
// manifest and signature, so it's quick. Installing checks everything again.
func (a *Agent) usbApps() ([]FoundApp, error) {
	sticks, err := a.usbSticks()
	if err != nil {
		return nil, err
	}
	keys := a.trustedKeys()
	apps := a.loadApps()
	found := []FoundApp{}
	for _, stick := range sticks {
		_ = a.withStick(stick.Device, false, func(dir string) error {
			folder := filepath.Join(dir, usbAppsFolder)
			entries, _ := os.ReadDir(folder)
			for _, e := range entries {
				info, err := os.Lstat(filepath.Join(folder, e.Name()))
				if err != nil || !info.Mode().IsRegular() || !packageFile.MatchString(e.Name()) {
					continue
				}
				item := FoundApp{Device: stick.Device, Stick: stick.Label, File: e.Name(), Size: info.Size()}
				f, err := os.Open(filepath.Join(folder, e.Name()))
				if err != nil {
					continue
				}
				m, _, err := zapp.ReadHead(f, keys)
				f.Close()
				switch {
				case errors.Is(err, zapp.ErrNotZoomriya) || len(keys) == 0:
					item.Problem = "Not a ZOOMRIYA app."
				case err != nil:
					item.Problem = "Can't be read."
				default:
					item.OK, item.ID, item.Name, item.Version, item.About = true, m.ID, m.Name, m.Version, m.Description
					item.Kind = "new"
					if kind, newest, why := a.stickRefusal(m); why != "" {
						item.OK, item.Problem, item.Kind, item.Newest = false, why, kind, newest
					}
					if app := apps[m.ID]; app != nil && item.OK {
						item.Installed = app.Active
						switch c := compareVersions(m.Version, app.Active); {
						case c > 0:
							item.Kind = "update"
						case c < 0:
							item.Kind = "older"
						default:
							item.Kind = "same"
						}
					}
				}
				found = append(found, item)
			}
			return nil
		})
	}
	return found, nil
}
