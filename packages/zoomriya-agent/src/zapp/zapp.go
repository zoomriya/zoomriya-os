// Package zapp is a ZOOMRIYA app package (design §6.9): one uncompressed tar
// file, <id>-<version>.zapp, that App Center installs from a USB stick.
//
// Its first entry is manifest.json and its second manifest.sig, an Ed25519
// signature of the manifest's exact bytes by ZOOMRIYA's app key. Every other
// entry is listed in the manifest with its SHA-256 and size: the app's image
// (a tar for `machinectl import-tar`) and its systemd units. A package that
// isn't signed by a key the box trusts, or that has anything not listed or not
// matching, is refused before anything is installed.
package zapp

import (
	"archive/tar"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	Format       = 1
	ManifestName = "manifest.json"
	SigName      = "manifest.sig"
	maxManifest  = 64 << 10
	maxSig       = 1 << 10
	maxUnit      = 64 << 10
	maxImage     = 64 << 30
)

var (
	idPattern      = regexp.MustCompile(`^[a-z][a-z0-9]{1,30}$`)
	versionPattern = regexp.MustCompile(`^[0-9]{1,6}(\.[0-9]{1,6}){0,3}(-[a-z0-9]{1,20})?$`)
	unitName       = regexp.MustCompile(`^zoomriya-([a-z0-9]+)-[a-z0-9-]{1,40}(@)?\.(service|timer|socket)$`)
	imageName      = regexp.MustCompile(`^image\.tar(\.(gz|xz))?$`)
	databaseName   = regexp.MustCompile(`^[a-z][a-z0-9_]{1,40}$`)
)

// ErrNotZoomriya: not signed by a key this box trusts.
var ErrNotZoomriya = errors.New("this isn't a ZOOMRIYA app: its signature doesn't match ZOOMRIYA's key")

type File struct {
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type Manifest struct {
	Format      int    `json:"format"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Publisher   string `json:"publisher"`
	Description string `json:"description"`
	// The lowest ZOOMRIYA OS version (os-release's VERSION_ID) it runs on.
	MinOS int `json:"min_os"`
	// The image, imported as zoomriya-<id>-<version>.
	Image string `json:"image"`
	// Its units, under units/; Start and Migrate name templates among them,
	// started with the version as their instance.
	Units   []string `json:"units"`
	Start   []string `json:"start"`
	Migrate string   `json:"migrate,omitempty"`
	// Data: it keeps the family's data in /data/apps/<id>; Databases: its
	// PostgreSQL databases. Both may be deleted when the app is removed.
	Data      bool            `json:"data"`
	Databases []string        `json:"databases"`
	Files     map[string]File `json:"files"`
}

// Validate checks everything the agent relies on: names, versions, and that
// the files listed are exactly the image and the units.
func (m *Manifest) Validate() error {
	if m.Format != Format {
		return fmt.Errorf("package format %d isn't one this box reads", m.Format)
	}
	if !idPattern.MatchString(m.ID) {
		return fmt.Errorf("app id %q isn't valid", m.ID)
	}
	if !versionPattern.MatchString(m.Version) {
		return fmt.Errorf("version %q isn't valid", m.Version)
	}
	if strings.TrimSpace(m.Name) == "" || len(m.Name) > 60 {
		return errors.New("the app has no name")
	}
	if !imageName.MatchString(m.Image) {
		return fmt.Errorf("image %q isn't valid", m.Image)
	}
	wanted := map[string]bool{m.Image: true}
	units := map[string]bool{}
	for _, u := range m.Units {
		base := strings.TrimPrefix(u, "units/")
		g := unitName.FindStringSubmatch(base)
		if !strings.HasPrefix(u, "units/") || g == nil || g[1] != m.ID {
			return fmt.Errorf("unit %q isn't named zoomriya-%s-…", u, m.ID)
		}
		units[base] = g[2] == "@"
		wanted[u] = true
	}
	templates := append([]string{}, m.Start...)
	if m.Migrate != "" {
		templates = append(templates, m.Migrate)
	}
	if len(m.Start) == 0 {
		return errors.New("the app starts nothing")
	}
	for _, t := range templates {
		if isTemplate, ok := units[t]; !ok || !isTemplate {
			return fmt.Errorf("%q isn't one of the app's template units", t)
		}
	}
	for _, db := range m.Databases {
		if !databaseName.MatchString(db) || (db != m.ID && !strings.HasPrefix(db, m.ID+"_")) {
			return fmt.Errorf("database %q isn't named %s or %s_…", db, m.ID, m.ID)
		}
	}
	if len(m.Files) != len(wanted) {
		return errors.New("the files listed aren't exactly the image and the units")
	}
	for name, f := range m.Files {
		if !wanted[name] || len(f.SHA256) != 64 || f.Bytes < 0 {
			return fmt.Errorf("file %q isn't expected", name)
		}
	}
	return nil
}

// Machine is the image's name: zoomriya-<id>-<version>.
func (m *Manifest) Machine() string { return Machine(m.ID, m.Version) }

func Machine(id, version string) string { return "zoomriya-" + id + "-" + version }

// ParseKeys reads trusted keys: "ed25519 <base64> [name]" per line.
func ParseKeys(text string) []ed25519.PublicKey {
	var keys []ed25519.PublicKey
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "ed25519" {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(fields[1])
		if err == nil && len(raw) == ed25519.PublicKeySize {
			keys = append(keys, ed25519.PublicKey(raw))
		}
	}
	return keys
}

func readEntry(r *tar.Reader, want string, max int64) ([]byte, error) {
	h, err := r.Next()
	if err != nil {
		return nil, fmt.Errorf("not an app package: %w", err)
	}
	if h.Name != want || h.Typeflag != tar.TypeReg || h.Size > max {
		return nil, fmt.Errorf("not an app package: %s isn't where it should be", want)
	}
	return io.ReadAll(io.LimitReader(r, max))
}

// ReadHead reads and checks the manifest and its signature: the first two
// entries. It returns the tar reader positioned after them.
func ReadHead(r io.Reader, keys []ed25519.PublicKey) (Manifest, *tar.Reader, error) {
	var m Manifest
	t := tar.NewReader(r)
	data, err := readEntry(t, ManifestName, maxManifest)
	if err != nil {
		return m, nil, err
	}
	sigText, err := readEntry(t, SigName, maxSig)
	if err != nil {
		return m, nil, err
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigText)))
	if err != nil {
		return m, nil, ErrNotZoomriya
	}
	signed := false
	for _, k := range keys {
		signed = signed || ed25519.Verify(k, data, sig)
	}
	if !signed {
		return m, nil, ErrNotZoomriya
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return m, nil, fmt.Errorf("the app's manifest isn't readable: %w", err)
	}
	if err := m.Validate(); err != nil {
		return m, nil, err
	}
	return m, t, nil
}

// Extract checks a whole package and writes its files under dir (dir/image…,
// dir/units/…). Nothing is left unchecked: a file not in the manifest, one
// that doesn't match, or one missing fails it.
func Extract(r io.Reader, keys []ed25519.PublicKey, dir string) (Manifest, error) {
	m, t, err := ReadHead(r, keys)
	if err != nil {
		return m, err
	}
	seen := map[string]bool{}
	for {
		h, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return m, fmt.Errorf("the package is damaged: %w", err)
		}
		want, listed := m.Files[h.Name]
		if !listed || seen[h.Name] || h.Typeflag != tar.TypeReg || h.Size != want.Bytes || path.Clean(h.Name) != h.Name {
			return m, fmt.Errorf("the package has %q, which its manifest doesn't list", h.Name)
		}
		limit := int64(maxUnit)
		if h.Name == m.Image {
			limit = maxImage
		}
		if h.Size > limit {
			return m, fmt.Errorf("%q is too big", h.Name)
		}
		seen[h.Name] = true
		out := filepath.Join(dir, filepath.FromSlash(h.Name))
		if err := os.MkdirAll(filepath.Dir(out), 0o700); err != nil {
			return m, err
		}
		f, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return m, err
		}
		sum := sha256.New()
		_, err = io.Copy(io.MultiWriter(f, sum), io.LimitReader(t, limit))
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return m, err
		}
		if hex.EncodeToString(sum.Sum(nil)) != want.SHA256 {
			return m, fmt.Errorf("%q doesn't match its manifest: the package is damaged or changed", h.Name)
		}
	}
	if len(seen) != len(m.Files) {
		return m, errors.New("the package is missing files its manifest lists")
	}
	return m, nil
}

// Pack writes a signed package from a manifest (its Files filled in here)
// and the files on disk: image is the image's path, units the units' paths.
func Pack(w io.Writer, m Manifest, key ed25519.PrivateKey, image string, units []string) error {
	m.Format = Format
	m.Files = map[string]File{}
	sources := map[string]string{m.Image: image}
	m.Units = nil
	for _, u := range units {
		name := "units/" + filepath.Base(u)
		m.Units = append(m.Units, name)
		sources[name] = u
	}
	sort.Strings(m.Units)
	for name, src := range sources {
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		sum := sha256.New()
		n, err := io.Copy(sum, f)
		f.Close()
		if err != nil {
			return err
		}
		m.Files[name] = File{SHA256: hex.EncodeToString(sum.Sum(nil)), Bytes: n}
	}
	if err := m.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(key, data)) + "\n"
	t := tar.NewWriter(w)
	put := func(name string, size int64, body io.Reader) error {
		if err := t.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: size, Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		_, err := io.Copy(t, body)
		return err
	}
	if err := put(ManifestName, int64(len(data)), bytes.NewReader(data)); err != nil {
		return err
	}
	if err := put(SigName, int64(len(sig)), strings.NewReader(sig)); err != nil {
		return err
	}
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		f, err := os.Open(sources[name])
		if err != nil {
			return err
		}
		err = put(name, m.Files[name].Bytes, f)
		f.Close()
		if err != nil {
			return err
		}
	}
	return t.Close()
}
