package zapp

import (
	"archive/tar"
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func keyLine(pub ed25519.PublicKey) string {
	return "ed25519 " + base64.StdEncoding.EncodeToString(pub) + " test\n"
}

// Build writes a package for tests: an image and two units.
func build(t *testing.T, priv ed25519.PrivateKey, edit func(*Manifest)) []byte {
	t.Helper()
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	image := write("image.tar", "an image")
	api := write("zoomriya-notes-api@.service", "[Service]\nExecStart=/usr/bin/systemd-nspawn\n")
	migrate := write("zoomriya-notes-migrate@.service", "[Service]\nType=oneshot\n")
	m := Manifest{ID: "notes", Name: "Notes", Version: "1.2.0", Publisher: "ZOOMRIYA", MinOS: 1,
		Image: "image.tar", Start: []string{"zoomriya-notes-api@.service"}, Migrate: "zoomriya-notes-migrate@.service",
		Data: true, Databases: []string{"notes"}}
	if edit != nil {
		edit(&m)
	}
	var buf bytes.Buffer
	if err := Pack(&buf, m, priv, image, []string{api, migrate}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPackAndExtract(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	keys := ParseKeys("# ZOOMRIYA\n" + keyLine(pub))
	dir := t.TempDir()
	m, err := Extract(bytes.NewReader(build(t, priv, nil)), keys, dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "notes" || m.Machine() != "zoomriya-notes-1.2.0" || len(m.Units) != 2 {
		t.Errorf("manifest %+v", m)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "image.tar")); string(data) != "an image" {
		t.Errorf("image = %q", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "units", "zoomriya-notes-api@.service")); err != nil {
		t.Error(err)
	}
}

func TestOnlyZoomriyaKeys(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	for name, keys := range map[string][]ed25519.PublicKey{"another key": ParseKeys(keyLine(pub)), "no key": nil} {
		_, err := Extract(bytes.NewReader(build(t, other, nil)), keys, t.TempDir())
		if !errors.Is(err, ErrNotZoomriya) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// rewrite copies a package, changing what change returns for each entry.
func rewrite(t *testing.T, pkg []byte, change func(name string, body []byte) (string, []byte, bool)) []byte {
	var out bytes.Buffer
	r, w := tar.NewReader(bytes.NewReader(pkg)), tar.NewWriter(&out)
	for {
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		body, _ := io.ReadAll(r)
		name, body, keep := change(h.Name, body)
		if !keep {
			continue
		}
		_ = w.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
		_, _ = w.Write(body)
	}
	_ = w.Close()
	return out.Bytes()
}

func TestTamperedPackages(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	keys := ParseKeys(keyLine(pub))
	pkg := build(t, priv, nil)
	cases := map[string]func(string, []byte) (string, []byte, bool){
		"image changed": func(n string, b []byte) (string, []byte, bool) {
			if n == "image.tar" {
				return n, []byte("an imagf"), true
			}
			return n, b, true
		},
		"manifest changed": func(n string, b []byte) (string, []byte, bool) {
			if n == ManifestName {
				return n, bytes.Replace(b, []byte(`"Notes"`), []byte(`"Nots!"`), 1), true
			}
			return n, b, true
		},
		"unit missing": func(n string, b []byte) (string, []byte, bool) {
			return n, b, !strings.Contains(n, "migrate")
		},
		"extra file": func(n string, b []byte) (string, []byte, bool) {
			if strings.Contains(n, "migrate") {
				return "units/zoomriya-notes-evil@.service", b, true
			}
			return n, b, true
		},
		"path escape": func(n string, b []byte) (string, []byte, bool) {
			if n == "image.tar" {
				return "../image.tar", b, true
			}
			return n, b, true
		},
	}
	for name, change := range cases {
		if _, err := Extract(bytes.NewReader(rewrite(t, pkg, change)), keys, t.TempDir()); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestValidate(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	for name, edit := range map[string]func(*Manifest){
		"bad id":             func(m *Manifest) { m.ID = "Notes!" },
		"bad version":        func(m *Manifest) { m.Version = "1.2; rm -rf /" },
		"start not template": func(m *Manifest) { m.Start = []string{"zoomriya-other-api@.service"} },
		"other app's db":     func(m *Manifest) { m.Databases = []string{"photos"} },
		"no start":           func(m *Manifest) { m.Start = nil },
	} {
		func() {
			defer func() { _ = recover() }()
			var buf bytes.Buffer
			m := Manifest{ID: "notes", Name: "Notes", Version: "1.2.0", Image: "image.tar",
				Start: []string{"zoomriya-notes-api@.service"}}
			edit(&m)
			dir := t.TempDir()
			_ = os.WriteFile(filepath.Join(dir, "image.tar"), []byte("x"), 0o644)
			_ = os.WriteFile(filepath.Join(dir, "zoomriya-notes-api@.service"), []byte("x"), 0o644)
			if err := Pack(&buf, m, priv, filepath.Join(dir, "image.tar"),
				[]string{filepath.Join(dir, "zoomriya-notes-api@.service")}); err == nil {
				t.Errorf("%s: packed", name)
			}
		}()
	}
}
