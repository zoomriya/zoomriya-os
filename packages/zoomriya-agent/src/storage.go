// storage and storage-check (design §7): the library's space and its mirror,
// the system drive's space by what uses it, and the check that both copies of
// the library match (a Btrfs scrub).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The library is the Btrfs filesystem on the library drives; storaged mounts
// it under here (Photos' drive helper).
const libraryMounts = "/data/zoomriya/volumes/"

// The unit the check runs in.
const checkUnit = "zoomriya-storage-check"

// What fills the system drive, counted at most every ten minutes: du over a
// big tree takes a while.
const (
	partsCache    = "run/zoomriya/storage-parts.json"
	partsCacheAge = 10 * time.Minute
)

var systemParts = []struct {
	Kind  string
	Paths []string
}{
	{"databases", []string{"/var/lib/postgresql"}},
	{"app-images", []string{"/var/lib/machines"}},
	{"models-and-maps", []string{"/data/models", "/data/maps"}},
	{"app-data", []string{"/data"}},
}

type Space struct {
	SizeBytes int64 `json:"size_bytes"`
	UsedBytes int64 `json:"used_bytes"`
	FreeBytes int64 `json:"free_bytes"`
}

type Part struct {
	Kind  string `json:"kind"`
	Bytes int64  `json:"bytes"`
}

type LibraryDrive struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Missing bool   `json:"missing"`
}

type Check struct {
	Running  bool    `json:"running"`
	Paused   bool    `json:"paused"`
	Percent  float64 `json:"percent"`
	Started  string  `json:"started"`
	Errors   int     `json:"errors"`
	Finished bool    `json:"finished"`
	NeverRun bool    `json:"never_run"`
}

type Library struct {
	Space
	Path   string         `json:"path"`
	Mirror bool           `json:"mirror"`
	Drives []LibraryDrive `json:"drives"`
	Errors int64          `json:"errors"`
	Check  Check          `json:"check"`
}

func (a *Agent) storage() (Response, error) {
	library, err := a.library()
	if err != nil {
		return nil, err
	}
	system, err := a.space("/")
	if err != nil {
		return nil, err
	}
	return Response{
		"library":      library,
		"system":       system,
		"system_parts": a.parts(system.UsedBytes),
	}, nil
}

// library finds the Btrfs filesystem mounted under libraryMounts; nil when
// the library isn't set up.
func (a *Agent) library() (*Library, error) {
	out, err := a.Run("findmnt", "--json", "--list", "--types", "btrfs", "--output", "TARGET")
	if err != nil {
		return nil, nil // nothing mounted that way
	}
	var mounts struct {
		Filesystems []struct {
			Target string `json:"target"`
		} `json:"filesystems"`
	}
	if json.Unmarshal(out, &mounts) != nil {
		return nil, nil
	}
	path := ""
	for _, m := range mounts.Filesystems {
		if strings.HasPrefix(m.Target, libraryMounts) {
			path = m.Target
			break
		}
	}
	if path == "" {
		return nil, nil
	}

	space, err := a.space(path)
	if err != nil {
		return nil, err
	}
	lib := &Library{Space: *space, Path: path, Drives: []LibraryDrive{}}

	if out, err := a.Run("btrfs", "filesystem", "df", path); err == nil {
		lib.Mirror = strings.Contains(string(out), "Data, RAID1")
	}
	if out, err := a.Run("btrfs", "filesystem", "show", path); err == nil {
		lib.Drives = parseDevices(string(out))
	}
	if out, err := a.Run("btrfs", "device", "stats", path); err == nil {
		lib.Errors = sumStats(string(out))
	}
	if out, err := a.Run("btrfs", "scrub", "status", path); err == nil {
		lib.Check = parseScrub(string(out))
	}
	return lib, nil
}

var (
	devicePattern = regexp.MustCompile(`devid\s+\d+\s+size\s+\S+\s+used\s+\S+\s+path\s+(\S+)`)
	missingDevice = regexp.MustCompile(`devid\s+\d+\s+size\s+\S+\s+used\s+\S+\s+path\s+<missing disk #\d+>`)
)

func parseDevices(show string) []LibraryDrive {
	drives := []LibraryDrive{}
	for _, m := range devicePattern.FindAllStringSubmatch(show, -1) {
		path := m[1]
		if strings.HasPrefix(path, "<") {
			continue
		}
		name := strings.TrimPrefix(path, "/dev/")
		drives = append(drives, LibraryDrive{Path: path, Name: name})
	}
	for range missingDevice.FindAllString(show, -1) {
		drives = append(drives, LibraryDrive{Missing: true})
	}
	return drives
}

// sumStats adds up `btrfs device stats`: read, write, flush, corruption and
// generation errors on every drive.
func sumStats(stats string) int64 {
	var total int64
	for _, line := range strings.Split(stats, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			if n, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				total += n
			}
		}
	}
	return total
}

var (
	scrubStatus  = regexp.MustCompile(`(?m)^Status:\s+(\w+)`)
	scrubStarted = regexp.MustCompile(`(?m)^Scrub started:\s+(.+)$`)
	scrubPercent = regexp.MustCompile(`(?m)^Bytes scrubbed:\s+\S+\s+\(([\d.]+)%\)`)
	scrubErrors  = regexp.MustCompile(`(?m)^Error summary:\s+(.+)$`)
	errorCount   = regexp.MustCompile(`=(\d+)`)
)

func parseScrub(status string) Check {
	var c Check
	if strings.Contains(status, "no stats available") {
		c.NeverRun = true
		return c
	}
	if m := scrubStatus.FindStringSubmatch(status); m != nil {
		switch m[1] {
		case "running":
			c.Running = true
		case "interrupted", "aborted":
			c.Paused = true
		case "finished":
			c.Finished = true
		}
	}
	if m := scrubStarted.FindStringSubmatch(status); m != nil {
		c.Started = strings.TrimSpace(m[1])
	}
	if m := scrubPercent.FindStringSubmatch(status); m != nil {
		c.Percent, _ = strconv.ParseFloat(m[1], 64)
	} else if c.Finished {
		c.Percent = 100
	}
	if m := scrubErrors.FindStringSubmatch(status); m != nil && !strings.Contains(m[1], "no errors") {
		for _, n := range errorCount.FindAllStringSubmatch(m[1], -1) {
			v, _ := strconv.Atoi(n[1])
			c.Errors += v
		}
	}
	return c
}

func (a *Agent) space(path string) (*Space, error) {
	out, err := a.Run("df", "--block-size=1", "--output=size,used,avail", path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) != 3 {
		return nil, fmt.Errorf("df's answer: %q", out)
	}
	var n [3]int64
	for i, f := range fields {
		n[i], _ = strconv.ParseInt(f, 10, 64)
	}
	return &Space{SizeBytes: n[0], UsedBytes: n[1], FreeBytes: n[2]}, nil
}

// parts is what fills the system drive; "system" is what's left of used.
func (a *Agent) parts(used int64) []Part {
	if data, err := os.ReadFile(a.path(partsCache)); err == nil {
		if info, err := os.Stat(a.path(partsCache)); err == nil && a.now().Sub(info.ModTime()) < partsCacheAge {
			var cached []Part
			if json.Unmarshal(data, &cached) == nil {
				return cached
			}
		}
	}

	parts := []Part{}
	var counted int64
	seen := map[string]int64{}
	for _, p := range systemParts {
		var bytes int64
		for _, path := range p.Paths {
			n := a.du(path)
			seen[path] = n
			bytes += n
		}
		if p.Kind == "app-data" { // /data, less the models and maps counted above
			bytes -= seen["/data/models"] + seen["/data/maps"]
		}
		if bytes < 0 {
			bytes = 0
		}
		counted += bytes
		parts = append(parts, Part{Kind: p.Kind, Bytes: bytes})
	}
	system := used - counted
	if system < 0 {
		system = 0
	}
	parts = append(parts, Part{Kind: "system", Bytes: system})

	if data, err := json.Marshal(parts); err == nil {
		_ = os.MkdirAll(a.path("run/zoomriya"), 0o755)
		_ = os.WriteFile(a.path(partsCache), data, 0o644)
	}
	return parts
}

func (a *Agent) du(path string) int64 {
	out, err := a.Run("du", "--summarize", "--bytes", "--one-file-system", path)
	if err != nil && len(out) == 0 {
		return 0
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return 0
	}
	n, _ := strconv.ParseInt(fields[0], 10, 64)
	return n
}

// storageCheck starts, pauses or resumes the check that both copies of the
// library match.
func (a *Agent) storageCheck(action *string) (Response, error) {
	lib, err := a.library()
	if err != nil {
		return nil, err
	}
	if lib == nil {
		return nil, refuse("There's no library to check yet.")
	}
	// The check takes hours on a full library, so it runs as its own unit
	// (design §7), not as the agent's child: restarting the agent or Box
	// Desktop doesn't stop it. Pausing cancels it; resuming carries on from
	// where it stopped.
	run := func(btrfs ...string) []string {
		// When it's over, the watcher tells the admins (watch.go).
		return append([]string{"--quiet", "--collect", "--unit=" + checkUnit,
			"--description=Check the library for damage",
			"--property=ExecStopPost=" + agentPath + " watch", "--", "btrfs"}, btrfs...)
	}
	var runErr error
	switch {
	case action != nil && *action == "start":
		_, runErr = a.Run("systemd-run", run("scrub", "start", "-B", lib.Path)...)
	case action != nil && *action == "pause":
		_, runErr = a.Run("btrfs", "scrub", "cancel", lib.Path)
	case action != nil && *action == "resume":
		_, runErr = a.Run("systemd-run", run("scrub", "resume", "-B", lib.Path)...)
	default:
		return nil, refuse("Start, pause or resume the check?")
	}
	if runErr != nil {
		return nil, runErr
	}
	return Response{"check": *action}, nil
}
