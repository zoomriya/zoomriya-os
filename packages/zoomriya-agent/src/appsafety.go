// What keeps App Center's apps safe beyond the signature (design §6.9):
//
//   - No going back by stick. A package older than the newest release of that
//     app the box has ever had is refused, even after the app was removed, so
//     an old, signed release with a known hole can't be put back. Restore
//     previous version is the only way back, to a release the box kept.
//   - Revocation. zoomriya-base's app-revoked lists keys no longer trusted and
//     releases not to install or restore; OS updates bring it.
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"

	"zoomriya.com/os/zoomriya-agent/zapp"
)

const (
	appRevoked = "usr/share/zoomriya-base/app-revoked"
	// The newest release of each app the box has had, kept after a removal.
	appHighest = "var/lib/zoomriya-agent/app-highest.json"
)

type revocations struct {
	keys     map[string]bool   // base64 public keys
	releases map[string]bool   // "<id> <version>"
	below    map[string]string // id: every release before this one
}

// revoked reads app-revoked: "key <base64>", "release <id> <version>" and
// "below <id> <version>" lines; # starts a comment.
func (a *Agent) revoked() revocations {
	r := revocations{keys: map[string]bool{}, releases: map[string]bool{}, below: map[string]string{}}
	for _, line := range strings.Split(a.readTrimmed(appRevoked), "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		f := strings.Fields(line)
		switch {
		case len(f) == 2 && f[0] == "key":
			r.keys[f[1]] = true
		case len(f) == 3 && f[0] == "release":
			r.releases[f[1]+" "+f[2]] = true
		case len(f) == 3 && f[0] == "below":
			r.below[f[1]] = f[2]
		}
	}
	return r
}

// blocked: a release not to install or restore.
func (r revocations) blocked(id, version string) bool {
	if r.releases[id+" "+version] {
		return true
	}
	floor, ok := r.below[id]
	return ok && compareVersions(version, floor) < 0
}

// trustedKeys: app-keys, less the revoked ones.
func (a *Agent) trustedKeys() []ed25519.PublicKey {
	revoked := a.revoked().keys
	var keys []ed25519.PublicKey
	for _, k := range zapp.ParseKeys(a.readTrimmed(appKeys)) {
		if !revoked[base64.StdEncoding.EncodeToString(k)] {
			keys = append(keys, k)
		}
	}
	return keys
}

// highest: each app's newest release the box has had, the apps on it now
// included.
func (a *Agent) highest() map[string]string {
	high := map[string]string{}
	if data, err := os.ReadFile(a.path(appHighest)); err == nil {
		_ = json.Unmarshal(data, &high)
	}
	for id, app := range a.loadApps() {
		for v := range app.Releases {
			if compareVersions(v, high[id]) > 0 {
				high[id] = v
			}
		}
	}
	return high
}

func (a *Agent) noteHighest(id, version string) error {
	high := a.highest()
	if compareVersions(version, high[id]) > 0 {
		high[id] = version
	}
	data, _ := json.Marshal(high)
	return writeFileAtomic(a.path(appHighest), data, 0o600)
}

// stickRefusal: why a package can't come from a stick ("blocked" or "older",
// with the newest release the box has had), or "".
func (a *Agent) stickRefusal(m zapp.Manifest) (kind, newest, why string) {
	if a.revoked().blocked(m.ID, m.Version) {
		return "blocked", "", "Has a known problem. Use a newer version."
	}
	if h := a.highest()[m.ID]; h != "" && compareVersions(m.Version, h) < 0 {
		return "older", h, "Older than " + h + ", which this box has had."
	}
	return "", "", ""
}
