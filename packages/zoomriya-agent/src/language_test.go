package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLanguage(t *testing.T) {
	agent, box := newFakeBox(t)
	box.write("usr/share/i18n/SUPPORTED", "en_CA.UTF-8 UTF-8\nen_CA ISO-8859-1\nta_LK UTF-8\n")
	box.write("etc/locale.gen", "# en_CA.UTF-8 UTF-8\n# ta_LK UTF-8\n")
	box.write("etc/default/locale", "LANG=C.UTF-8\n")
	if resp, _ := agent.Handle(Request{Op: "setup-status"}); resp["language"] != "" {
		t.Errorf("a language before one was chosen: %v", resp["language"])
	}
	if _, err := agent.Handle(Request{Op: "language-set", Locale: str("ta_LK")}); err != nil {
		t.Fatal(err)
	}
	gen, _ := os.ReadFile(filepath.Join(box.root, "etc/locale.gen"))
	if string(gen) != "# en_CA.UTF-8 UTF-8\nta_LK UTF-8\n" {
		t.Errorf("locale.gen: %q", gen)
	}
	if !ranCommand(box, "locale-gen") || !ranCommand(box, "update-locale LANG=ta_LK") {
		t.Errorf("ran %v", box.ran)
	}

	// One not in locale.gen at all is added; one already there stays once.
	box.write("etc/locale.gen", "ta_LK UTF-8\n")
	for range 2 {
		if _, err := agent.Handle(Request{Op: "language-set", Locale: str("en_CA.UTF-8")}); err != nil {
			t.Fatal(err)
		}
	}
	if gen, _ := os.ReadFile(filepath.Join(box.root, "etc/locale.gen")); string(gen) != "ta_LK UTF-8\nen_CA.UTF-8 UTF-8\n" {
		t.Errorf("locale.gen: %q", gen)
	}

	box.write("etc/default/locale", "LANG=ta_LK\n")
	if resp, _ := agent.Handle(Request{Op: "setup-status"}); resp["language"] != "ta_LK" {
		t.Errorf("got %v", resp["language"])
	}
	for _, locale := range []string{"", "en_CA", "xx_YY.UTF-8", "ta_LK UTF-8", "../etc", "ta_LK\n"} {
		if _, err := agent.Handle(Request{Op: "language-set", Locale: str(locale)}); err == nil {
			t.Errorf("%q accepted", locale)
		}
	}
}
