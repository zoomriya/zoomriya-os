// The box's one language model: exactly one copy, in llama.cpp's
// llama-server, answering everyone (Complete, Stream) and, when its vision
// file is installed beside it, looking at pictures (Describe).
//
// The admin picks from the .gguf files in ModelsDir; the choice is saved in
// StateDir and used from then on, including after a restart. With no saved
// choice, DefaultModel. Switching stops the old model before starting the
// new one (one model in memory at a time) and goes back to the old one if
// the new one fails. If llama-server stops by itself, it is started again.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Unavailable: no model to answer with right now (switching, loading, not
// installed). The apps show its words to the member as they are.
type Unavailable struct{ Message string }

func (u *Unavailable) Error() string { return u.Message }

func unavailable(message string) error { return &Unavailable{message} }

// The words the apps already show for each case.
var (
	errSwitching = unavailable("Myira is switching models. Try again in a moment.")
	errLoading   = unavailable("Myira's model is still loading. Try again in a moment.")
	errFailed    = unavailable("Myira's model couldn't be loaded. An admin can pick another.")
	errSwitched  = unavailable("Myira switched models. Ask again.")
	errNoVision  = unavailable("This model has no vision file installed")
)

// KnownModels: friendly names for the models we ship; any other .gguf is
// listed by its file name.
var KnownModels = map[string][2]string{
	"gemma-4-E2B-it-Q4_K_M.gguf": {"Gemma 4 E2B", "Faster replies and less memory. Good for everyday questions."},
	"gemma-4-E4B-it-Q4_K_M.gguf": {"Gemma 4 E4B", "Smarter and better at multi-step requests, but slower and needs more memory."},
}

// DefaultStop: Gemma 4 ends a turn with <turn|>.
var DefaultStop = []string{"<turn|>", "<eos>"}

var gemmaName = regexp.MustCompile(`^(gemma-4-[A-Za-z0-9]+-it)-.+\.gguf$`)

// VisionFor: the vision file that goes with a Gemma 4 GGUF, when it's
// installed beside it.
func VisionFor(modelPath string) string {
	m := gemmaName.FindStringSubmatch(filepath.Base(modelPath))
	if m == nil {
		return ""
	}
	vision := filepath.Join(filepath.Dir(modelPath), "mmproj-"+m[1]+"-BF16.gguf")
	if fileExists(vision) {
		return vision
	}
	return ""
}

// Running is a started llama-server with its model loaded.
type Running struct {
	// Base: its address; Key: the key it asks for, known only to this service.
	Base, Key string
	File      string
	Sees      bool
	// PID: llama-server's process, for the memory it takes (0: not known).
	PID int
	// Done is closed when it has stopped, for whatever reason.
	Done <-chan struct{}
	Stop func()
}

// Starter starts llama-server with a model (and its vision file, or "")
// and returns once the model is loaded and answering.
type Starter func(model, vision string) (*Running, error)

type Engine struct {
	cfg   Config
	start Starter
	Turns *Turns
	// client: to llama-server, on loopback; answers take as long as they take.
	client *http.Client

	swap    sync.Mutex // held while a model is started or replaced
	mu      sync.Mutex
	current *Running
	status  string // idle, loading, failed
	failure string
	target  string
	closed  bool
}

func NewEngine(cfg Config, start Starter) *Engine {
	return &Engine{cfg: cfg, start: start, Turns: NewTurns(), client: &http.Client{}, status: "idle"}
}

// ---- the installed models and the admin's choice -----------------------------------

type ModelOption struct {
	File        string `json:"file"`
	Label       string `json:"label"`
	Description string `json:"description"`
	SizeBytes   int64  `json:"size_bytes"`
}

func (e *Engine) AvailableModels() []ModelOption {
	paths, _ := filepath.Glob(filepath.Join(e.cfg.ModelsDir, "*.gguf"))
	sort.Strings(paths)
	options := []ModelOption{}
	for _, path := range paths {
		name := filepath.Base(path)
		if strings.HasPrefix(name, "mmproj-") {
			continue // a vision file, not a chat model
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		label, description := strings.TrimSuffix(name, ".gguf"), "Custom model."
		if known, ok := KnownModels[name]; ok {
			label, description = known[0], known[1]
		}
		options = append(options, ModelOption{name, label, description, info.Size()})
	}
	return options
}

func (e *Engine) choiceFile() string { return filepath.Join(e.cfg.StateDir, "model.json") }

func (e *Engine) SavedChoice() string {
	raw, err := os.ReadFile(e.choiceFile())
	if err != nil {
		return ""
	}
	var saved struct {
		File string `json:"file"`
	}
	if json.Unmarshal(raw, &saved) != nil {
		return ""
	}
	return saved.File
}

func (e *Engine) saveChoice(file string) error {
	if err := os.MkdirAll(e.cfg.StateDir, 0o700); err != nil {
		return err
	}
	raw, _ := json.Marshal(map[string]string{"file": file})
	temporary := e.choiceFile() + ".tmp"
	if err := os.WriteFile(temporary, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, e.choiceFile())
}

// SelectedFile: the saved choice while it is still installed, else the default.
func (e *Engine) SelectedFile() string {
	if choice := e.SavedChoice(); choice != "" && validFile(choice) && fileExists(filepath.Join(e.cfg.ModelsDir, choice)) {
		return choice
	}
	return e.cfg.DefaultModel
}

// validFile: a file name in ModelsDir, never a path out of it.
func validFile(name string) bool {
	return name != "" && name == filepath.Base(name) && name != "." && name != ".." && strings.HasSuffix(name, ".gguf")
}

func labelFor(file string) any {
	if file == "" {
		return nil
	}
	if known, ok := KnownModels[file]; ok {
		return known[0]
	}
	return strings.TrimSuffix(file, ".gguf")
}

// ---- loading, switching ---------------------------------------------------------------

func (e *Engine) LoadedFile() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.current == nil {
		return ""
	}
	return e.current.File
}

func (e *Engine) CanSee() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.current != nil && e.current.Sees
}

func (e *Engine) switching() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.status == "loading"
}

// replace stops the current model first, then starts file; if that fails,
// starts the old one again.
func (e *Engine) replace(file string) error {
	e.swap.Lock()
	defer e.swap.Unlock()
	e.mu.Lock()
	old := e.current
	e.mu.Unlock()
	oldFile := ""
	if old != nil {
		oldFile = old.File
		// Let an answer under way finish first.
		release, _ := e.Turns.Take(context.Background(), Interactive)
		e.mu.Lock()
		e.current = nil
		e.mu.Unlock()
		release()
		old.Stop()
	}
	running, err := e.launch(file)
	if err != nil && oldFile != "" {
		if back, again := e.launch(oldFile); again == nil {
			e.use(back)
		}
		return err
	}
	if err != nil {
		return err
	}
	e.use(running)
	return nil
}

func (e *Engine) launch(file string) (*Running, error) {
	path := filepath.Join(e.cfg.ModelsDir, file)
	if !fileExists(path) {
		return nil, fmt.Errorf("model file not found at %s", path)
	}
	running, err := e.start(path, VisionFor(path))
	if err != nil {
		return nil, err
	}
	running.File = file
	return running, nil
}

// use makes running the model, and starts it again if it stops by itself.
func (e *Engine) use(running *Running) {
	e.mu.Lock()
	e.current = running
	e.mu.Unlock()
	go func() {
		<-running.Done
		e.mu.Lock()
		unexpected := e.current == running && !e.closed
		if unexpected {
			e.current = nil
		}
		e.mu.Unlock()
		if unexpected {
			log.Printf("llama-server stopped by itself; starting %s again", running.File)
			time.Sleep(3 * time.Second)
			if err := e.StartLoading(running.File); err != nil {
				log.Printf("couldn't start it again: %v", err)
			}
		}
	}()
}

// StartLoading loads file from ModelsDir in the background.
func (e *Engine) StartLoading(file string) error {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return errors.New("shutting down")
	}
	if e.status == "loading" {
		e.mu.Unlock()
		return unavailable("A model is already loading")
	}
	e.status, e.failure, e.target = "loading", "", file
	e.mu.Unlock()
	go func() {
		err := e.replace(file)
		e.mu.Lock()
		defer e.mu.Unlock()
		if err != nil {
			log.Printf("the model %s didn't load: %v", file, err)
			e.status, e.failure, e.target = "failed", err.Error(), file
			return
		}
		e.status, e.failure, e.target = "idle", "", ""
	}()
	return nil
}

var errNotInstalled = errors.New("That model isn't installed on this box")

// Choose: the admin's choice, saved, then loaded unless it already is.
func (e *Engine) Choose(file string) error {
	installed := false
	for _, option := range e.AvailableModels() {
		installed = installed || option.File == file
	}
	if !installed || !validFile(file) {
		return errNotInstalled
	}
	if e.switching() {
		return unavailable("A model is already loading. Try again shortly")
	}
	if err := e.saveChoice(file); err != nil {
		return err
	}
	if e.LoadedFile() != file {
		return e.StartLoading(file)
	}
	return nil
}

// Close stops llama-server for good.
func (e *Engine) Close() {
	e.mu.Lock()
	e.closed = true
	running := e.current
	e.current = nil
	e.mu.Unlock()
	if running != nil {
		running.Stop()
	}
}

func (e *Engine) ready() (*Running, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.status == "loading" {
		return nil, errSwitching
	}
	if e.current == nil {
		if e.status == "failed" {
			return nil, errFailed
		}
		return nil, errLoading
	}
	return e.current, nil
}

func (e *Engine) StatusPayload() map[string]any {
	e.mu.Lock()
	status, failure, target := e.status, e.failure, e.target
	e.mu.Unlock()
	selected := target
	if selected == "" {
		selected = e.SelectedFile()
	}
	loaded := e.LoadedFile()
	var loadedAny, failureAny any
	if loaded != "" {
		loadedAny = loaded
	}
	if failure != "" {
		failureAny = failure
	}
	return map[string]any{
		"options":                e.AvailableModels(),
		"selected":               selected,
		"loaded":                 loadedAny,
		"loaded_label":           labelFor(loaded),
		"sees":                   e.CanSee(),
		"status":                 status,
		"error":                  failureAny,
		"memory_available_bytes": availableMemory(),
		// What Myira's model takes now: llama-server's resident memory and
		// what the graphics driver holds for it in the box's memory (the
		// model's weights and its caches); null when none is loaded.
		"memory_used_bytes": e.usedMemory(),
	}
}

func (e *Engine) usedMemory() any {
	e.mu.Lock()
	pid := 0
	if e.current != nil {
		pid = e.current.PID
	}
	e.mu.Unlock()
	if pid <= 0 {
		return nil
	}
	resident, ok := residentBytes(fmt.Sprintf("/proc/%d/status", pid)).(int64)
	if !ok {
		return nil
	}
	return resident + gpuSystemBytes(fmt.Sprintf("/proc/%d/fdinfo", pid))
}

// gpuSystemBytes: what the graphics driver holds in the box's own memory for
// the process (its fdinfo, per DRM client, once each). With the model on the
// box's built-in graphics chip, its weights are there and not in VmRSS.
// A graphics card's own memory (VRAM) isn't the box's, so it isn't counted.
func gpuSystemBytes(fdinfo string) int64 {
	entries, err := os.ReadDir(fdinfo)
	if err != nil {
		return 0
	}
	seen := map[string]bool{}
	var total int64
	for _, entry := range entries {
		raw, err := os.ReadFile(filepath.Join(fdinfo, entry.Name()))
		if err != nil || !strings.Contains(string(raw), "drm-driver:") {
			continue
		}
		var client string
		var bytes int64
		for _, line := range strings.Split(string(raw), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			key := strings.TrimSuffix(fields[0], ":")
			switch {
			case key == "drm-client-id":
				client = fields[1]
			// Intel (i915, xe): drm-resident-system0; AMD: drm-memory-gtt
			// and drm-memory-cpu. Never vram or stolen memory.
			case strings.HasPrefix(key, "drm-resident-system"),
				key == "drm-memory-gtt", key == "drm-memory-cpu":
				bytes += sizeBytes(fields[1:])
			}
		}
		if client == "" || seen[client] {
			continue
		}
		seen[client] = true
		total += bytes
	}
	return total
}

// sizeBytes: "4072660 KiB" (or MiB, GiB, or plain bytes) in bytes.
func sizeBytes(fields []string) int64 {
	n, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0
	}
	if len(fields) > 1 {
		switch fields[1] {
		case "KiB", "kB":
			n *= 1 << 10
		case "MiB":
			n *= 1 << 20
		case "GiB":
			n *= 1 << 30
		}
	}
	return n
}

// residentBytes: VmRSS from a /proc status file, in bytes; nil when unreadable.
func residentBytes(path string) any {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 && fields[0] == "VmRSS:" {
			if kb, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				return kb * 1024
			}
		}
	}
	return nil
}

func availableMemory() any {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 && fields[0] == "MemAvailable:" {
			if kb, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
				return kb * 1024
			}
		}
	}
	return nil
}

// ---- answering ------------------------------------------------------------------------

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Options struct {
	Temperature float64
	MaxTokens   int
	Stop        []string
	Priority    string
	// Schema: the answer must be JSON fitting it (llama.cpp's grammar).
	Schema map[string]any
}

func chatBody(messages any, o Options, stream bool) map[string]any {
	stop := o.Stop
	if len(stop) == 0 {
		stop = DefaultStop
	}
	body := map[string]any{
		"messages":     messages,
		"temperature":  o.Temperature,
		"max_tokens":   o.MaxTokens,
		"stop":         stop,
		"stream":       stream,
		"cache_prompt": true,
		// An answer, not the model's thinking first.
		"chat_template_kwargs": map[string]any{"enable_thinking": false},
	}
	if o.Schema != nil {
		body["response_format"] = map[string]any{"type": "json_object", "schema": o.Schema}
	}
	return body
}

// ask sends one request to llama-server, holding the model's turn.
func (e *Engine) ask(ctx context.Context, running *Running, body map[string]any) (*http.Response, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, running.Base+"/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+running.Key)
	response, err := e.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errLoading
	}
	if response.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		response.Body.Close()
		if response.StatusCode == http.StatusServiceUnavailable {
			return nil, errLoading
		}
		return nil, fmt.Errorf("llama-server: %s: %s", response.Status, strings.TrimSpace(string(detail)))
	}
	return response, nil
}

// turn waits for the model and checks it is still the one that was asked.
func (e *Engine) turn(ctx context.Context, priority string) (*Running, func(), error) {
	running, err := e.ready()
	if err != nil {
		return nil, nil, err
	}
	release, err := e.Turns.Take(ctx, priority)
	if err != nil {
		return nil, nil, err
	}
	e.mu.Lock()
	same := e.current == running
	e.mu.Unlock()
	if !same {
		release()
		return nil, nil, errSwitched
	}
	return running, release, nil
}

type completion struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete: one answer, forgotten straight after.
func (e *Engine) Complete(ctx context.Context, messages []Message, o Options) (string, error) {
	running, release, err := e.turn(ctx, o.Priority)
	if err != nil {
		return "", err
	}
	defer release()
	response, err := e.ask(ctx, running, chatBody(messages, o, false))
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var answer completion
	if err := json.NewDecoder(response.Body).Decode(&answer); err != nil {
		return "", err
	}
	if len(answer.Choices) == 0 {
		return "", nil
	}
	return strings.TrimSpace(answer.Choices[0].Message.Content), nil
}

// Ready says, before anything is sent, whether there is a model to answer.
func (e *Engine) Ready() error {
	_, err := e.ready()
	return err
}

// Stream: the answer as the model writes it, a few characters at a time,
// each handed to emit. A cancelled ctx (the asker went away) stops the model.
func (e *Engine) Stream(ctx context.Context, messages []Message, o Options, emit func(string) error) error {
	running, release, err := e.turn(ctx, o.Priority)
	if err != nil {
		return err
	}
	defer release()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // closes the connection: llama-server stops writing
	response, err := e.ask(ctx, running, chatBody(messages, o, true))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	reader := bufio.NewReaderSize(response.Body, 64*1024)
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if data, ok := strings.CutPrefix(line, "data:"); ok {
			data = strings.TrimSpace(data)
			if data == "[DONE]" {
				return nil
			}
			var chunk completion
			if json.Unmarshal([]byte(data), &chunk) == nil {
				if chunk.Error != nil {
					return errors.New(chunk.Error.Message)
				}
				if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
					if err := emit(chunk.Choices[0].Delta.Content); err != nil {
						return err
					}
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return errLoading
		}
	}
}

// Describe: one picture, looked at by the same model that chats (never a
// second copy): Family Photos' descriptions, Family Vault reading a scan.
func (e *Engine) Describe(ctx context.Context, jpegDataURI, prompt string, maxTokens int, priority string) (string, string, error) {
	running, err := e.ready()
	if err != nil {
		return "", "", err
	}
	if !running.Sees {
		return "", "", errNoVision
	}
	running, release, err := e.turn(ctx, priority)
	if err != nil {
		return "", "", err
	}
	defer release()
	messages := []map[string]any{{
		"role": "user",
		"content": []map[string]any{
			{"type": "image_url", "image_url": map[string]string{"url": jpegDataURI}},
			{"type": "text", "text": prompt},
		},
	}}
	body := chatBody(messages, Options{Temperature: 0, MaxTokens: maxTokens}, false)
	delete(body, "stop")
	response, err := e.ask(ctx, running, body)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	var answer completion
	if err := json.NewDecoder(response.Body).Decode(&answer); err != nil {
		return "", "", err
	}
	if len(answer.Choices) == 0 {
		return "", running.File, nil
	}
	return strings.TrimSpace(answer.Choices[0].Message.Content), running.File, nil
}

// WatchForModel loads the chosen model once its file is there, on a box that
// had none when this service started: every interval, while nothing is
// loaded, loading or failed on that same file.
func (e *Engine) WatchForModel(ctx context.Context, interval time.Duration) {
	tried := ""
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		file := e.SelectedFile()
		info, err := os.Stat(filepath.Join(e.cfg.ModelsDir, file))
		if err != nil || !info.Mode().IsRegular() || e.LoadedFile() != "" || e.switching() {
			continue
		}
		mark := file + "@" + info.ModTime().String()
		if mark == tried {
			continue
		}
		tried = mark
		if err := e.StartLoading(file); err != nil {
			log.Printf("the model didn't start loading: %v", err)
		}
	}
}
