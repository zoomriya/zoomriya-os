// The box's own lists and hardware for the desktop's pages: places and
// languages, System Info's extra facts, the sound, the screen, and a picture
// from a USB stick made the box's background.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// frameHost: the box's name as this request named it, where its apps are.
func (s *Server) frameHost(r *http.Request) string {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return host
}

// applyScreen puts an admin's keyboard and mouse on the box's screen.
func (s *Server) applyScreen(prefs Prefs) {
	if layout, _ := prefs["keyboard"].(string); layout != "" {
		s.Hardware.Keyboard(layout)
	}
	s.Hardware.LeftHanded(prefs["handed"] == "left")
	mouse, _ := prefs["mouse"].(map[string]any)
	speed, _ := mouse["speed"].(float64)
	accel, _ := mouse["accel"].(bool)
	natural, _ := mouse["natural"].(bool)
	s.Hardware.Pointer(speed, accel, natural)
}

// resetScreen: the box's own keyboard and a right-handed mouse, for the next one.
func (s *Server) resetScreen() {
	s.Hardware.LeftHanded(false)
	if answer, p := s.Agent.Ask("box-desktop", "keyboard", nil); p == nil {
		layout, _ := answer["layout"].(string)
		if layout == "" {
			layout = "us"
		}
		s.Hardware.Keyboard(layout)
	}
}

func inputChanged(before, after Prefs) bool {
	a, _ := json.Marshal([]any{before["keyboard"], before["handed"], before["mouse"]})
	b, _ := json.Marshal([]any{after["keyboard"], after["handed"], after["mouse"]})
	return string(a) != string(b)
}

func (s *Server) lang(r *http.Request) string {
	if l := r.URL.Query().Get("lang"); l != "" {
		return Ours(l)
	}
	return s.Places.BoxLanguage()
}

// places: the languages, the countries and the time zones.
func (s *Server) places(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	writeJSON(w, http.StatusOK, map[string]any{
		"languages": Languages, "regions": s.Places.Regions(isoLang(s.lang(r))), "time_zones": s.Places.TimeZones(),
	})
}

// isoLang: iso-codes' catalogue for one of our languages (zh_CN is zh_CN there too).
func isoLang(lang string) string { return lang }

func (s *Server) subdivisions(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	region := r.URL.Query().Get("region")
	if len(region) != 2 || strings.ToUpper(region) != region {
		writeProblem(w, problem(http.StatusBadRequest, "bad_request"))
		return
	}
	writeJSON(w, http.StatusOK, s.Places.Subdivisions(region, isoLang(s.lang(r))))
}

func (s *Server) zonesIn(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	writeJSON(w, http.StatusOK, s.Places.ZonesIn(r.URL.Query().Get("region")))
}

func (s *Server) locale(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	q := r.URL.Query()
	writeJSON(w, http.StatusOK, map[string]string{"locale": s.Places.LocaleFor(q.Get("language"), q.Get("region"))})
}

// info: what the desktop knows of the box without the agent.
func (s *Server) info(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	writeJSON(w, http.StatusOK, map[string]any{
		"os": s.Places.OSName(), "processor": s.Places.Processor(), "sound": s.Hardware.HasSound(),
		"language": s.Places.BoxLanguage(), "app_base": s.Cfg.AppBase,
	})
}

// --- Sound ------------------------------------------------------------------

type soundSide struct {
	Devices []Device `json:"devices"`
	Volume  float64  `json:"volume"`
	Muted   bool     `json:"muted"`
}

func (s *Server) soundGet(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	if !s.Hardware.HasSound() {
		writeJSON(w, http.StatusOK, map[string]any{"sound": false})
		return
	}
	side := func(output bool) soundSide {
		v, m := s.Hardware.Volume(output)
		devices := s.Hardware.Devices(output)
		if devices == nil {
			devices = []Device{}
		}
		return soundSide{devices, v, m}
	}
	writeJSON(w, http.StatusOK, map[string]any{"sound": true, "output": side(true), "input": side(false)})
}

func (s *Server) soundPost(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	var b struct {
		Action string  `json:"action"` // use, volume, mute, test
		Device string  `json:"device"` // output or input
		ID     int     `json:"id"`
		Level  float64 `json:"level"`
		Muted  bool    `json:"muted"`
	}
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	output := b.Device != "input"
	switch b.Action {
	case "use":
		known := false
		for _, d := range s.Hardware.Devices(output) {
			known = known || d.ID == b.ID
		}
		if !known {
			writeProblem(w, problem(http.StatusBadRequest, "bad_request"))
			return
		}
		s.Hardware.Use(b.ID)
	case "volume":
		s.Hardware.SetVolume(output, b.Level)
	case "mute":
		s.Hardware.SetMuted(output, b.Muted)
	case "test":
		s.Hardware.PlayTest()
	default:
		writeProblem(w, problem(http.StatusBadRequest, "bad_request"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// soundLevel: the microphone's level as it hears, while Sound is in front.
func (s *Server) soundLevel(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeProblem(w, problem(http.StatusInternalServerError, "no_stream"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	se.Stream(1)
	defer se.Stream(-1)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		select {
		case <-se.Done:
			cancel()
		case <-ctx.Done():
		}
	}()
	levels := make(chan float64, 4)
	go s.Hardware.Listen(ctx, func(v float64) {
		select {
		case levels <- v:
		default:
		}
	})
	io.WriteString(w, "retry: 3000\n\n")
	flusher.Flush()
	for {
		select {
		case <-ctx.Done():
			return
		case v := <-levels:
			fmt.Fprintf(w, "data: %.3f\n\n", v)
			flusher.Flush()
		}
	}
}

// --- The box's screen --------------------------------------------------------

func (s *Server) displayGet(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	outputs := s.Hardware.Outputs()
	if outputs == nil {
		outputs = []Output{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"outputs": outputs})
}

var transforms = map[string]bool{"normal": true, "90": true, "180": true, "270": true}

// displayPost tries a screen setting at once; the agent's display-set keeps it.
func (s *Server) displayPost(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	var b struct {
		Name      string  `json:"name"`
		Mode      Mode    `json:"mode"`
		Scale     float64 `json:"scale"`
		Transform string  `json:"transform"`
	}
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	var out *Output
	for _, o := range s.Hardware.Outputs() {
		if o.Name == b.Name {
			out = &o
		}
	}
	known := false
	if out != nil {
		for _, m := range out.Modes {
			known = known || m == b.Mode
		}
	}
	if !known || !transforms[b.Transform] || b.Scale < 1 || b.Scale > 2 {
		writeProblem(w, problem(http.StatusBadRequest, "bad_request"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": s.Hardware.Apply(b.Name, b.Mode, b.Scale, b.Transform)})
}

// screenPower: the box's screen turns off when no one's using it, and on at a
// touch. Only the box's own screen asks.
func (s *Server) screenPower(w http.ResponseWriter, r *http.Request, l Listener, _ *Session) {
	if l != ListenerScreen {
		writeProblem(w, problem(http.StatusForbidden, "at_the_box"))
		return
	}
	var b struct{ On bool }
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": s.Hardware.Power(b.On)})
}

// --- The family's own background --------------------------------------------

// boxPictureSave: a picture on a USB stick, as the agent made it a JPEG,
// kept as the box's own background (never the file on the stick).
func (s *Server) boxPictureSave(w http.ResponseWriter, r *http.Request, l Listener, se *Session) {
	var b struct{ Device, Name string }
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	fields := map[string]json.RawMessage{}
	for k, v := range map[string]string{"action": "read", "device": b.Device, "name": b.Name} {
		fields[k], _ = json.Marshal(v)
	}
	answer, p := s.Agent.Ask(se.Login().Member.Username, "pictures", fields)
	if p != nil {
		writeProblem(w, p)
		return
	}
	jpeg, err := base64.StdEncoding.DecodeString(asString(answer["jpeg"]))
	if err == nil {
		err = s.Store.SavePicture(jpeg)
	}
	if err != nil {
		writeProblem(w, problem(http.StatusUnprocessableEntity, "picture_not_kept"))
		return
	}
	box, err := s.Store.SaveBox(map[string]any{"background": "own"})
	if err != nil {
		writeProblem(w, problem(http.StatusInternalServerError, "not_saved"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"box": box, "at": strconv.FormatInt(time.Now().Unix(), 10)})
}
