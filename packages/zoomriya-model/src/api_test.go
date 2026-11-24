package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeLlama stands in for llama-server: it answers chat completions and
// remembers the last request.
type fakeLlama struct {
	mu      sync.Mutex
	last    map[string]any
	started []string
	stopped int
}

func (f *fakeLlama) starter(t *testing.T) Starter {
	return func(model, vision string) (*Running, error) {
		f.mu.Lock()
		f.started = append(f.started, filepath.Base(model))
		f.mu.Unlock()
		if strings.Contains(model, "broken") {
			return nil, errors.New("llama-server stopped while loading the model")
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			f.last = body
			f.mu.Unlock()
			if body["stream"] == true {
				w.Header().Set("Content-Type", "text/event-stream")
				for _, piece := range []string{"Hello", " there."} {
					fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", piece)
				}
				fmt.Fprint(w, "data: [DONE]\n\n")
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{"message": map[string]any{"content": "  An answer. \n"}}},
			})
		}))
		done := make(chan struct{})
		var once sync.Once
		return &Running{
			Base: server.URL, Key: "secret", Sees: vision != "", Done: done,
			Stop: func() {
				once.Do(func() {
					f.mu.Lock()
					f.stopped++
					f.mu.Unlock()
					server.Close()
					close(done)
				})
			},
		}, nil
	}
}

type harness struct {
	t      *testing.T
	id     *testID
	llama  *fakeLlama
	engine *Engine
	api    http.Handler
	models string
}

func newHarness(t *testing.T, files ...string) *harness {
	t.Helper()
	models := t.TempDir()
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(models, f), []byte("gguf"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := Config{ModelsDir: models, StateDir: t.TempDir(), DefaultModel: "gemma-4-E2B-it-Q4_K_M.gguf"}
	h := &harness{t: t, id: newTestID(t), llama: &fakeLlama{}, models: models}
	h.engine = NewEngine(cfg, h.llama.starter(t))
	h.api = NewAPI(h.engine, h.id.auth())
	t.Cleanup(h.engine.Close)
	return h
}

func (h *harness) load(file string) {
	h.t.Helper()
	if err := h.engine.StartLoading(file); err != nil {
		h.t.Fatal(err)
	}
	h.settle()
}

// settle waits for a load in the background to end.
func (h *harness) settle() {
	for i := 0; i < 200 && h.engine.switching(); i++ {
		time.Sleep(5 * time.Millisecond)
	}
}

func (h *harness) do(method, path, token string, body any) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = strings.NewReader(string(raw))
	}
	r := httptest.NewRequest(method, base+path, reader)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.api.ServeHTTP(w, r)
	return w
}

func field(w *httptest.ResponseRecorder, name string) any {
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return body[name]
}

var question = map[string]any{"messages": []map[string]string{{"role": "user", "content": "Hi"}}}

func TestCompleteAnswersMembersOnly(t *testing.T) {
	h := newHarness(t, "gemma-4-E2B-it-Q4_K_M.gguf")
	h.load("gemma-4-E2B-it-Q4_K_M.gguf")
	if w := h.do("POST", "/complete", "", question); w.Code != 401 || field(w, "detail") != "Sign in first" {
		t.Errorf("no token: %d %s", w.Code, w.Body)
	}
	if w := h.do("POST", "/complete", "forged", question); w.Code != 401 || field(w, "detail") != "Sign in again" {
		t.Errorf("bad token: %d %s", w.Code, w.Body)
	}
	w := h.do("POST", "/complete", h.id.token(nil, nil), map[string]any{
		"messages":    question["messages"],
		"priority":    "background",
		"json_schema": map[string]any{"type": "object"},
	})
	if w.Code != 200 || field(w, "reply") != "An answer." {
		t.Fatalf("complete: %d %s", w.Code, w.Body)
	}
	last := h.llama.last
	if last["max_tokens"] != 300.0 || last["temperature"] != 0.2 || last["stream"] != false {
		t.Errorf("defaults not sent: %v", last)
	}
	if format, _ := last["response_format"].(map[string]any); format["schema"] == nil {
		t.Errorf("the schema wasn't sent: %v", last)
	}
	if stop, _ := last["stop"].([]any); len(stop) != 2 {
		t.Errorf("Gemma's stop words weren't sent: %v", last["stop"])
	}
}

func TestCompleteChecksTheRequest(t *testing.T) {
	h := newHarness(t, "gemma-4-E2B-it-Q4_K_M.gguf")
	h.load("gemma-4-E2B-it-Q4_K_M.gguf")
	token := h.id.token(nil, nil)
	bad := []map[string]any{
		{"messages": []any{}},
		{"messages": []map[string]string{{"role": "tool", "content": "x"}}},
		{"messages": question["messages"], "max_tokens": 2000},
		{"messages": question["messages"], "temperature": 2},
		{"messages": question["messages"], "priority": "now"},
		{"messages": question["messages"], "stop": []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}},
	}
	for _, body := range bad {
		if w := h.do("POST", "/complete", token, body); w.Code != 422 {
			t.Errorf("%v: %d, want 422", body, w.Code)
		}
	}
}

func TestNoModelIsA503WithTheAppsWords(t *testing.T) {
	h := newHarness(t)
	w := h.do("POST", "/complete/stream", h.id.token(nil, nil), question)
	if w.Code != 503 || field(w, "detail") != "Myira's model is still loading. Try again in a moment." {
		t.Errorf("no model: %d %s", w.Code, w.Body)
	}
}

func TestStreamSendsPiecesThenDone(t *testing.T) {
	h := newHarness(t, "gemma-4-E2B-it-Q4_K_M.gguf")
	h.load("gemma-4-E2B-it-Q4_K_M.gguf")
	w := h.do("POST", "/complete/stream", h.id.token(nil, nil), question)
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/x-ndjson" {
		t.Fatalf("stream: %d %s", w.Code, w.Header())
	}
	var lines []string
	scanner := bufio.NewScanner(w.Body)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	want := []string{`{"text":"Hello"}`, `{"text":" there."}`, `{"done":true}`}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %q", lines)
	}
}

func TestDescribeNeedsTheVisionFile(t *testing.T) {
	h := newHarness(t, "gemma-4-E2B-it-Q4_K_M.gguf")
	h.load("gemma-4-E2B-it-Q4_K_M.gguf")
	picture := map[string]any{"image_jpeg_base64": "/9j/4AAQ", "prompt": "Describe it."}
	if w := h.do("POST", "/describe", "", picture); w.Code != 503 {
		t.Errorf("without a vision file: %d %s", w.Code, w.Body)
	}

	h = newHarness(t, "gemma-4-E2B-it-Q4_K_M.gguf", "mmproj-gemma-4-E2B-it-BF16.gguf")
	h.load("gemma-4-E2B-it-Q4_K_M.gguf")
	if w := h.do("POST", "/describe", "", map[string]any{"image_jpeg_base64": "not base64!", "prompt": "x"}); w.Code != 400 {
		t.Errorf("bad base64: %d", w.Code)
	}
	w := h.do("POST", "/describe", "", picture)
	if w.Code != 200 || field(w, "reply") != "An answer." || field(w, "model") != "gemma-4-E2B-it-Q4_K_M.gguf" {
		t.Fatalf("describe: %d %s", w.Code, w.Body)
	}
	content := h.llama.last["messages"].([]any)[0].(map[string]any)["content"].([]any)
	if content[0].(map[string]any)["type"] != "image_url" || content[1].(map[string]any)["text"] != "Describe it." {
		t.Errorf("picture not sent first: %v", content)
	}
	current := h.do("GET", "/model/current", "", nil)
	if field(current, "sees") != true || field(current, "loaded") != "gemma-4-E2B-it-Q4_K_M.gguf" {
		t.Errorf("current: %s", current.Body)
	}
}

func TestAdminChoosesTheModel(t *testing.T) {
	h := newHarness(t, "gemma-4-E2B-it-Q4_K_M.gguf", "gemma-4-E4B-it-Q4_K_M.gguf", "mmproj-gemma-4-E2B-it-BF16.gguf")
	h.load("gemma-4-E2B-it-Q4_K_M.gguf")
	admin := h.id.token(map[string]any{"role": "admin"}, nil)
	if w := h.do("GET", "/model", h.id.token(nil, nil), nil); w.Code != 403 {
		t.Errorf("a member saw the models: %d", w.Code)
	}
	w := h.do("GET", "/model", admin, nil)
	if options := field(w, "options").([]any); len(options) != 2 {
		t.Fatalf("the vision file was listed as a model: %v", options)
	}
	if field(w, "loaded_label") != "Gemma 4 E2B" {
		t.Errorf("label: %s", w.Body)
	}
	if w := h.do("PUT", "/model", admin, map[string]string{"file": "../etc/passwd"}); w.Code != 400 {
		t.Errorf("a path out of the models' folder: %d", w.Code)
	}
	if w := h.do("PUT", "/model", admin, map[string]string{"file": "gemma-4-E4B-it-Q4_K_M.gguf"}); w.Code != 200 {
		t.Fatalf("choose: %d %s", w.Code, w.Body)
	}
	h.settle()
	if h.engine.LoadedFile() != "gemma-4-E4B-it-Q4_K_M.gguf" || h.engine.SelectedFile() != "gemma-4-E4B-it-Q4_K_M.gguf" {
		t.Errorf("loaded %s, selected %s", h.engine.LoadedFile(), h.engine.SelectedFile())
	}
	if h.llama.stopped != 1 {
		t.Errorf("the old model was stopped %d times, want once", h.llama.stopped)
	}
}

func TestAFailedSwitchGoesBack(t *testing.T) {
	h := newHarness(t, "gemma-4-E2B-it-Q4_K_M.gguf", "broken.gguf")
	h.load("gemma-4-E2B-it-Q4_K_M.gguf")
	h.load("broken.gguf")
	if h.engine.LoadedFile() != "gemma-4-E2B-it-Q4_K_M.gguf" {
		t.Errorf("after a failed switch, loaded %q", h.engine.LoadedFile())
	}
	if status := h.engine.StatusPayload(); status["status"] != "failed" || status["error"] == nil {
		t.Errorf("the failure isn't shown to the admin: %v", status)
	}
	// The old model is back: members are answered while the admin sees the failure.
	if w := h.do("POST", "/complete", h.id.token(nil, nil), question); w.Code != 200 {
		t.Errorf("after a failed switch: %d %s", w.Code, w.Body)
	}
}

func TestEachGemmaGetsItsOwnVisionFile(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"gemma-4-E2B-it-Q4_K_M.gguf", "gemma-4-E4B-it-Q4_K_M.gguf", "mmproj-gemma-4-E2B-it-BF16.gguf", "other.gguf"} {
		os.WriteFile(filepath.Join(dir, f), []byte("gguf"), 0o644)
	}
	if got := VisionFor(filepath.Join(dir, "gemma-4-E2B-it-Q4_K_M.gguf")); filepath.Base(got) != "mmproj-gemma-4-E2B-it-BF16.gguf" {
		t.Errorf("E2B: %q", got)
	}
	for _, f := range []string{"gemma-4-E4B-it-Q4_K_M.gguf", "other.gguf"} {
		if got := VisionFor(filepath.Join(dir, f)); got != "" {
			t.Errorf("%s got another model's vision file: %q", f, got)
		}
	}
}

func TestAStreamHoldsTheModelUntilItEnds(t *testing.T) {
	h := newHarness(t, "gemma-4-E2B-it-Q4_K_M.gguf")
	h.load("gemma-4-E2B-it-Q4_K_M.gguf")
	held := false
	err := h.engine.Stream(t.Context(), []Message{{"user", "hi"}}, Options{Priority: Interactive, MaxTokens: 5},
		func(string) error { held = held || h.engine.Turns.Busy(); return nil })
	if err != nil || !held {
		t.Errorf("held %v, err %v", held, err)
	}
	if h.engine.Turns.Busy() {
		t.Error("the model is still held after the stream ended")
	}
}

func TestAnswersWaitWhileSwitching(t *testing.T) {
	h := newHarness(t, "gemma-4-E2B-it-Q4_K_M.gguf")
	h.load("gemma-4-E2B-it-Q4_K_M.gguf")
	h.engine.mu.Lock()
	h.engine.status = "loading"
	h.engine.mu.Unlock()
	w := h.do("POST", "/complete", h.id.token(nil, nil), question)
	if w.Code != 503 || field(w, "detail") != "Myira is switching models. Try again in a moment." {
		t.Errorf("while switching: %d %s", w.Code, w.Body)
	}
	if w := h.do("PUT", "/model", h.id.token(map[string]any{"role": "admin"}, nil), map[string]string{"file": "gemma-4-E2B-it-Q4_K_M.gguf"}); w.Code != 409 {
		t.Errorf("a second switch while one loads: %d", w.Code)
	}
}

func TestAModelThatArrivesLaterIsLoaded(t *testing.T) {
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.engine.WatchForModel(ctx, 10*time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	if h.engine.LoadedFile() != "" {
		t.Fatal("loaded with no model")
	}
	os.WriteFile(filepath.Join(h.models, "gemma-4-E2B-it-Q4_K_M.gguf"), []byte("gguf"), 0o644)
	for i := 0; i < 100 && h.engine.LoadedFile() == ""; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if h.engine.LoadedFile() != "gemma-4-E2B-it-Q4_K_M.gguf" {
		t.Errorf("the new model wasn't loaded")
	}
}
