// The box's own hardware (box-desktop.md §6.2): its speakers and microphone
// (PipeWire: wpctl, pw-dump, pw-play, pw-record) and its screen, keyboard
// and mouse (sway: swaymsg), as the native desktop's sound.py, display.py
// and zoomriya_setup/screen.py changed them. Whichever browser asks, they
// change the box: the box's screen runs in Box Desktop's own account
// (box-desktop-screen), so its sound server and sway are this account's.
package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Device is a speaker or a microphone.
type Device struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	On   bool   `json:"on"` // the one the box uses
}

// Mode is a screen's resolution and refresh rate (mHz, as sway counts it).
type Mode struct {
	Width   int `json:"width"`
	Height  int `json:"height"`
	Refresh int `json:"refresh"`
}

// Output is a screen plugged into the box.
type Output struct {
	Name      string  `json:"name"`
	Title     string  `json:"title"`
	Modes     []Mode  `json:"modes"`
	Mode      Mode    `json:"mode"`
	Scale     float64 `json:"scale"`
	Transform string  `json:"transform"`
}

// Hardware is what Box Desktop changes on the box; the preview's is a
// sample (FakeHardware).
type Hardware interface {
	HasSound() bool
	Devices(output bool) []Device
	Use(id int)
	Volume(output bool) (float64, bool)
	SetVolume(output bool, level float64)
	SetMuted(output bool, muted bool)
	PlayTest()
	// Listen calls level (0 to 1) about ten times a second until ctx ends.
	Listen(ctx context.Context, level func(float64))
	Outputs() []Output
	Apply(name string, mode Mode, scale float64, transform string) bool
	Power(on bool) bool
	Pointer(speed float64, accel, natural bool) bool
	Keyboard(layout string) bool
	LeftHanded(left bool) bool
}

// BoxHardware runs the box's own programs, in this account's session.
type BoxHardware struct {
	// Runtime is this account's XDG_RUNTIME_DIR, where PipeWire's and
	// sway's sockets are.
	Runtime string
	// Root is "/" on the box (the sound cards' list).
	Root string
}

func NewBoxHardware() *BoxHardware {
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		runtime = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	return &BoxHardware{Runtime: runtime, Root: "/"}
}

func (h *BoxHardware) env() []string {
	env := append(os.Environ(), "XDG_RUNTIME_DIR="+h.Runtime)
	// sway's socket: the newest in the runtime folder (sway-ipc.<uid>.<pid>.sock).
	socks, _ := filepath.Glob(filepath.Join(h.Runtime, "sway-ipc.*.sock"))
	sort.Slice(socks, func(i, j int) bool {
		a, _ := os.Stat(socks[i])
		b, _ := os.Stat(socks[j])
		return a != nil && b != nil && a.ModTime().After(b.ModTime())
	})
	if len(socks) > 0 {
		env = append(env, "SWAYSOCK="+socks[0])
	}
	return env
}

func (h *BoxHardware) run(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = h.env()
	return cmd.Output()
}

func (h *BoxHardware) swaymsg(args ...string) bool {
	for _, e := range h.env() {
		if strings.HasPrefix(e, "SWAYSOCK=") {
			_, err := h.run("swaymsg", args...)
			return err == nil
		}
	}
	return false
}

// HasSound: a sound card the kernel found; the Sound tools show only then.
func (h *BoxHardware) HasSound() bool {
	raw, err := os.ReadFile(filepath.Join(h.Root, "proc/asound/cards"))
	return err == nil && !strings.Contains(string(raw), "no soundcards")
}

func sink(output bool) string {
	if output {
		return "@DEFAULT_AUDIO_SINK@"
	}
	return "@DEFAULT_AUDIO_SOURCE@"
}

// shortName: a device's name as the sound system gives it, shorter.
func shortName(name string) string {
	return strings.ReplaceAll(name, "HDMI / DisplayPort", "HDMI/DP")
}

func (h *BoxHardware) Devices(output bool) []Device {
	wanted, key := "Audio/Source", "default.audio.source"
	if output {
		wanted, key = "Audio/Sink", "default.audio.sink"
	}
	raw, err := h.run("pw-dump")
	if err != nil {
		return nil
	}
	var objects []struct {
		ID       int    `json:"id"`
		Type     string `json:"type"`
		Props    map[string]any
		Info     struct{ Props map[string]any } `json:"info"`
		Metadata []struct {
			Key   string `json:"key"`
			Value any    `json:"value"`
		} `json:"metadata"`
	}
	if json.Unmarshal(raw, &objects) != nil {
		return nil
	}
	chosen := ""
	for _, o := range objects {
		if o.Type == "PipeWire:Interface:Metadata" && o.Props["metadata.name"] == "default" {
			for _, m := range o.Metadata {
				if v, ok := m.Value.(map[string]any); m.Key == key && ok {
					chosen, _ = v["name"].(string)
				}
			}
		}
	}
	var found []Device
	for _, o := range objects {
		if o.Type != "PipeWire:Interface:Node" || o.Info.Props["media.class"] != wanted {
			continue
		}
		name, _ := o.Info.Props["node.description"].(string)
		node, _ := o.Info.Props["node.name"].(string)
		if name == "" {
			name = node
		}
		found = append(found, Device{o.ID, shortName(name), node == chosen})
	}
	return found
}

func (h *BoxHardware) Use(id int) { h.run("wpctl", "set-default", strconv.Itoa(id)) }

func (h *BoxHardware) Volume(output bool) (float64, bool) {
	out, _ := h.run("wpctl", "get-volume", sink(output))
	// "Volume: 0.40" or "Volume: 0.40 [MUTED]"
	parts := strings.Fields(string(out))
	level := 0.0
	if len(parts) > 1 {
		level, _ = strconv.ParseFloat(parts[1], 64)
	}
	return level, strings.Contains(string(out), "[MUTED]")
}

func (h *BoxHardware) SetVolume(output bool, level float64) {
	h.run("wpctl", "set-volume", "-l", "1.0", sink(output), strconv.FormatFloat(clamp(level, 0, 1), 'f', 2, 64))
}

func (h *BoxHardware) SetMuted(output bool, muted bool) {
	v := "0"
	if muted {
		v = "1"
	}
	h.run("wpctl", "set-mute", sink(output), v)
}

func clamp(v, low, high float64) float64 { return math.Max(low, math.Min(high, v)) }

// chime is the test sound: a short two-note chime, made here, so no sound
// files are needed (sound.py's play_test).
func chime() []byte {
	const rate = 48000
	var samples []int16
	for _, note := range []struct{ freq, length float64 }{{660, 0.18}, {880, 0.28}} {
		n := int(rate * note.length)
		for i := 0; i < n; i++ {
			fade := math.Min(1, (rate*note.length-float64(i))/(rate*0.05))
			samples = append(samples, int16(9000*fade*math.Sin(2*math.Pi*note.freq*float64(i)/rate)))
		}
	}
	var wav []byte
	le := binary.LittleEndian
	data := len(samples) * 2
	wav = append(wav, "RIFF"...)
	wav = le.AppendUint32(wav, uint32(36+data))
	wav = append(wav, "WAVEfmt "...)
	wav = le.AppendUint32(wav, 16)
	wav = le.AppendUint16(wav, 1) // PCM
	wav = le.AppendUint16(wav, 1) // mono
	wav = le.AppendUint32(wav, rate)
	wav = le.AppendUint32(wav, rate*2)
	wav = le.AppendUint16(wav, 2)
	wav = le.AppendUint16(wav, 16)
	wav = append(wav, "data"...)
	wav = le.AppendUint32(wav, uint32(data))
	for _, s := range samples {
		wav = le.AppendUint16(wav, uint16(s))
	}
	return wav
}

func (h *BoxHardware) PlayTest() {
	go func() {
		// The service's own /tmp (PrivateTmp): /run is read-only to it.
		f, err := os.CreateTemp("", "box-desktop-test-*.wav")
		if err != nil {
			return
		}
		f.Write(chime())
		f.Close()
		defer os.Remove(f.Name())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "pw-play", f.Name())
		cmd.Env = h.env()
		cmd.Run()
	}()
}

func (h *BoxHardware) Listen(ctx context.Context, level func(float64)) {
	cmd := exec.CommandContext(ctx, "pw-record", "--rate", "16000", "--channels", "1", "--format", "s16", "-")
	cmd.Env = h.env()
	out, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return
	}
	defer cmd.Wait()
	reader := bufio.NewReaderSize(out, 3200)
	chunk := make([]byte, 3200) // a tenth of a second
	for {
		n, err := ioReadFull(reader, chunk)
		if n > 0 {
			peak := 0.0
			for i := 0; i+1 < n; i += 2 {
				v := math.Abs(float64(int16(binary.LittleEndian.Uint16(chunk[i:])))) / 32768
				peak = math.Max(peak, v)
			}
			level(math.Min(1, peak*1.6))
		}
		if err != nil {
			return
		}
	}
}

func ioReadFull(r *bufio.Reader, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		m, err := r.Read(buf[n:])
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (h *BoxHardware) Outputs() []Output {
	if !h.hasSway() {
		return nil
	}
	raw, err := h.run("swaymsg", "-t", "get_outputs", "-r")
	if err != nil {
		return nil
	}
	var listed []struct {
		Name        string  `json:"name"`
		Make        string  `json:"make"`
		Model       string  `json:"model"`
		Active      bool    `json:"active"`
		Modes       []Mode  `json:"modes"`
		CurrentMode Mode    `json:"current_mode"`
		Scale       float64 `json:"scale"`
		Transform   string  `json:"transform"`
	}
	if json.Unmarshal(raw, &listed) != nil {
		return nil
	}
	var found []Output
	for _, o := range listed {
		if !o.Active {
			continue
		}
		var title []string
		for _, p := range []string{o.Make, o.Model} {
			if p != "" && p != "Unknown" {
				title = append(title, p)
			}
		}
		out := Output{Name: o.Name, Title: strings.Join(title, " "), Modes: o.Modes, Mode: o.CurrentMode,
			Scale: o.Scale, Transform: o.Transform}
		if out.Title == "" {
			out.Title = o.Name
		}
		if len(out.Modes) == 0 {
			out.Modes = []Mode{o.CurrentMode}
		}
		if out.Scale == 0 {
			out.Scale = 1
		}
		if out.Transform == "" {
			out.Transform = "normal"
		}
		found = append(found, out)
	}
	return found
}

func (h *BoxHardware) hasSway() bool {
	for _, e := range h.env() {
		if strings.HasPrefix(e, "SWAYSOCK=") {
			return true
		}
	}
	return false
}

func (h *BoxHardware) Apply(name string, mode Mode, scale float64, transform string) bool {
	return h.swaymsg("output", name, "mode", fmt.Sprintf("%dx%d@%.3fHz", mode.Width, mode.Height, float64(mode.Refresh)/1000),
		"scale", strconv.FormatFloat(scale, 'g', -1, 64), "transform", transform)
}

// Power: the monitor on, or off while no one's using it; the box runs on.
func (h *BoxHardware) Power(on bool) bool {
	state := "off"
	if on {
		state = "on"
	}
	return h.swaymsg("output", "*", "power", state)
}

// Pointer: speed from -1 (slow) to 1 (fast); accel: faster as the mouse
// moves faster; natural: scrolling moves the page, as on a phone.
func (h *BoxHardware) Pointer(speed float64, accel, natural bool) bool {
	profile, scroll := "flat", "disabled"
	if accel {
		profile = "adaptive"
	}
	if natural {
		scroll = "enabled"
	}
	return h.swaymsg("input", "type:pointer", "accel_speed", strconv.FormatFloat(clamp(speed, -1, 1), 'f', 2, 64)) &&
		h.swaymsg("input", "type:pointer", "accel_profile", profile) &&
		h.swaymsg("input", "type:pointer", "natural_scroll", scroll)
}

// Keyboard: a layout as the agent lists it: "fr", or "ch:fr" with its variant.
func (h *BoxHardware) Keyboard(layout string) bool {
	name, variant, _ := strings.Cut(layout, ":")
	if variant == "" {
		variant = `""`
	}
	return h.swaymsg("input", "type:keyboard", "xkb_layout", name) &&
		h.swaymsg("input", "type:keyboard", "xkb_variant", variant)
}

func (h *BoxHardware) LeftHanded(left bool) bool {
	v := "disabled"
	if left {
		v = "enabled"
	}
	return h.swaymsg("input", "type:pointer", "left_handed", v)
}

// FakeHardware is the preview's sample box: a sound card, one screen, and
// nothing on this computer changes (fake.py's FakeSound and FakeDisplay).
type FakeHardware struct {
	mu      sync.Mutex
	volumes map[bool]float64
	muted   map[bool]bool
	chosen  map[bool]int
	output  Output
}

func NewFakeHardware() *FakeHardware {
	var modes []Mode
	for _, m := range []struct {
		w, h  int
		rates []int
	}{{2560, 1440, []int{59951, 74971}}, {1920, 1080, []int{60000, 59940, 50000}}, {1680, 1050, []int{59883}},
		{1280, 720, []int{60000, 50000}}, {1024, 768, []int{60004}}} {
		for _, r := range m.rates {
			modes = append(modes, Mode{m.w, m.h, r})
		}
	}
	return &FakeHardware{
		volumes: map[bool]float64{true: 0.6, false: 0.8},
		muted:   map[bool]bool{},
		chosen:  map[bool]int{true: 41, false: 51},
		output:  Output{"HDMI-A-1", "Dell U2724D", modes, modes[0], 1, "normal"},
	}
}

func (f *FakeHardware) HasSound() bool { return true }
func (f *FakeHardware) Devices(output bool) []Device {
	f.mu.Lock()
	defer f.mu.Unlock()
	// A mini PC rarely has a microphone of its own: these are plugged in.
	list := []Device{{41, "HDMI/DP, built-in audio", false}, {42, "Headphones, built-in audio", false}}
	if !output {
		list = []Device{{51, "USB Microphone", false}, {52, "Headset microphone, built-in audio", false}}
	}
	for i := range list {
		list[i].On = list[i].ID == f.chosen[output]
	}
	return list
}
func (f *FakeHardware) Use(id int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chosen[id < 50] = id
}
func (f *FakeHardware) Volume(output bool) (float64, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.volumes[output], f.muted[output]
}
func (f *FakeHardware) SetVolume(output bool, level float64) {
	f.mu.Lock()
	f.volumes[output] = clamp(level, 0, 1)
	f.mu.Unlock()
}
func (f *FakeHardware) SetMuted(output bool, muted bool) {
	f.mu.Lock()
	f.muted[output] = muted
	f.mu.Unlock()
}
func (f *FakeHardware) PlayTest() {}
func (f *FakeHardware) Listen(ctx context.Context, level func(float64)) {
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for x := 0.0; ; x += 0.4 {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			level(math.Abs(math.Sin(x)) * 0.7)
		}
	}
}
func (f *FakeHardware) Outputs() []Output {
	f.mu.Lock()
	defer f.mu.Unlock()
	return []Output{f.output}
}
func (f *FakeHardware) Apply(name string, mode Mode, scale float64, transform string) bool {
	f.mu.Lock()
	f.output.Mode, f.output.Scale, f.output.Transform = mode, scale, transform
	f.mu.Unlock()
	return true
}
func (f *FakeHardware) Power(bool) bool                  { return true }
func (f *FakeHardware) Pointer(float64, bool, bool) bool { return true }
func (f *FakeHardware) Keyboard(string) bool             { return true }
func (f *FakeHardware) LeftHanded(bool) bool             { return true }
