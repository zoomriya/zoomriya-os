// Each admin's own Box Desktop, kept between sign-ins (box-desktop.md §6.4,
// BD-4): the same files, format and checks as the native desktop's prefs.py,
// so an admin's settings carry over. One file per admin, named by a hash of
// their ZOOMRIYA ID member id, in /data/zoomriya/admin, readable by Box
// Desktop's account only; beside them the box's own (box.json, box.jpg).
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
)

const (
	DockMax      = 8
	ShortcutsMax = 8
	DefaultTheme = "midnight"
	// DefaultAccent and DefaultBackground: appearance.py's.
	DefaultAccent     = "blue"
	DefaultBackground = "smoke"
)

var (
	appID    = regexp.MustCompile(`^[a-z][a-z0-9]{1,30}$`)
	shortcut = regexp.MustCompile(`^(control|storage|apps|resources|logs|support|app:[a-z][a-z0-9]{1,30})$`)
	// When the screen locks, and turns off, with no one using it: minutes (0: never).
	lockAfter      = []int{0, 1, 2, 5, 10, 15, 30}
	screenOffAfter = []int{0, 5, 10, 15, 30, 60}
	// appearance.py's accents, pictures and colours, by key.
	accents     = []string{"blue", "teal", "green", "yellow", "orange", "red", "pink", "purple", "slate"}
	backgrounds = []string{"smoke", "slate", "dusk", "lakeside", "night", "ocean", "forest", "plum", "graphite", "own"}
)

// Prefs is one admin's settings, as JSON. Unknown keys are dropped.
type Prefs map[string]any

func defaultPrefs() Prefs {
	return Prefs{
		"version": 1,
		"theme":   DefaultTheme,
		// Control Panel › Appearance: the accent colour (the background is the box's).
		"accent": DefaultAccent,
		// The keyboard's layout on the box's screen ("" for the box's own) and the mouse.
		"keyboard":      "",
		"handed":        "right",
		"mouse":         map[string]any{"speed": 0.0, "accel": true, "natural": false},
		"windows":       []any{},
		"dock":          []any{},
		"shortcuts":     []any{},
		"notifications": map[string]any{"off": []any{}, "read_until": ""},
	}
}

type Store struct {
	Folder string
}

func (s *Store) path(memberID int) string {
	sum := sha256.Sum256([]byte(strconv.Itoa(memberID)))
	return filepath.Join(s.Folder, hex.EncodeToString(sum[:])+".json")
}

// Load gives the admin's settings, each checked; the default where one isn't right.
func (s *Store) Load(memberID int) Prefs {
	prefs := defaultPrefs()
	raw, err := os.ReadFile(s.path(memberID))
	if err != nil {
		return prefs
	}
	var saved map[string]any
	if json.Unmarshal(raw, &saved) != nil {
		return prefs
	}
	return checkPrefs(saved)
}

// checkPrefs keeps what's right of saved, over the defaults (prefs.py's load).
func checkPrefs(saved map[string]any) Prefs {
	prefs := defaultPrefs()
	for k, v := range saved {
		if _, ok := prefs[k]; ok {
			prefs[k] = v
		}
	}
	if t, _ := prefs["theme"].(string); t != "daylight" && t != "midnight" {
		prefs["theme"] = DefaultTheme
	}
	if a, _ := prefs["accent"].(string); !slices.Contains(accents, a) {
		prefs["accent"] = DefaultAccent
	}
	prefs["dock"] = uniqueMatching(prefs["dock"], appID, DockMax)
	prefs["shortcuts"] = uniqueMatching(prefs["shortcuts"], shortcut, ShortcutsMax)
	if h, _ := prefs["handed"].(string); h != "right" && h != "left" {
		prefs["handed"] = "right"
	}
	if _, ok := prefs["keyboard"].(string); !ok {
		prefs["keyboard"] = ""
	}
	mouse, _ := prefs["mouse"].(map[string]any)
	speed, ok := mouse["speed"].(float64)
	if !ok || math.IsNaN(speed) {
		speed = 0
	}
	accel, isBool := mouse["accel"].(bool)
	natural, _ := mouse["natural"].(bool)
	prefs["mouse"] = map[string]any{
		"speed":   math.Max(-1, math.Min(1, speed)),
		"accel":   !isBool || accel,
		"natural": natural,
	}
	if _, ok := prefs["windows"].([]any); !ok {
		prefs["windows"] = []any{}
	}
	if n, ok := prefs["notifications"].(map[string]any); !ok {
		prefs["notifications"] = map[string]any{"off": []any{}, "read_until": ""}
	} else {
		if _, ok := n["off"].([]any); !ok {
			n["off"] = []any{}
		}
		if _, ok := n["read_until"].(string); !ok {
			n["read_until"] = ""
		}
	}
	prefs["version"] = 1
	return prefs
}

func uniqueMatching(v any, re *regexp.Regexp, max int) []any {
	list, _ := v.([]any)
	out := []any{}
	seen := map[string]bool{}
	for _, item := range list {
		s, ok := item.(string)
		if !ok || !re.MatchString(s) || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
		if len(out) == max {
			break
		}
	}
	return out
}

// Save keeps the admin's settings, checked first.
func (s *Store) Save(memberID int, saved map[string]any) (Prefs, error) {
	prefs := checkPrefs(saved)
	raw, err := json.Marshal(prefs)
	if err != nil {
		return nil, err
	}
	return prefs, s.write(s.path(memberID), raw)
}

// Box is the box's own settings, the same for everyone (box.json): its
// background, and after how many minutes with no one using it the screen
// locks and turns off (0: never).
type Box struct {
	Background     string `json:"background"`
	LockAfter      int    `json:"lock_after"`
	ScreenOffAfter int    `json:"screen_off_after"`
}

func defaultBox() Box {
	return Box{Background: DefaultBackground, LockAfter: 5, ScreenOffAfter: 10}
}

func (s *Store) Box() Box {
	found := defaultBox()
	raw, err := os.ReadFile(filepath.Join(s.Folder, "box.json"))
	if err != nil {
		return found
	}
	var saved map[string]any
	if json.Unmarshal(raw, &saved) != nil {
		return found
	}
	if b, ok := saved["background"].(string); ok && slices.Contains(backgrounds, b) {
		found.Background = b
	}
	if n, ok := wholeNumber(saved["lock_after"]); ok && slices.Contains(lockAfter, n) {
		found.LockAfter = n
	}
	if n, ok := wholeNumber(saved["screen_off_after"]); ok && slices.Contains(screenOffAfter, n) {
		found.ScreenOffAfter = n
	}
	return found
}

func wholeNumber(v any) (int, bool) {
	f, ok := v.(float64)
	if !ok || f != math.Trunc(f) {
		return 0, false
	}
	return int(f), true
}

var errNotAllowed = errors.New("not one of the choices")

// SaveBox changes some of the box's settings; a value that isn't one of the
// choices changes nothing.
func (s *Store) SaveBox(changes map[string]any) (Box, error) {
	box := s.Box()
	for key, v := range changes {
		switch key {
		case "background":
			b, ok := v.(string)
			if !ok || !slices.Contains(backgrounds, b) {
				return box, errNotAllowed
			}
			box.Background = b
		case "lock_after", "screen_off_after":
			n, ok := wholeNumber(v)
			choices := lockAfter
			if key == "screen_off_after" {
				choices = screenOffAfter
			}
			if !ok || !slices.Contains(choices, n) {
				return box, errNotAllowed
			}
			if key == "lock_after" {
				box.LockAfter = n
			} else {
				box.ScreenOffAfter = n
			}
		default:
			return box, errNotAllowed
		}
	}
	raw, _ := json.Marshal(box)
	return box, s.write(filepath.Join(s.Folder, "box.json"), raw)
}

// Picture is the box's own picture: the JPEG the agent made from a stick's.
func (s *Store) Picture() string { return filepath.Join(s.Folder, "box.jpg") }

func (s *Store) SavePicture(jpeg []byte) error {
	if !bytes.HasPrefix(jpeg, []byte{0xff, 0xd8, 0xff}) || len(jpeg) > 32<<20 {
		return errors.New("not a JPEG")
	}
	return s.write(s.Picture(), jpeg)
}

// write: a new file, renamed into place, readable by Box Desktop's account only.
func (s *Store) write(path string, data []byte) error {
	if err := os.MkdirAll(s.Folder, 0o700); err != nil {
		return err
	}
	os.Chmod(s.Folder, 0o700)
	f, err := os.CreateTemp(s.Folder, ".new-")
	if err != nil {
		return err
	}
	temp := f.Name()
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(temp, 0o600)
	}
	if err == nil {
		err = os.Rename(temp, path)
	}
	if err != nil {
		os.Remove(temp)
	}
	return err
}
