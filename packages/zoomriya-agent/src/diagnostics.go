// The diagnostics bundle (design §7.2): how ZOOMRIYA support finds what's
// wrong without signing in to the box. An admin makes one in Box Desktop,
// reviews what's in it, and saves it to a USB stick to send from a computer.
// One .tar.gz with a versioned manifest, the checks' results first, and no
// family content; passwords, tokens and private keys found in it are removed.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	diagnosticsDir    = "var/lib/zoomriya-agent/diagnostics"
	diagnosticsFormat = 1
	diagnosticsKeep   = 5
	// The system log in a bundle: the last week, at most this much of it.
	diagnosticsJournalMax = 48 << 20
	// What read returns of a part, for the admin to look through.
	diagnosticsReadMax = 256 << 10
	// The admin's note, at most.
	diagnosticsNoteMax = 2000
	// On a USB stick, bundles go in this folder.
	usbBundleFolder = "ZOOMRIYA"
)

var bundleName = regexp.MustCompile(`^zoomriya-diagnostics-\d{8}-\d{6}\.tar\.gz$`)

// The parts an admin may leave out: they say most about the home.
var optionalParts = map[string]bool{"journal": true, "network": true}

// What a bundle holds from the agent's own requests, all read only.
var diagnosticsRequests = []struct{ Op, What string }{
	{"status", "The OS's version, uptime, restarts waiting, the maintenance hour"},
	{"disk-health", "Each drive's health (SMART)"},
	{"storage", "Space used, and the library's mirror and last check"},
	{"resources", "Processor, memory, temperatures, and each service's use"},
	{"security", "Security updates, the firewall, the file integrity check, AppArmor, Secure Boot"},
	{"restore-point-list", "The restore points"},
	{"held", "Updates held back by a restore"},
	{"notifications", "The last month's notifications"},
	{"keyboard", "The keyboard layout"},
	{"setup-status", "The first setup's state"},
}

// Passwords, tokens and private keys, wherever they turn up.
var redactions = []struct {
	pattern *regexp.Regexp
	with    string
}{
	{regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`), "[private key removed]"},
	{regexp.MustCompile(`(?i)\b(bearer)\s+[A-Za-z0-9._~+/=-]{8,}`), "$1 [removed]"},
	{regexp.MustCompile(`(?i)\b(password|passwd|passphrase|secret|token|api[_-]?key|access[_-]?key|setup[_-]?code)("?\s*[:=]\s*"?)[^\s",;&]+`), "$1$2[removed]"},
}

func redact(text string) (string, int) {
	count := 0
	for _, r := range redactions {
		text = r.pattern.ReplaceAllStringFunc(text, func(found string) string {
			count++
			return r.pattern.ReplaceAllString(found, r.with)
		})
	}
	return text, count
}

type BundlePart struct {
	File      string `json:"file"`
	What      string `json:"what"`
	Bytes     int    `json:"bytes"`
	Sensitive bool   `json:"sensitive"`
}

type DiagCheck struct {
	ID       string `json:"id"`
	Severity string `json:"severity"` // "error", "warning" or "info"
	Summary  string `json:"summary"`
}

type Manifest struct {
	Format   int            `json:"format"`
	Made     string         `json:"made"`
	Box      map[string]any `json:"box"`
	Note     string         `json:"note,omitempty"`
	Parts    []BundlePart   `json:"parts"`
	LeftOut  []string       `json:"left_out"`
	Redacted int            `json:"redacted"`
	Checks   []DiagCheck    `json:"checks"`
	Apps     string         `json:"apps"`
}

type Bundle struct {
	Name     string `json:"name"`
	Size     int64  `json:"size_bytes"`
	Made     string `json:"made"`
	Errors   int    `json:"errors"`
	Warnings int    `json:"warnings"`
}

// diagnostics: no action lists the bundles and the USB sticks plugged in;
// "make" (Include, and the admin's Note), "show", "read" (Part),
// "export" (Device) and "delete", each of a bundle's Name.
func (a *Agent) diagnostics(req Request) (Response, error) {
	action := ""
	if req.Action != nil {
		action = *req.Action
	}
	name := ""
	if action != "" && action != "make" {
		if req.Name == nil || !bundleName.MatchString(*req.Name) {
			return nil, refuse("Which bundle?")
		}
		name = *req.Name
		if _, err := os.Stat(filepath.Join(a.path(diagnosticsDir), name)); err != nil {
			return nil, refuse("That bundle isn't on the box any more.")
		}
	}
	path := filepath.Join(a.path(diagnosticsDir), name)

	switch action {
	case "":
		bundles, err := a.bundles()
		if err != nil {
			return nil, err
		}
		sticks, err := a.usbSticks()
		if err != nil {
			sticks = []Stick{}
		}
		return Response{"bundles": bundles, "usb": sticks}, nil
	case "make":
		note := ""
		if req.Note != nil {
			note = strings.TrimSpace(*req.Note)
		}
		made, err := a.makeDiagnostics(req.Include, note)
		if err != nil {
			return nil, err
		}
		bundles, err := a.bundles()
		if err != nil {
			return nil, err
		}
		return Response{"bundles": bundles, "made": made}, nil
	case "show":
		manifest, err := readManifest(path)
		if err != nil {
			return nil, err
		}
		return Response{"name": name, "manifest": manifest}, nil
	case "read":
		if req.Part == nil {
			return nil, refuse("Which part of the bundle?")
		}
		manifest, err := readManifest(path)
		if err != nil {
			return nil, err
		}
		known := false
		for _, p := range manifest.Parts {
			known = known || p.File == *req.Part
		}
		if !known {
			return nil, refuse("There's no such part in the bundle.")
		}
		data, err := readPart(path, *req.Part)
		if err != nil {
			return nil, err
		}
		cut := len(data) > diagnosticsReadMax
		if cut {
			data = data[:diagnosticsReadMax]
		}
		return Response{"part": *req.Part, "text": string(bytes.ToValidUTF8(data, []byte("?"))), "cut": cut}, nil
	case "export":
		if req.Device == nil {
			return nil, refuse("Which USB stick?")
		}
		saved := filepath.Join(usbBundleFolder, name)
		err := a.withStick(*req.Device, true, func(dir string) error {
			return copyDurably(path, filepath.Join(dir, saved))
		})
		if err != nil {
			return nil, err
		}
		return Response{"saved": saved, "device": *req.Device}, nil
	case "delete":
		if err := os.Remove(path); err != nil {
			return nil, err
		}
		bundles, err := a.bundles()
		if err != nil {
			return nil, err
		}
		return Response{"bundles": bundles}, nil
	}
	return nil, refuse("Make, show, read, save or delete a bundle?")
}

func (a *Agent) bundles() ([]Bundle, error) {
	list := []Bundle{}
	dir := a.path(diagnosticsDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return list, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !bundleName.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		b := Bundle{Name: e.Name(), Size: info.Size(), Made: info.ModTime().UTC().Format(time.RFC3339)}
		if m, err := readManifest(filepath.Join(dir, e.Name())); err == nil {
			b.Made = m.Made
			for _, c := range m.Checks {
				switch c.Severity {
				case "error":
					b.Errors++
				case "warning":
					b.Warnings++
				}
			}
		}
		list = append(list, b)
	}
	// Newest first; the name ends in the time it was made.
	sort.Slice(list, func(i, j int) bool { return list[i].Name > list[j].Name })
	return list, nil
}

const diagnosticsReadme = `ZOOMRIYA OS diagnostics

Made on the box by an admin, in Box Desktop, for ZOOMRIYA support.
manifest.json lists every file in it, what it is, and the checks' results:
start there. Format 1.

No photos, files, messages or other family content are in it. Passwords,
tokens and private keys found in the logs are replaced by [removed].
`

type part struct {
	BundlePart
	data []byte
}

// makeDiagnostics writes a new bundle, root's only, and keeps the newest few.
func (a *Agent) makeDiagnostics(include []string, note string) (string, error) {
	for _, p := range include {
		if !optionalParts[p] {
			return "", refuse("There's no part called %q.", p)
		}
	}
	// No include: everything. Otherwise, only the optional parts named.
	wanted := func(p string) bool { return include == nil || contains(include, p) }
	if r := []rune(note); len(r) > diagnosticsNoteMax {
		note = string(r[:diagnosticsNoteMax])
	}

	now := a.now().UTC()
	name := fmt.Sprintf("zoomriya-diagnostics-%s.tar.gz", now.Format("20060102-150405"))
	redacted := 0
	parts := []part{}
	add := func(file, what string, sensitive bool, data []byte) {
		text, n := redact(string(data))
		redacted += n
		parts = append(parts, part{BundlePart{File: file, What: what, Bytes: len(text), Sensitive: sensitive}, []byte(text)})
	}
	asJSON := func(resp Response, err error) []byte {
		if err != nil {
			resp = Response{"error": err.Error()}
		}
		data, _ := json.MarshalIndent(resp, "", "  ")
		return append(data, '\n')
	}
	run := func(name string, args ...string) []byte {
		out, err := a.Run(name, args...)
		if err != nil {
			return append(out, []byte("\n["+name+" failed: "+err.Error()+"]\n")...)
		}
		return out
	}

	versions, _ := a.Run("dpkg-query", "-W", "-f=${Package} ${Version}\n")
	add("versions.txt", "ZOOMRIYA OS's release and every package's version", false,
		[]byte(a.readTrimmed("usr/lib/os-release")+"\n\n"+string(versions)))
	answers := map[string]Response{}
	for _, r := range diagnosticsRequests {
		resp, err := a.Handle(Request{Op: r.Op})
		answers[r.Op] = resp
		add(r.Op+".json", r.What, false, asJSON(resp, err))
	}
	if wanted("network") {
		add("network.json", "The box's network: its ports, addresses, router and name servers", true,
			asJSON(a.Handle(Request{Op: "network"})))
	}

	// Services that failed, each with its last lines.
	failedOut, _ := a.Run("systemctl", "--failed", "--plain", "--no-legend", "--no-pager")
	failed := []string{}
	var failedText strings.Builder
	failedText.Write(failedOut)
	for _, line := range strings.Split(string(failedOut), "\n") {
		if fields := strings.Fields(line); len(fields) > 0 {
			failed = append(failed, fields[0])
			failedText.WriteString("\n== " + fields[0] + " ==\n")
			failedText.Write(run("journalctl", "--no-pager", "--quiet", "--output=short-iso", "--boot", "--lines=40", "--unit="+fields[0]))
		}
	}
	if len(failed) == 0 {
		failedText.WriteString("No service has failed.\n")
	}
	add("failed-units.txt", "Services that failed, each with its last log lines", false, []byte(failedText.String()))

	add("boots.txt", "Each start of the box, and the kernel's tries", false,
		run("journalctl", "--no-pager", "--list-boots"))
	if wanted("journal") {
		journal := run("journalctl", "--no-pager", "--quiet", "--output=short-iso", "--since=-7d")
		if len(journal) > diagnosticsJournalMax {
			journal = append([]byte("[the older part was left out]\n"), journal[len(journal)-diagnosticsJournalMax:]...)
		}
		add("journal.txt", "The system log of the last 7 days, with the admins' actions and sign-ins", true, journal)
		add("previous-boot.txt", "The system log of the start before this one, to its end", true,
			run("journalctl", "--no-pager", "--quiet", "--output=short-iso", "--boot=-1", "--lines=5000"))
	}

	// Hand changes, which updates and restore points may undo (design §7.1).
	var access strings.Builder
	if on := a.accessIsOn(); on != nil {
		fmt.Fprintf(&access, "Advanced access: on since %s, turned on by %s.\n", on.Since, on.By)
	} else {
		access.WriteString("Advanced access: off.\n")
	}
	used := a.lastSignIns()
	fmt.Fprintf(&access, "Keys: %d. Sign-ins over SSH: %d keys used.\n", len(a.accessKeys()), len(used))
	for fingerprint, at := range used {
		fmt.Fprintf(&access, "  last sign-in %s with %s\n", at, fingerprint)
	}
	access.WriteString("\nFiles that differ from their packages (dpkg --verify):\n")
	access.Write(run("dpkg", "--verify"))
	add("advanced-access.txt", "Whether advanced access was used, and the OS's files changed by hand", false, []byte(access.String()))

	add("connect.txt", "ZOOMRIYA Connect's services on the box", false,
		run("systemctl", "status", "--no-pager", "--lines=20", "zoomriya-connect*"))

	box := Response{}
	if status := answers["status"]; status != nil {
		for _, key := range []string{"model", "serial", "os", "image_version", "support_end", "base", "kernel"} {
			box[key] = status[key]
		}
	}
	leftOut := []string{}
	for _, p := range []string{"journal", "network"} {
		if !wanted(p) {
			leftOut = append(leftOut, p)
		}
	}
	manifest := Manifest{Format: diagnosticsFormat, Made: now.Format(time.RFC3339), Box: box, Note: note,
		LeftOut: leftOut, Redacted: redacted, Checks: a.diagnosticChecks(answers, failed),
		Apps: "Each app's own diagnostics come with App Center; not in this format yet."}
	for _, p := range parts {
		manifest.Parts = append(manifest.Parts, p.BundlePart)
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	parts = append([]part{
		{BundlePart{File: "manifest.json", What: "What's in the bundle, and the checks' results"}, append(data, '\n')},
		{BundlePart{File: "README.txt", What: "How to read the bundle"}, []byte(diagnosticsReadme)},
	}, parts...)

	var buf bytes.Buffer
	zipped := gzip.NewWriter(&buf)
	archive := tar.NewWriter(zipped)
	folder := strings.TrimSuffix(name, ".tar.gz")
	for _, p := range parts {
		header := &tar.Header{Name: folder + "/" + p.File, Mode: 0o644, Size: int64(len(p.data)), ModTime: now,
			Typeflag: tar.TypeReg}
		if err := archive.WriteHeader(header); err != nil {
			return "", err
		}
		if _, err := archive.Write(p.data); err != nil {
			return "", err
		}
	}
	if err := archive.Close(); err != nil {
		return "", err
	}
	if err := zipped.Close(); err != nil {
		return "", err
	}
	dir := a.path(diagnosticsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := writeFileAtomic(filepath.Join(dir, name), buf.Bytes(), 0o600); err != nil {
		return "", err
	}
	if bundles, err := a.bundles(); err == nil {
		for _, old := range bundles[min(len(bundles), diagnosticsKeep):] {
			_ = os.Remove(filepath.Join(dir, old.Name))
		}
	}
	return name, nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// generic turns an answer into plain JSON values, to look into.
func generic(resp Response) map[string]any {
	data, _ := json.Marshal(resp)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}

func field(m map[string]any, path ...string) any {
	var v any = m
	for _, key := range path {
		next, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = next[key]
	}
	return v
}

// diagnosticChecks: what support should look at first, worst first.
func (a *Agent) diagnosticChecks(answers map[string]Response, failed []string) []DiagCheck {
	checks := []DiagCheck{}
	add := func(id, severity, format string, args ...any) {
		checks = append(checks, DiagCheck{ID: id, Severity: severity, Summary: fmt.Sprintf(format, args...)})
	}
	for _, unit := range failed {
		add("failed-unit", "error", "%s failed.", unit)
	}
	if drives, ok := field(generic(answers["disk-health"]), "drives").([]any); ok {
		for _, d := range drives {
			drive, _ := d.(map[string]any)
			name := fmt.Sprint(drive["model"])
			switch drive["health"] {
			case healthReplaceNow:
				add("drive", "error", "%s should be replaced now.", name)
			case healthCheckSoon:
				add("drive", "warning", "%s should be checked soon.", name)
			}
		}
	}
	storage := generic(answers["storage"])
	for _, place := range []string{"system", "library"} {
		size, _ := field(storage, place, "size_bytes").(float64)
		used, _ := field(storage, place, "used_bytes").(float64)
		if size > 0 && used/size > 0.9 {
			add("space", "warning", "The %s is %.0f%% full.", map[string]string{"system": "box's own disk", "library": "library"}[place], used/size*100)
		}
	}
	if errs, _ := field(storage, "library", "check", "errors").(float64); errs > 0 {
		add("library-check", "error", "The library's last check found %.0f problem(s).", errs)
	}
	security := generic(answers["security"])
	if field(security, "updates", "automatic") == false {
		add("security-updates", "error", "Security updates don't install by themselves.")
	}
	if field(security, "firewall", "on") == false {
		add("firewall", "error", "The firewall isn't running.")
	}
	if field(security, "integrity", "clean") == false {
		add("integrity", "warning", "OS files changed outside an update.")
	}
	if field(security, "apparmor", "on") == false {
		add("apparmor", "warning", "AppArmor is off.")
	}
	status := generic(answers["status"])
	if field(status, "restart_needed") == true {
		add("restart", "info", "A restart is waiting, for the maintenance hour.")
	}
	if held, ok := field(generic(answers["held"]), "held").([]any); ok && len(held) > 0 {
		add("held", "info", "%d update(s) held back by a restore.", len(held))
	}
	if a.accessIsOn() != nil {
		add("advanced-access", "info", "Advanced access is on: changes by hand are possible.")
	}
	rank := map[string]int{"error": 0, "warning": 1, "info": 2}
	sort.SliceStable(checks, func(i, j int) bool { return rank[checks[i].Severity] < rank[checks[j].Severity] })
	return checks
}

// readPart is one file of a bundle.
func readPart(path, file string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zipped, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	archive := tar.NewReader(zipped)
	for {
		header, err := archive.Next()
		if err == io.EOF {
			return nil, refuse("There's no such part in the bundle.")
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(header.Name) == file {
			return io.ReadAll(io.LimitReader(archive, diagnosticsJournalMax+(1<<20)))
		}
	}
}

func readManifest(path string) (Manifest, error) {
	var m Manifest
	data, err := readPart(path, "manifest.json")
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(data, &m)
}

// copyDurably copies a file and makes sure it's on the stick before it's
// unmounted.
func copyDurably(from, to string) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	temp := to + ".part"
	dst, err := os.OpenFile(temp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return refuse("The box couldn't write to that USB stick. Is it full, or locked?")
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(temp)
		return refuse("The box couldn't write to that USB stick. Is it full, or locked?")
	}
	if err := dst.Sync(); err != nil {
		dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	if err := os.Rename(temp, to); err != nil {
		return err
	}
	if dir, err := os.Open(filepath.Dir(to)); err == nil {
		_ = dir.Sync()
		dir.Close()
	}
	return nil
}
