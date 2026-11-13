package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zoomriya.com/os/zoomriya-agent/zapp"
)

// appsBox: a box with ZOOMRIYA's app key, a USB stick, and units that run
// unless named in failing.
func appsBox(t *testing.T) (*Agent, *fakeBox, ed25519.PrivateKey, map[string]bool) {
	agent, box, _ := accessBox(t)
	box.answers = map[string]string{"sticks": sticksJSON}
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	box.write(appKeys, "# ZOOMRIYA's app key\ned25519 "+base64.StdEncoding.EncodeToString(pub)+" zoomriya-apps-2026\n")
	box.write("usr/lib/os-release", "NAME=\"ZOOMRIYA OS\"\nVERSION_ID=1\n")
	failing := map[string]bool{}
	inner := agent.Run
	agent.Run = func(name string, args ...string) ([]byte, error) {
		if name == "systemctl" && len(args) == 2 && args[0] == "is-active" {
			box.ran = append(box.ran, append([]string{name}, args...))
			if args[1] == appJobUnit+".service" {
				return []byte("inactive\n"), nil
			}
			if failing[args[1]] {
				return []byte("failed\n"), nil
			}
			return []byte("active\n"), nil
		}
		return inner(name, args...)
	}
	return agent, box, priv, failing
}

// onStick packs an app onto the stick, signed by key.
func onStick(t *testing.T, box *fakeBox, key ed25519.PrivateKey, version string) string {
	t.Helper()
	dir := t.TempDir()
	image := filepath.Join(dir, "image.tar")
	api := filepath.Join(dir, "zoomriya-notes-api@.service")
	migrate := filepath.Join(dir, "zoomriya-notes-migrate@.service")
	for p, body := range map[string]string{image: "image " + version, api: "[Service]\n", migrate: "[Service]\n"} {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := zapp.Manifest{ID: "notes", Name: "Notes", Version: version, Publisher: "ZOOMRIYA", MinOS: 1,
		Description: "The family's notes", Image: "image.tar", Start: []string{"zoomriya-notes-api@.service"},
		Migrate: "zoomriya-notes-migrate@.service", Data: true, Databases: []string{"notes"}}
	var buf bytes.Buffer
	if err := zapp.Pack(&buf, m, key, image, []string{api, migrate}); err != nil {
		t.Fatal(err)
	}
	file := "notes-" + version + ".zapp"
	box.write(filepath.Join(usbMount, usbAppsFolder, file), buf.String())
	return file
}

func TestInstallFromUSB(t *testing.T) {
	agent, box, key, _ := appsBox(t)
	file := onStick(t, box, key, "1.2.0")
	_, stranger, _ := ed25519.GenerateKey(rand.Reader)
	strangerPkg := onStick(t, box, stranger, "9.9.9")
	box.write(filepath.Join(usbMount, usbAppsFolder, "readme.txt"), "hello")

	resp, err := agent.Handle(Request{Op: "apps", Action: str("usb")})
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]FoundApp{}
	for _, f := range resp["found"].([]FoundApp) {
		found[f.File] = f
	}
	if f := found[file]; !f.OK || f.Name != "Notes" || f.Kind != "new" || f.Stick != "ANNA" {
		t.Errorf("%s: %+v", file, f)
	}
	if f := found[strangerPkg]; f.OK || f.Problem != "Not a ZOOMRIYA app." {
		t.Errorf("%s: %+v", strangerPkg, f)
	}
	if len(found) != 2 {
		t.Errorf("found %v", found)
	}

	// Install starts the job; the job does the work.
	if _, err := agent.Handle(Request{Op: "apps", Action: str("install"), Device: str("/dev/sdb1"), Name: str(file), By: "amma"}); err != nil {
		t.Fatal(err)
	}
	if !ranCommand(box, "systemd-run --quiet --collect --unit=zoomriya-app-job --property=Type=oneshot --description=Install a ZOOMRIYA app -- /usr/lib/zoomriya-agent/zoomriya-agent app-job install /dev/sdb1 "+file+" amma") {
		t.Errorf("ran %v", box.ran)
	}
	if name, err := agent.installApp("/dev/sdb1", file, "amma"); err != nil || name != "Notes 1.2.0" {
		t.Fatal(name, err)
	}
	for _, want := range []string{
		"machinectl import-tar --read-only " + filepath.Join(box.root, appsDir, "incoming/files/image.tar") + " zoomriya-notes-1.2.0",
		"systemctl daemon-reload",
		"systemctl start zoomriya-notes-migrate@1.2.0.service",
		"systemctl start zoomriya-notes-api@1.2.0.service",
		"systemctl enable zoomriya-notes-api@1.2.0.service",
	} {
		if !ranCommand(box, want) {
			t.Errorf("didn't run %s", want)
		}
	}
	if _, err := os.Stat(filepath.Join(box.root, unitDir, "zoomriya-notes-api@.service")); err != nil {
		t.Error("unit not installed:", err)
	}
	if _, err := os.Stat(filepath.Join(box.root, appsDir, "incoming")); !os.IsNotExist(err) {
		t.Error("the copy is still there")
	}
	resp, _ = agent.Handle(Request{Op: "apps"})
	apps := resp["apps"].([]AppStatus)
	if len(apps) != 1 || apps[0].Version != "1.2.0" || apps[0].Health != "running" || apps[0].By != "amma" {
		t.Errorf("apps = %+v", apps)
	}
	if job := resp["job"].(*AppJob); !job.Done || !job.OK || job.Name != "Notes" {
		t.Errorf("job = %+v", job)
	}

	// The same again is refused; a stranger's package never installs.
	if _, err := agent.installApp("/dev/sdb1", file, "amma"); err == nil {
		t.Error("installed the same version twice")
	}
	box.ran = nil
	if _, err := agent.installApp("/dev/sdb1", strangerPkg, "amma"); err == nil || !strings.Contains(err.Error(), "isn't a ZOOMRIYA app") {
		t.Errorf("stranger: %v", err)
	}
	if len(ran(box, "machinectl")) > 0 {
		t.Errorf("a stranger's package got as far as %v", ran(box, "machinectl"))
	}
}

func TestUpdateRollsBackAndRestore(t *testing.T) {
	agent, box, key, failing := appsBox(t)
	if _, err := agent.installApp("/dev/sdb1", onStick(t, box, key, "1.2.0"), "amma"); err != nil {
		t.Fatal(err)
	}

	// 1.3.0 doesn't run: the box goes back to 1.2.0 and drops 1.3.0.
	failing["zoomriya-notes-api@1.3.0.service"] = true
	box.ran = nil
	_, err := agent.installApp("/dev/sdb1", onStick(t, box, key, "1.3.0"), "amma")
	if err == nil || !strings.Contains(err.Error(), "went back to 1.2.0") {
		t.Fatalf("err = %v", err)
	}
	if !ranCommand(box, "machinectl remove zoomriya-notes-1.3.0") || !ranCommand(box, "systemctl start zoomriya-notes-api@1.2.0.service") {
		t.Errorf("ran %v", box.ran)
	}
	if app := agent.loadApps()["notes"]; app.Active != "1.2.0" || app.Previous != "" {
		t.Errorf("app = %+v", app)
	}

	// 1.3.1 runs; then Restore previous version goes back to 1.2.0.
	if _, err := agent.installApp("/dev/sdb1", onStick(t, box, key, "1.3.1"), "amma"); err != nil {
		t.Fatal(err)
	}
	if app := agent.loadApps()["notes"]; app.Active != "1.3.1" || app.Previous != "1.2.0" {
		t.Errorf("after update %+v", app)
	}
	if _, err := agent.Handle(Request{Op: "apps", Action: str("restore"), Name: str("notes"), By: "amma"}); err != nil {
		t.Fatal(err)
	}
	if app := agent.loadApps()["notes"]; app.Active != "1.2.0" || app.Previous != "1.3.1" {
		t.Errorf("after restore %+v", app)
	}

	// A third release drops the oldest.
	if _, err := agent.installApp("/dev/sdb1", onStick(t, box, key, "1.4.0"), "amma"); err != nil {
		t.Fatal(err)
	}
	app := agent.loadApps()["notes"]
	if len(app.Releases) != 2 || !ranCommand(box, "machinectl remove zoomriya-notes-1.3.1") {
		t.Errorf("releases %v", app.Releases)
	}
}

func TestRemoveApp(t *testing.T) {
	agent, box, key, _ := appsBox(t)
	if _, err := agent.installApp("/dev/sdb1", onStick(t, box, key, "1.2.0"), "amma"); err != nil {
		t.Fatal(err)
	}
	box.write(filepath.Join(appDataRoot, "notes", "a-note.txt"), "milk")

	// Kept data by default.
	if _, err := agent.Handle(Request{Op: "apps", Action: str("remove"), Name: str("notes"), By: "amma"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(box.root, appDataRoot, "notes", "a-note.txt")); err != nil {
		t.Error("the data went, though kept")
	}
	if _, err := os.Stat(filepath.Join(box.root, unitDir, "zoomriya-notes-api@.service")); !os.IsNotExist(err) {
		t.Error("the unit is still there")
	}
	if !ranCommand(box, "machinectl remove zoomriya-notes-1.2.0") || ranCommand(box, "runuser -u postgres -- dropdb --if-exists notes") {
		t.Errorf("ran %v", box.ran)
	}

	// With its data.
	if _, err := agent.installApp("/dev/sdb1", onStick(t, box, key, "1.2.0"), "amma"); err != nil {
		t.Fatal(err)
	}
	if _, err := agent.Handle(Request{Op: "apps", Action: str("remove"), Name: str("notes"), Include: []string{"data"}, By: "amma"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(box.root, appDataRoot, "notes")); !os.IsNotExist(err) {
		t.Error("the data is still there")
	}
	if !ranCommand(box, "runuser -u postgres -- dropdb --if-exists notes") {
		t.Errorf("ran %v", box.ran)
	}
	if resp, _ := agent.Handle(Request{Op: "apps"}); len(resp["apps"].([]AppStatus)) != 0 {
		t.Errorf("apps %v", resp["apps"])
	}
}

func TestNoKeyNoApps(t *testing.T) {
	agent, box, key, _ := appsBox(t)
	file := onStick(t, box, key, "1.2.0")
	box.write(appKeys, "# none yet\n")
	if _, err := agent.installApp("/dev/sdb1", file, "amma"); err == nil {
		t.Error("installed with no key on the box")
	}
	resp, _ := agent.Handle(Request{Op: "apps", Action: str("usb")})
	if f := resp["found"].([]FoundApp); len(f) != 1 || f[0].OK {
		t.Errorf("found %+v", f)
	}
	for _, bad := range []string{"../../etc/shadow", "notes.tar", ""} {
		if _, err := agent.Handle(Request{Op: "apps", Action: str("install"), Device: str("/dev/sdb1"), Name: str(bad)}); err == nil {
			t.Errorf("install %q accepted", bad)
		}
	}
}

func TestNoOlderVersionsFromAStick(t *testing.T) {
	agent, box, key, _ := appsBox(t)
	old := onStick(t, box, key, "1.1.0")
	if _, err := agent.installApp("/dev/sdb1", onStick(t, box, key, "1.2.0"), "amma"); err != nil {
		t.Fatal(err)
	}
	_, err := agent.installApp("/dev/sdb1", old, "amma")
	if err == nil || !strings.Contains(err.Error(), "Older than 1.2.0") {
		t.Errorf("older: %v", err)
	}
	resp, _ := agent.Handle(Request{Op: "apps", Action: str("usb")})
	for _, f := range resp["found"].([]FoundApp) {
		if f.File == old && (f.OK || f.Kind != "older") {
			t.Errorf("%+v", f)
		}
	}

	// Removed, the box still remembers: 1.1.0 stays refused, 1.2.0 comes back.
	if _, err := agent.Handle(Request{Op: "apps", Action: str("remove"), Name: str("notes"), By: "amma"}); err != nil {
		t.Fatal(err)
	}
	if _, err := agent.installApp("/dev/sdb1", old, "amma"); err == nil {
		t.Error("an older version came back after a removal")
	}
	if _, err := agent.installApp("/dev/sdb1", "notes-1.2.0.zapp", "amma"); err != nil {
		t.Errorf("reinstalling the newest: %v", err)
	}
}

func TestRevocation(t *testing.T) {
	agent, box, key, _ := appsBox(t)
	if _, err := agent.installApp("/dev/sdb1", onStick(t, box, key, "1.2.0"), "amma"); err != nil {
		t.Fatal(err)
	}
	if _, err := agent.installApp("/dev/sdb1", onStick(t, box, key, "1.3.0"), "amma"); err != nil {
		t.Fatal(err)
	}

	// An OS update blocks Notes 1.2.0 (and old Photos, which isn't here).
	box.write(appRevoked, "# revoked\nrelease notes 1.2.0\nbelow photos 1.4.0\n")
	if _, err := agent.Handle(Request{Op: "apps", Action: str("restore"), Name: str("notes")}); err == nil {
		t.Error("restored a blocked release")
	}
	resp, _ := agent.Handle(Request{Op: "apps"})
	if a := resp["apps"].([]AppStatus)[0]; a.Blocked || !a.PreviousBlocked {
		t.Errorf("status %+v", a)
	}

	// The running one blocked: the watcher says so, once.
	box.write(appRevoked, "below notes 1.4.0\n")
	sent := &sentNotes{}
	if err := agent.watch(sent); err != nil {
		t.Fatal(err)
	}
	if err := agent.watch(sent); err != nil {
		t.Fatal(err)
	}
	blocked := 0
	for _, n := range sent.sent {
		if strings.HasPrefix(n, "apps error: Notes 1.3.0 has a known problem") {
			blocked++
		}
	}
	if blocked != 1 {
		t.Errorf("notifications %v", sent.sent)
	}
	if _, err := agent.installApp("/dev/sdb1", onStick(t, box, key, "1.3.5"), "amma"); err == nil {
		t.Error("installed a release below the floor")
	}
	if _, err := agent.installApp("/dev/sdb1", onStick(t, box, key, "1.4.0"), "amma"); err != nil {
		t.Errorf("the fixed release: %v", err)
	}

	// A revoked key: nothing it signed installs, or even looks installable.
	pub := key.Public().(ed25519.PublicKey)
	box.write(appRevoked, "key "+base64.StdEncoding.EncodeToString(pub)+"\n")
	if _, err := agent.installApp("/dev/sdb1", onStick(t, box, key, "1.5.0"), "amma"); err == nil {
		t.Error("installed with a revoked key")
	}
	if resp, _ := agent.Handle(Request{Op: "apps"}); resp["keys"] != 0 {
		t.Errorf("keys = %v", resp["keys"])
	}
}
