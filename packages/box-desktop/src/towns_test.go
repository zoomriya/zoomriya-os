package main

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestTownsByWhatWasTyped(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer
	z := gzip.NewWriter(&buf)
	z.Write([]byte("# id\tname\tascii\tcountry\tadmin1\tlat\tlon\tpopulation\ttimezone\n" +
		"6167865\tToronto\tToronto\tCA\tOntario\t43.6533\t-79.3839\t2794356\tAmerica/Toronto\n" +
		"6077243\tMontréal\tMontreal\tCA\tQuebec\t45.5088\t-73.5878\t1762949\tAmerica/Toronto\n" +
		"6325494\tQuébec\tQuebec\tCA\tQuebec\t46.8123\t-71.2145\t531902\tAmerica/Toronto\n" +
		"6138517\tSaint-Jérôme\tSaint-Jerome\tCA\tQuebec\t45.7804\t-74.0036\t78439\tAmerica/Toronto\n" +
		"5128581\tNew York City\tNew York City\tUS\tNew York\t40.7143\t-74.0060\t8804190\tAmerica/New_York\n"))
	z.Close()
	os.MkdirAll(filepath.Join(root, "usr/share/zoomriya"), 0o755)
	os.WriteFile(filepath.Join(root, townsFile), buf.Bytes(), 0o644)
	towns = townCache{}
	p := &Places{Root: root}
	names := func(list []TownChoice) []string {
		out := []string{}
		for _, t := range list {
			out = append(out, t.Name)
		}
		return out
	}
	for typed, want := range map[string][]string{
		"mont":   {"Montréal"},
		"Montré": {"Montréal"},
		"que":    {"Québec"},
		"jer":    {"Saint-Jérôme"}, // a word of the name
		"new":    {},               // another country's
		"":       {"Toronto", "Montréal", "Québec", "Saint-Jérôme"},
	} {
		got := names(p.Towns("CA", typed))
		if len(got) != len(want) {
			t.Errorf("%q: %v", typed, got)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%q: %v", typed, got)
			}
		}
	}
	if got := p.Towns("US", "new"); len(got) != 1 || got[0].Admin1 != "New York" {
		t.Errorf("%v", got)
	}
}
