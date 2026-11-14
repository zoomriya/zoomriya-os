package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLanguage(t *testing.T) {
	agent, box := newFakeBox(t)
	box.write("usr/share/i18n/SUPPORTED", "en_CA.UTF-8 UTF-8\nen_CA ISO-8859-1\nfr_CA.UTF-8 UTF-8\nta_LK UTF-8\nde_DE.UTF-8 UTF-8\n")
	box.write("etc/locale.gen", "# en_CA.UTF-8 UTF-8\n# fr_CA.UTF-8 UTF-8\n")
	box.write("etc/default/locale", "LANG=C.UTF-8\n")
	if resp, _ := agent.Handle(Request{Op: "setup-status"}); resp["language"] != "" {
		t.Errorf("a language before one was chosen: %v", resp["language"])
	}
	if _, err := agent.Handle(Request{Op: "language-set", Locale: str("fr_CA.UTF-8")}); err != nil {
		t.Fatal(err)
	}
	gen, _ := os.ReadFile(filepath.Join(box.root, "etc/locale.gen"))
	if string(gen) != "# en_CA.UTF-8 UTF-8\nfr_CA.UTF-8 UTF-8\n" {
		t.Errorf("locale.gen: %q", gen)
	}
	if !ranCommand(box, "locale-gen") || !ranCommand(box, "update-locale LANG=fr_CA.UTF-8") {
		t.Errorf("ran %v", box.ran)
	}

	// One not in locale.gen at all is added; one already there stays once.
	box.write("etc/locale.gen", "fr_CA.UTF-8 UTF-8\n")
	for range 2 {
		if _, err := agent.Handle(Request{Op: "language-set", Locale: str("en_CA.UTF-8")}); err != nil {
			t.Fatal(err)
		}
	}
	if gen, _ := os.ReadFile(filepath.Join(box.root, "etc/locale.gen")); string(gen) != "fr_CA.UTF-8 UTF-8\nen_CA.UTF-8 UTF-8\n" {
		t.Errorf("locale.gen: %q", gen)
	}

	box.write("etc/default/locale", "LANG=ta_LK\n")
	if resp, _ := agent.Handle(Request{Op: "setup-status"}); resp["language"] != "ta_LK" {
		t.Errorf("got %v", resp["language"])
	}
	for _, locale := range []string{"", "en_CA", "xx_YY.UTF-8", "fr_CA UTF-8", "../etc", "fr_CA.UTF-8\n"} {
		if _, err := agent.Handle(Request{Op: "language-set", Locale: str(locale)}); err == nil {
			t.Errorf("%q accepted", locale)
		}
	}

	// English and French only for now: the box's other languages are refused,
	// even ones Debian can build, and nothing is changed.
	box.ran = nil
	for _, hidden := range []string{"ta_LK", "de_DE.UTF-8", "zh_CN.UTF-8", "es_ES.UTF-8", "ja_JP.UTF-8"} {
		_, err := agent.Handle(Request{Op: "language-set", Locale: str(hidden)})
		if err == nil || !strings.Contains(err.Error(), "English and French") {
			t.Errorf("%s: %v", hidden, err)
		}
	}
	if len(box.ran) != 0 {
		t.Errorf("a hidden language ran %v", box.ran)
	}
}
