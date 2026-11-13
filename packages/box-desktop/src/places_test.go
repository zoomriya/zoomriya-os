package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlacesFromTheBoxFiles(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("usr/share/zoneinfo/iso3166.tab", "# comment\nCA\tCanada\nLK\tSri Lanka\nFR\tFrance\n")
	write("usr/share/iso-codes/json/iso_3166-1.json", `{"3166-1":[{"alpha_2":"CA","name":"Canada"},{"alpha_2":"FR","name":"France"}]}`)
	write("usr/share/iso-codes/json/iso_3166-2.json", `{"3166-2":[{"code":"CA-ON","name":"Ontario"},{"code":"CA-BC","name":"British Columbia"},{"code":"FR-75","name":"Paris","parent":"IDF"}]}`)
	write("usr/share/i18n/SUPPORTED", "ta_IN UTF-8\nta_LK UTF-8\nfr_CA.UTF-8 UTF-8\nfr_FR.UTF-8 UTF-8\nfr_FR ISO-8859-1\nC.UTF-8 UTF-8\n")
	write("usr/share/zoneinfo/America/Toronto", "TZif2...")
	write("usr/share/zoneinfo/Etc/UTC", "TZif2...")
	write("usr/share/zoneinfo/posixrules", "TZif2...")
	write("etc/default/locale", "LANG=\"fr_CA.UTF-8\"\n")
	p := &Places{Root: root}

	regions := p.Regions("en")
	if len(regions) != 3 || regions[0].Name != "Canada" || regions[2].Name != "Sri Lanka" {
		t.Errorf("regions: %v", regions)
	}
	if subs := p.Subdivisions("CA", "en"); len(subs) != 2 || subs[0].Name != "British Columbia" {
		t.Errorf("subdivisions: %v", subs)
	}
	if subs := p.Subdivisions("FR", "en"); len(subs) != 0 {
		t.Errorf("France's departments are a level below: %v", subs)
	}
	if z := p.TimeZones(); len(z) != 2 || z[0] != "America/Toronto" || z[1] != "UTC" {
		t.Errorf("zones: %v", z)
	}
	for _, c := range []struct{ lang, region, want string }{
		{"ta", "LK", "ta_LK"}, {"ta", "CA", "ta_IN"}, {"fr", "CA", "fr_CA.UTF-8"}, {"fr", "DE", "fr_FR.UTF-8"}, {"xx", "CA", ""},
	} {
		if got := p.LocaleFor(c.lang, c.region); got != c.want {
			t.Errorf("LocaleFor(%s, %s) = %q, want %q", c.lang, c.region, got, c.want)
		}
	}
	if got := p.BoxLanguage(); got != "fr" {
		t.Errorf("box language %q", got)
	}
	for in, want := range map[string]string{"zh_TW.UTF-8": "zh_CN", "pt_BR": "en", "ta_LK": "ta", "zh_CN": "zh_CN"} {
		if Ours(in) != want {
			t.Errorf("Ours(%s) = %s", in, Ours(in))
		}
	}
}

func TestReadMOOnThisMachine(t *testing.T) {
	path := "/usr/share/locale/de/LC_MESSAGES/iso_3166-1.mo"
	if _, err := os.Stat(path); err != nil {
		t.Skip("no iso-codes here")
	}
	if got := readMO(path)["Germany"]; got != "Deutschland" {
		t.Errorf("Germany in German: %q", got)
	}
}
