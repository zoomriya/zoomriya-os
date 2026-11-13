// resources (design §7): the box's CPU, memory, network and drive activity
// now, what each app and service is using, and its temperatures. Box
// Desktop's server asks every few seconds while someone has it open, and
// keeps the history.
package main

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Use is measured over this long.
const cpuSample = 500 * time.Millisecond

// How many apps and services are listed, busiest first.
const topServices = 15

type cpuTimes struct{ idle, total uint64 }

// cpu reads the "cpu" line of /proc/stat: time spent idle (idle and iowait)
// and in all.
func (a *Agent) cpu() (cpuTimes, int) {
	var t cpuTimes
	cores := 0
	for _, line := range strings.Split(a.readTrimmed("proc/stat"), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 || !strings.HasPrefix(fields[0], "cpu") {
			continue
		}
		if fields[0] != "cpu" {
			cores++
			continue
		}
		for i, f := range fields[1:] {
			n, _ := strconv.ParseUint(f, 10, 64)
			t.total += n
			if i == 3 || i == 4 { // idle, iowait
				t.idle += n
			}
		}
	}
	return t, cores
}

// meminfo reads one /proc/meminfo line, in bytes.
func (a *Agent) meminfo(key string) int64 {
	for _, line := range strings.Split(a.readTrimmed("proc/meminfo"), "\n") {
		if rest, found := strings.CutPrefix(line, key+":"); found {
			fields := strings.Fields(rest)
			if len(fields) > 0 {
				kb, _ := strconv.ParseInt(fields[0], 10, 64)
				return kb * 1024
			}
		}
	}
	return 0
}

// counters are what's grown between the two readings, by name.
type counters map[string][2]uint64

// netCounters: bytes in and out on each of the box's own ports.
func (a *Agent) netCounters() counters {
	c := counters{}
	entries, _ := os.ReadDir(a.path("sys/class/net"))
	for _, e := range entries {
		name := e.Name()
		if a.readTrimmed(filepath.Join("sys/class/net", name, "type")) != "1" || a.kindOf(name) == "" {
			continue
		}
		in, _ := strconv.ParseUint(a.readTrimmed(filepath.Join("sys/class/net", name, "statistics/rx_bytes")), 10, 64)
		out, _ := strconv.ParseUint(a.readTrimmed(filepath.Join("sys/class/net", name, "statistics/tx_bytes")), 10, 64)
		c[name] = [2]uint64{in, out}
	}
	return c
}

// diskCounters: bytes read and written on each drive (not its partitions,
// nor loop or mapped devices).
func (a *Agent) diskCounters() counters {
	c := counters{}
	for _, line := range strings.Split(a.readTrimmed("proc/diskstats"), "\n") {
		f := strings.Fields(line)
		if len(f) < 10 {
			continue
		}
		if _, err := os.Stat(a.path(filepath.Join("sys/block", f[2], "device"))); err != nil {
			continue
		}
		read, _ := strconv.ParseUint(f[5], 10, 64)
		written, _ := strconv.ParseUint(f[9], 10, 64)
		c[f[2]] = [2]uint64{read * 512, written * 512}
	}
	return c
}

// A service's or app's cgroup: CPU time used so far, and memory now.
type cgroup struct {
	name, unit, kind string
	usec             uint64
	memory           int64
}

// The OS's own services, in words.
var serviceNames = map[string]string{
	"zoomriya-agent.service":      "ZOOMRIYA agent",
	"box-desktop.service":         "Box Desktop",
	"box-desktop-screen.service":  "Box Desktop on the box's screen",
	"systemd-journald.service":    "System log",
	"systemd-networkd.service":    "Network",
	"systemd-resolved.service":    "Name lookups",
	"avahi-daemon.service":        "Found on the home network",
	"unattended-upgrades.service": "Security updates",
	"apt-daily-upgrade.service":   "Security updates",
	checkUnit + ".service":        "Checking the library",
	"zoomriya-aide-check.service": "File integrity check",
	"postgresql.service":          "Databases",
	"valkey-server.service":       "Cache",
	"caddy.service":               "Gateway",
}

// cgroups reads system.slice's services and machine.slice's apps.
func (a *Agent) cgroups() map[string]cgroup {
	groups := map[string]cgroup{}
	for _, slice := range []string{"system.slice", "machine.slice"} {
		entries, _ := os.ReadDir(a.path(filepath.Join("sys/fs/cgroup", slice)))
		for _, e := range entries {
			unit := e.Name()
			if !e.IsDir() || !(strings.HasSuffix(unit, ".service") || strings.HasSuffix(unit, ".scope")) {
				continue
			}
			dir := filepath.Join("sys/fs/cgroup", slice, unit)
			g := cgroup{unit: unit, kind: "system", name: serviceNames[unit]}
			if slice == "machine.slice" {
				g.kind = "app"
				g.name = strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(unit, "systemd-nspawn@"), "machine-"), ".service"), ".scope")
			}
			if g.name == "" {
				g.name = strings.TrimSuffix(unit, ".service")
			}
			for _, line := range strings.Split(a.readTrimmed(filepath.Join(dir, "cpu.stat")), "\n") {
				if v, found := strings.CutPrefix(line, "usage_usec "); found {
					g.usec, _ = strconv.ParseUint(v, 10, 64)
				}
			}
			g.memory, _ = strconv.ParseInt(a.readTrimmed(filepath.Join(dir, "memory.current")), 10, 64)
			groups[slice+"/"+unit] = g
		}
	}
	return groups
}

type LinkRate struct {
	Name string  `json:"name"`
	In   float64 `json:"in_bps"`
	Out  float64 `json:"out_bps"`
}

type DriveRate struct {
	Name  string  `json:"name"`
	Read  float64 `json:"read_bps"`
	Write float64 `json:"write_bps"`
}

type ServiceUse struct {
	Name        string  `json:"name"`
	Unit        string  `json:"unit"`
	Kind        string  `json:"kind"` // "app" or "system"
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryBytes int64   `json:"memory_bytes"`
}

type Sensor struct {
	Sensor  string  `json:"sensor"`
	Label   string  `json:"label"`
	Celsius float64 `json:"celsius,omitempty"`
	RPM     int     `json:"rpm,omitempty"`
}

func (a *Agent) resources() (Response, error) {
	before, cores := a.cpu()
	netBefore, diskBefore, groupsBefore := a.netCounters(), a.diskCounters(), a.cgroups()
	a.sleep(cpuSample)
	after, _ := a.cpu()
	netAfter, diskAfter, groupsAfter := a.netCounters(), a.diskCounters(), a.cgroups()

	seconds := cpuSample.Seconds()
	perSecond := func(now, then uint64) float64 {
		if now < then {
			return 0 // the counter started again
		}
		return math.Round(float64(now-then) / seconds)
	}

	percent := 0.0
	if total := after.total - before.total; total > 0 {
		percent = 100 * (1 - float64(after.idle-before.idle)/float64(total))
	}

	links := []LinkRate{}
	var in, out float64
	for name, now := range netAfter {
		then, ok := netBefore[name]
		if !ok {
			continue
		}
		r := LinkRate{Name: name, In: perSecond(now[0], then[0]), Out: perSecond(now[1], then[1])}
		in, out = in+r.In, out+r.Out
		links = append(links, r)
	}
	sort.Slice(links, func(i, j int) bool { return links[i].Name < links[j].Name })

	drives := []DriveRate{}
	var read, written float64
	for name, now := range diskAfter {
		then, ok := diskBefore[name]
		if !ok {
			continue
		}
		r := DriveRate{Name: name, Read: perSecond(now[0], then[0]), Write: perSecond(now[1], then[1])}
		read, written = read+r.Read, written+r.Write
		drives = append(drives, r)
	}
	sort.Slice(drives, func(i, j int) bool { return drives[i].Name < drives[j].Name })

	// A service's share of the whole box, as cpu_percent is.
	services := []ServiceUse{}
	capacity := float64(cores) * cpuSample.Seconds() * 1e6
	for key, now := range groupsAfter {
		use := ServiceUse{Name: now.name, Unit: now.unit, Kind: now.kind, MemoryBytes: now.memory}
		if then, ok := groupsBefore[key]; ok && capacity > 0 && now.usec >= then.usec {
			use.CPUPercent = math.Round(1000*float64(now.usec-then.usec)/capacity) / 10
		}
		services = append(services, use)
	}
	sort.Slice(services, func(i, j int) bool {
		if services[i].CPUPercent != services[j].CPUPercent {
			return services[i].CPUPercent > services[j].CPUPercent
		}
		return services[i].MemoryBytes > services[j].MemoryBytes
	})
	if len(services) > topServices {
		services = services[:topServices]
	}

	temperatures, fans := a.sensors()
	total := a.meminfo("MemTotal")
	available := a.meminfo("MemAvailable")
	swapTotal := a.meminfo("SwapTotal")
	return Response{
		"cpu_percent":         math.Round(percent*10) / 10,
		"cores":               cores,
		"load":                a.load(),
		"memory_total_bytes":  total,
		"memory_used_bytes":   total - available,
		"memory_cached_bytes": a.meminfo("Cached") + a.meminfo("Buffers"),
		"swap_total_bytes":    swapTotal,
		"swap_used_bytes":     swapTotal - a.meminfo("SwapFree"),
		"network":             Response{"in_bps": in, "out_bps": out, "links": links},
		"disks":               Response{"read_bps": read, "write_bps": written, "drives": drives},
		"services":            services,
		"temperatures":        temperatures,
		"fans":                fans,
	}, nil
}

// load is the 1, 5 and 15 minute load averages.
func (a *Agent) load() []float64 {
	load := []float64{}
	fields := strings.Fields(a.readTrimmed("proc/loadavg"))
	for i := 0; i < 3 && i < len(fields); i++ {
		v, _ := strconv.ParseFloat(fields[i], 64)
		load = append(load, v)
	}
	return load
}

// sensors reads the hardware monitors: the CPU's and board's temperatures,
// and the fans. Drives report theirs through disk-health.
func (a *Agent) sensors() ([]Sensor, []Sensor) {
	temperatures, fans := []Sensor{}, []Sensor{}
	monitors, _ := os.ReadDir(a.path("sys/class/hwmon"))
	for _, m := range monitors {
		dir := filepath.Join("sys/class/hwmon", m.Name())
		chip := a.readTrimmed(filepath.Join(dir, "name"))
		if chip == "nvme" || chip == "drivetemp" {
			continue // the drives', shown with the drives
		}
		files, _ := os.ReadDir(a.path(dir))
		for _, f := range files {
			name := f.Name()
			for _, kind := range []string{"temp", "fan"} {
				prefix, found := strings.CutSuffix(name, "_input")
				if !found || !strings.HasPrefix(prefix, kind) {
					continue
				}
				value, err := strconv.ParseInt(a.readTrimmed(filepath.Join(dir, name)), 10, 64)
				if err != nil {
					continue
				}
				label := a.readTrimmed(filepath.Join(dir, prefix+"_label"))
				if label == "" {
					label = prefix
				}
				if kind == "temp" {
					temperatures = append(temperatures, Sensor{Sensor: chip, Label: label, Celsius: float64(value) / 1000})
				} else {
					fans = append(fans, Sensor{Sensor: chip, Label: label, RPM: int(value)})
				}
			}
		}
	}
	return temperatures, fans
}

func (a *Agent) sleep(d time.Duration) {
	if a.Sleep != nil {
		a.Sleep(d)
		return
	}
	time.Sleep(d)
}
