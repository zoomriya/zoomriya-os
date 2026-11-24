// held and held-release (design §6.1): the packages a restore took back, held
// at their restored version so the next night's security fixes don't install
// the same versions again. restore-snapshot writes the list into the restored
// OS; `zoomriya-agent release-holds`, before each night's security fixes, lets
// a hold go once Debian's security archive has a newer version than the one
// taken back, and an admin can let one go in Box Desktop.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// heldFile has one line per held package: its name, the version the restore
// kept, and the version it took back. On @, so restoring further back or
// Reset the system clears it.
const heldFile = "var/lib/zoomriya-base/held-after-restore"

type Held struct {
	Package   string `json:"package"`
	Version   string `json:"version"`
	TakenBack string `json:"taken_back"`
}

func (a *Agent) heldPackages() []Held {
	list := []Held{}
	for _, line := range strings.Split(a.readTrimmed(heldFile), "\n") {
		if f := strings.Fields(line); len(f) == 3 {
			list = append(list, Held{Package: f[0], Version: f[1], TakenBack: f[2]})
		}
	}
	return list
}

func (a *Agent) held() (Response, error) {
	return Response{"held": a.heldPackages()}, nil
}

// heldRelease lets one held package update again, as an admin asks.
func (a *Agent) heldRelease(pkg *string) (Response, error) {
	if pkg == nil {
		return nil, refuse("Which package? None was given.")
	}
	for _, h := range a.heldPackages() {
		if h.Package == *pkg {
			if err := a.release(h.Package); err != nil {
				return nil, err
			}
			return Response{"released": h.Package}, nil
		}
	}
	return nil, refuse("%q isn't held back.", *pkg)
}

// release unholds a package and takes it off the list.
func (a *Agent) release(pkg string) error {
	if _, err := a.Run("apt-mark", "unhold", pkg); err != nil {
		return err
	}
	var keep []string
	for _, h := range a.heldPackages() {
		if h.Package != pkg {
			keep = append(keep, strings.Join([]string{h.Package, h.Version, h.TakenBack}, " "))
		}
	}
	path := a.path(heldFile)
	if len(keep) == 0 {
		return os.Remove(path)
	}
	temp := filepath.Join(filepath.Dir(path), ".held-after-restore.new")
	if err := os.WriteFile(temp, []byte(strings.Join(keep, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

// releaseFixed lets go of each hold Debian's security archive now has a newer
// version for than the one taken back, and says which.
func (a *Agent) releaseFixed() ([]string, error) {
	var released []string
	for _, h := range a.heldPackages() {
		out, err := a.Run("apt-cache", "madison", h.Package)
		if err != nil {
			return released, err
		}
		for _, line := range strings.Split(string(out), "\n") {
			f := strings.Split(line, "|")
			if len(f) < 3 || !strings.Contains(f[2], "-security/") {
				continue
			}
			if compareVersions(strings.TrimSpace(f[1]), h.TakenBack) > 0 {
				if err := a.release(h.Package); err != nil {
					return released, err
				}
				released = append(released, h.Package)
				break
			}
		}
	}
	return released, nil
}

// releaseHolds is `zoomriya-agent release-holds`, before each night's
// security fixes.
func releaseHolds() int {
	released, err := NewAgent().releaseFixed()
	for _, pkg := range released {
		log.Printf("%s has a newer security fix; no longer held back", pkg)
	}
	if err != nil {
		log.Print(err)
		return 1
	}
	return 0
}

// compareVersions orders two Debian versions as dpkg does: below zero when a
// is older, zero when they're the same, above zero when a is newer.
func compareVersions(a, b string) int {
	ea, ua, ra := splitVersion(a)
	eb, ub, rb := splitVersion(b)
	if ea != eb {
		return ea - eb
	}
	if c := compareParts(ua, ub); c != 0 {
		return c
	}
	return compareParts(ra, rb)
}

// splitVersion splits [epoch:]upstream[-revision].
func splitVersion(v string) (epoch int, upstream, revision string) {
	if before, after, found := strings.Cut(v, ":"); found {
		epoch, _ = strconv.Atoi(before)
		v = after
	}
	if i := strings.LastIndexByte(v, '-'); i >= 0 {
		return epoch, v[:i], v[i+1:]
	}
	return epoch, v, ""
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// order is dpkg's: digits and the end come first, then letters, then
// everything else; a tilde before even the end.
func order(s string) int {
	if s == "" {
		return 0
	}
	c := s[0]
	switch {
	case isDigit(c):
		return 0
	case c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z':
		return int(c)
	case c == '~':
		return -1
	}
	return int(c) + 256
}

// compareParts is dpkg's verrevcmp: runs of non-digits by order, then runs of
// digits by value.
func compareParts(a, b string) int {
	for a != "" || b != "" {
		for a != "" && !isDigit(a[0]) || b != "" && !isDigit(b[0]) {
			if oa, ob := order(a), order(b); oa != ob {
				return oa - ob
			}
			if a != "" {
				a = a[1:]
			}
			if b != "" {
				b = b[1:]
			}
		}
		a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
		first := 0
		for a != "" && isDigit(a[0]) && b != "" && isDigit(b[0]) {
			if first == 0 {
				first = int(a[0]) - int(b[0])
			}
			a, b = a[1:], b[1:]
		}
		if a != "" && isDigit(a[0]) {
			return 1
		}
		if b != "" && isDigit(b[0]) {
			return -1
		}
		if first != 0 {
			return first
		}
	}
	return 0
}
