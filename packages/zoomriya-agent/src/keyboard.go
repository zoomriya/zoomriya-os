// keyboard and keyboard-set (box-desktop.md §4, Control Panel's Keyboard):
// the layout of a keyboard plugged into the box itself, for its screen (the
// console, the recovery steps), as Debian keeps it: /etc/default/keyboard,
// applied by console-setup.
package main

import (
	"os"
	"strings"
)

const keyboardFile = "etc/default/keyboard"

// Layout is an XKB layout, with its variant after a colon when it has one.
type Layout struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// The layouts offered, in the order shown.
var keyboardLayouts = []Layout{
	{"us", "English (US)"},
	{"us:intl", "English (US, international)"},
	{"gb", "English (UK)"},
	{"fr", "French"},
	{"be", "French (Belgium)"},
	{"ca", "French (Canada)"},
	{"ch:fr", "French (Switzerland)"},
	{"de", "German"},
	{"ch", "German (Switzerland)"},
	{"es", "Spanish"},
	{"latam", "Spanish (Latin America)"},
	{"it", "Italian"},
	{"pt", "Portuguese"},
	{"br", "Portuguese (Brazil)"},
	{"nl", "Dutch"},
	{"dk", "Danish"},
	{"no", "Norwegian"},
	{"se", "Swedish"},
	{"fi", "Finnish"},
	{"pl", "Polish"},
	{"cz", "Czech"},
	{"hu", "Hungarian"},
	{"tr", "Turkish"},
}

// keyboardValues reads /etc/default/keyboard's KEY="value" lines.
func (a *Agent) keyboardValues() map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(a.readTrimmed(keyboardFile), "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if found && !strings.HasPrefix(key, "#") {
			values[key] = strings.Trim(value, `"`)
		}
	}
	return values
}

func (a *Agent) keyboard() (Response, error) {
	v := a.keyboardValues()
	id := v["XKBLAYOUT"]
	if id == "" {
		id = "us"
	}
	if v["XKBVARIANT"] != "" {
		id += ":" + v["XKBVARIANT"]
	}
	return Response{"layout": id, "layouts": keyboardLayouts}, nil
}

func (a *Agent) keyboardSet(layout *string) (Response, error) {
	if layout == nil {
		return nil, refuse("Which keyboard layout? None was given.")
	}
	known := false
	for _, l := range keyboardLayouts {
		known = known || l.ID == *layout
	}
	if !known {
		return nil, refuse("The box doesn't offer a keyboard layout called %q.", *layout)
	}
	name, variant, _ := strings.Cut(*layout, ":")

	// Debian's file, with the layout and variant changed and the rest kept.
	lines := []string{}
	seen := map[string]bool{}
	for _, line := range strings.Split(a.readTrimmed(keyboardFile), "\n") {
		key, _, _ := strings.Cut(strings.TrimSpace(line), "=")
		switch key {
		case "XKBLAYOUT":
			line = `XKBLAYOUT="` + name + `"`
		case "XKBVARIANT":
			line = `XKBVARIANT="` + variant + `"`
		}
		seen[key] = true
		if line != "" || len(lines) > 0 {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		lines = []string{"# KEYBOARD CONFIGURATION FILE", "", "# Consult the keyboard(5) manual page.", "", `XKBMODEL="pc105"`}
	}
	if !seen["XKBLAYOUT"] {
		lines = append(lines, `XKBLAYOUT="`+name+`"`)
	}
	if !seen["XKBVARIANT"] {
		lines = append(lines, `XKBVARIANT="`+variant+`"`)
	}
	if !seen["XKBOPTIONS"] {
		lines = append(lines, `XKBOPTIONS=""`)
	}
	if !seen["BACKSPACE"] {
		lines = append(lines, `BACKSPACE="guess"`)
	}
	file := a.path(keyboardFile)
	if err := os.WriteFile(file+".new", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(file+".new", file); err != nil {
		return nil, err
	}

	// Kept for the next start, and in use on the box's screen now when it has
	// one.
	if _, err := a.Run("setupcon", "--force", "--save-only", "--keyboard-only"); err != nil {
		return nil, err
	}
	_, _ = a.Run("systemctl", "restart", "keyboard-setup.service")
	return Response{"layout": *layout}, nil
}
