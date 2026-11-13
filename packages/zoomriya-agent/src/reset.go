// factory-reset (design §4): Reset the system makes the OS as it left the
// factory; Erase everything also empties the family's data, the databases,
// the app images and the logs, at the next start (zoomriya-base's erase), and
// needs the setup code from the sticker.
package main

import (
	"crypto/subtle"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	setupCodeFile = "efi/zoomriya/setup-code"
	// Made by the box itself, not the factory: there's no sticker with it.
	setupCodeMadeHere = "efi/zoomriya/setup-code-made-here"
)

func (a *Agent) factoryReset(level, setupCode *string) (Response, error) {
	if level == nil {
		return nil, refuse("Which reset? Reset the system, or erase everything.")
	}
	// A reset holds nothing back: it installs the latest fixes after.
	args := []string{"--reset"}
	switch *level {
	case "system":
	case "everything":
		if setupCode == nil || !a.setupCodeMatches(*setupCode) {
			return nil, refuse("That isn't this box's setup code. It's on the sticker.")
		}
		args = append(args, "--erase-everything")
	default:
		return nil, refuse("There's no reset called %q.", *level)
	}

	factory := 0
	points, err := a.restorePoints()
	if err != nil {
		return nil, err
	}
	for _, p := range points {
		if p.Kind == "factory" {
			factory = p.Number
		}
	}
	if factory == 0 {
		return nil, refuse("This box has no factory restore point to reset to.")
	}

	if _, err := a.Run(restoreSnapshot, append(args, strconv.Itoa(factory))...); err != nil {
		return nil, err
	}
	// No SSH keys left, and SSH off (design §7.1).
	if err := a.accessReset(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(a.path(resetPending)), 0o755); err == nil {
		_ = os.WriteFile(a.path(restorePending), []byte(strconv.Itoa(factory)+"\n"), 0o644)
		_ = os.WriteFile(a.path(resetPending), []byte(*level+"\n"), 0o644)
	}
	return Response{"reset": *level, "restart_needed": true}, nil
}

// setupCodeMatches compares in constant time, ignoring case, spaces and the
// dash, as people type it.
func (a *Agent) setupCodeMatches(typed string) bool {
	want := normaliseCode(a.readTrimmed(setupCodeFile))
	got := normaliseCode(typed)
	return want != "" && subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}

func normaliseCode(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' {
			return -1
		}
		return r
	}, strings.ToUpper(strings.TrimSpace(s)))
}
