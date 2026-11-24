package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeModel stands in for zoomriya-model: a pick by the question's words,
// then a streamed answer.
type fakeModel struct {
	loaded bool
	mu     sync.Mutex
	asked  []map[string]any
}

func (f *fakeModel) server(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /model/current", func(w http.ResponseWriter, _ *http.Request) {
		if f.loaded {
			fmt.Fprint(w, `{"file":"gemma.gguf","loaded":"gemma.gguf","sees":true}`)
		} else {
			fmt.Fprint(w, `{"file":"gemma.gguf","loaded":null,"sees":false}`)
		}
	})
	mux.HandleFunc("POST /complete", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.asked = append(f.asked, body)
		f.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer token-ada" {
			w.WriteHeader(401)
			return
		}
		question := body["messages"].([]any)[1].(map[string]any)["content"].(string)
		pick := `{"pick":"other"}`
		switch {
		case strings.Contains(question, "slow"):
			pick = `{"pick":"help:1"}`
		case strings.Contains(question, "full"):
			pick = `{"pick":"box.storage"}`
		case strings.Contains(question, "Erase"):
			pick = `{"pick":"open:0"}`
		case strings.Contains(question, "hour"):
			pick = `{"pick":"box.set_maintenance_hour","arguments":{"hour":"04:30","extra":"dropped"}}`
		}
		json.NewEncoder(w).Encode(map[string]string{"reply": pick})
	})
	mux.HandleFunc("POST /complete/stream", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.asked = append(f.asked, body)
		f.mu.Unlock()
		fmt.Fprint(w, "{\"text\":\"The library \"}\n{\"text\":\"is half full.\"}\n{\"done\":true}\n")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// fakeApp stands in for the Myira app's API.
type fakeApp struct {
	on   bool
	mu   sync.Mutex
	last map[string]any
	path string
}

func (f *fakeApp) server(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /me", func(w http.ResponseWriter, r *http.Request) {
		if !f.on || r.Header.Get("Authorization") != "Bearer token-ada" {
			w.WriteHeader(403)
			return
		}
		fmt.Fprint(w, `{"display_name":"Ada"}`)
	})
	ask := func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.last, f.path = body, r.URL.Path
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprint(w, "{\"type\":\"route\",\"conversation_id\":7}\n{\"type\":\"text\",\"text\":\"Hello.\"}\n{\"type\":\"done\",\"answer\":\"Hello.\"}\n")
	}
	mux.HandleFunc("POST /ask", ask)
	mux.HandleFunc("POST /ask/tool", ask)
	mux.HandleFunc("GET /conversations", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `[{"id":7,"title":"The disk"}]`)
	})
	mux.HandleFunc("DELETE /conversations/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func myiraBox(t *testing.T, model *fakeModel, app *fakeApp) *testBox {
	b := newBox(t)
	b.srv.Cfg.ModelAPI = model.server(t).URL
	b.srv.Cfg.MyiraApp = app.server(t).URL
	b.home = b.srv.Handler(ListenerHome)
	b.signIn()
	return b
}

func events(t *testing.T, w *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var all []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(w.Body.String()), "\n") {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("not a JSON line: %q", line)
		}
		all = append(all, event)
	}
	return all
}

func last(all []map[string]any) map[string]any { return all[len(all)-1] }

var help = []map[string]string{
	{"title": "Something stopped working after an update", "text": "Restore the OS.", "tool": "control", "section": "restore"},
	{"title": "The box is slow", "text": "See which app is busy. Photos works hard after many new photos.", "tool": "resources", "section": "apps"},
}

func TestTheOSsOwnMyiraAnswersWithHelpWordForWord(t *testing.T) {
	b := myiraBox(t, &fakeModel{loaded: true}, &fakeApp{})
	_, state := b.do("GET", "/api/myira", nil)
	if state["who"] != "os" || state["model"] != true || state["chats"] != false {
		t.Errorf("state: %v", state)
	}
	w, _ := b.do("POST", "/api/myira/ask", map[string]any{"question": "Why is the box so slow?", "lang": "en", "help": help})
	done := last(events(t, w))
	if done["answer"] != help[1]["text"] || done["fixed"] != true {
		t.Errorf("not Help's words: %v", done)
	}
	if open := done["open"].(map[string]any); open["tool"] != "resources" || open["section"] != "apps" {
		t.Errorf("no button to the tool: %v", done)
	}
}

func TestTheOSsOwnMyiraKnowsOnlyTheBox(t *testing.T) {
	model := &fakeModel{loaded: true}
	b := myiraBox(t, model, &fakeApp{})
	w, _ := b.do("POST", "/api/myira/ask", map[string]any{"question": "What's the capital of Peru?", "help": help})
	if done := last(events(t, w)); done["say"] != "not_box" {
		t.Errorf("a question about something else: %v", done)
	}
	// No model yet: said, not guessed.
	model.loaded = false
	w, _ = b.do("POST", "/api/myira/ask", map[string]any{"question": "Why is the box slow?", "help": help})
	if ev := last(events(t, w)); ev["type"] != "error" || ev["say"] != "no_model" {
		t.Errorf("no model: %v", ev)
	}
}

func TestAToolIsPickedThenRunByBoxDesktop(t *testing.T) {
	model := &fakeModel{loaded: true}
	b := myiraBox(t, model, &fakeApp{})
	w, _ := b.do("POST", "/api/myira/ask", map[string]any{"question": "Is the disk full?", "help": help})
	all := events(t, w)
	if all[1]["type"] != "tool" || all[1]["name"] != "storage" || all[1]["writes"] != false {
		t.Fatalf("no tool picked: %v", all)
	}
	w, _ = b.do("POST", "/api/myira/tool", map[string]any{"question": "Is the disk full?", "app": "box", "name": "storage", "title": "Storage"})
	all = events(t, w)
	if b.agent.last()["op"] != "storage" || b.agent.last()["by"] != "ada" {
		t.Errorf("the agent was asked %v", b.agent.last())
	}
	if done := last(all); done["answer"] != "The library is half full." {
		t.Errorf("answer: %v", all)
	}
	system := model.asked[len(model.asked)-1]["messages"].([]any)[0].(map[string]any)["content"].(string)
	if !strings.Contains(system, `"op":"storage"`) || !strings.Contains(system, "Reply in English") {
		t.Errorf("the answer wasn't asked from the tool's result: %s", system)
	}
}

func TestAChangeOnlyWhatTheToolTakes(t *testing.T) {
	b := myiraBox(t, &fakeModel{loaded: true}, &fakeApp{})
	w, _ := b.do("POST", "/api/myira/ask", map[string]any{"question": "Move the restart hour to 4:30", "help": help})
	tool := events(t, w)[1]
	if tool["writes"] != true || fmt.Sprint(tool["arguments"]) != "map[hour:04:30]" {
		t.Errorf("the change's card: %v", tool)
	}
	// Said no on the card: nothing done, nothing asked of the agent.
	before := len(b.agent.seen)
	w, _ = b.do("POST", "/api/myira/tool", map[string]any{"app": "box", "name": "set_maintenance_hour", "arguments": map[string]any{"hour": "04:30"}, "declined": true})
	if done := last(events(t, w)); done["say"] != "declined" || len(b.agent.seen) != before {
		t.Errorf("declined: %v", done)
	}
	b.do("POST", "/api/myira/tool", map[string]any{"app": "box", "name": "set_maintenance_hour", "arguments": map[string]any{"hour": "04:30"}})
	if got := b.agent.last(); got["op"] != "maintenance-hour" || got["hour"] != "04:30" {
		t.Errorf("the agent was asked %v", got)
	}
	// A bad hour never reaches the agent.
	before = len(b.agent.seen)
	b.do("POST", "/api/myira/tool", map[string]any{"app": "box", "name": "set_maintenance_hour", "arguments": map[string]any{"hour": "25:00"}})
	if len(b.agent.seen) != before {
		t.Error("a bad hour reached the agent")
	}
}

func TestNeverAToolThatAsksForThePassword(t *testing.T) {
	b := myiraBox(t, &fakeModel{loaded: true}, &fakeApp{})
	for _, name := range []string{"restart", "shutdown", "factory_reset", "restore_point_restore", "erase"} {
		if w, _ := b.do("POST", "/api/myira/tool", map[string]any{"app": "box", "name": name}); w.Code != 400 {
			t.Errorf("%s: %d", name, w.Code)
		}
	}
	if w, _ := b.do("POST", "/api/myira/tool", map[string]any{"app": "photos", "name": "storage"}); w.Code != 400 {
		t.Errorf("another app's tool: %d", w.Code)
	}
	for _, tool := range boxTools {
		if tool.App != "box" {
			t.Errorf("%s: app %q", tool.Name, tool.App)
		}
	}
	for _, op := range []string{"restart", "shutdown", "factory-reset", "restore-point-restore", "held-release"} {
		if _, err := b.srv.askAgent("ada", op, nil); err == nil {
			t.Errorf("%s ran without the password", op)
		}
	}
	if _, err := b.srv.askAgent("ada", "advanced-access", map[string]any{"action": "on"}); err == nil {
		t.Error("advanced access ran without the password")
	}
}

func TestTheMyiraAppAnswersWhenItIsOn(t *testing.T) {
	app := &fakeApp{on: true}
	b := myiraBox(t, &fakeModel{loaded: true}, app)
	_, state := b.do("GET", "/api/myira", nil)
	if state["who"] != "app" || state["chats"] != true {
		t.Errorf("state: %v", state)
	}
	w, _ := b.do("POST", "/api/myira/ask", map[string]any{"question": "Hi", "lang": "fr", "conversation_id": 7})
	if done := last(events(t, w)); done["answer"] != "Hello." {
		t.Errorf("the app's stream wasn't passed on: %s", w.Body)
	}
	tools := app.last["tools"].([]any)
	if len(tools) != len(boxTools) || tools[0].(map[string]any)["app"] != "box" || app.last["lang"] != "fr" ||
		app.last["surface"] != "box-desktop" {
		t.Errorf("the app wasn't offered the OS's tools: %v", app.last)
	}
	b.do("POST", "/api/myira/tool", map[string]any{"app": "box", "name": "storage", "conversation_id": 7, "question_id": 9})
	if app.path != "/ask/tool" || app.last["result"] != `{"ok":true,"op":"storage"}` || app.last["question_id"] != 9.0 {
		t.Errorf("the tool's result wasn't sent to the app: %s %v", app.path, app.last)
	}
	_, list := b.do("GET", "/api/myira/chats", nil)
	if list != nil { // an array: b.do reads objects only
		t.Errorf("chats: %v", list)
	}
	if w, _ := b.do("DELETE", "/api/myira/chats/7", nil); w.Code != 204 {
		t.Errorf("delete: %d", w.Code)
	}
	// Off for this admin: the OS's own Myira, and no chats.
	app.on = false
	_, state = b.do("GET", "/api/myira", nil)
	if state["who"] != "os" {
		t.Errorf("state: %v", state)
	}
	if w, _ := b.do("DELETE", "/api/myira/chats/7", nil); w.Code != 404 {
		t.Errorf("delete without the app: %d", w.Code)
	}
}

func TestMyiraIsForSignedInAdminsOnly(t *testing.T) {
	b := newBox(t)
	if w, _ := b.do("GET", "/api/myira", nil); w.Code != 401 {
		t.Errorf("signed out: %d", w.Code)
	}
	b.signIn()
	b.token = ""
	if w, _ := b.do("POST", "/api/myira/ask", map[string]any{"question": "hi"}); w.Code != 403 {
		t.Errorf("without the page's token: %d", w.Code)
	}
}

func TestToolAnswersSaySizesAsTheScreensDo(t *testing.T) {
	answer := map[string]any{"library": map[string]any{"size": 4000787030016.0, "used": 3100000000000.0, "free": 900787030016.0, "count": 12.0}}
	en, _ := json.Marshal(readable(answer, "", "en"))
	if string(en) != `{"library":{"count":12,"free":"901 GB","size":"4 TB","used":"3.1 TB","used_percent":77}}` {
		t.Errorf("en: %s", en)
	}
	fr, _ := json.Marshal(readable(answer, "", "fr"))
	if string(fr) != `{"library":{"count":12,"free":"901 Go","size":"4 To","used":"3,1 To","used_percent":77}}` {
		t.Errorf("fr: %s", fr)
	}
}

func TestSomeoneHurtGetsTheEmergencyNumberFirst(t *testing.T) {
	model := &fakeModel{loaded: false} // even with no model
	b := myiraBox(t, model, &fakeApp{})
	b.agent.answers = map[string]string{"setup-status": `{"ok":true,"region":"CA-ON"}`}
	for _, q := range []string{"My son is hurt and bleeding", "Mon fils est blessé", "Someone broke in!"} {
		w, _ := b.do("POST", "/api/myira/ask", map[string]any{"question": q, "help": help})
		if done := last(events(t, w)); done["say"] != "emergency" || done["number"] != "911" {
			t.Errorf("%q: %v", q, done)
		}
	}
	b.agent.answers = map[string]string{"setup-status": `{"ok":true,"region":"LK"}`}
	w, _ := b.do("POST", "/api/myira/ask", map[string]any{"question": "There's a fire in the kitchen", "help": help})
	if done := last(events(t, w)); done["say"] != "emergency" || done["number"] != nil {
		t.Errorf("a country with no number written here: %v", done)
	}
	// A box question with a word near one isn't an emergency.
	model.loaded = true
	w, _ = b.do("POST", "/api/myira/ask", map[string]any{"question": "Help me fix the network", "help": help})
	if done := last(events(t, w)); done["say"] == "emergency" {
		t.Errorf("not an emergency: %v", done)
	}
}

func TestWhatNeedsThePasswordOpensItsPlace(t *testing.T) {
	b := myiraBox(t, &fakeModel{loaded: true}, &fakeApp{})
	w, _ := b.do("POST", "/api/myira/ask", map[string]any{"question": "Erase everything on the box", "help": help})
	done := last(events(t, w))
	open, _ := done["open"].(map[string]any)
	if done["say"] != "needs_password" || open["tool"] != "control" || open["section"] != "restore" {
		t.Errorf("erase: %v", done)
	}
	if len(b.agent.seen) != 0 {
		t.Errorf("the agent was asked %v", b.agent.seen)
	}
}

func TestEachStepGoesBackForTheNextTool(t *testing.T) {
	app := &fakeApp{on: true}
	b := myiraBox(t, &fakeModel{loaded: true}, app)
	earlier := []map[string]any{{"app": "box", "name": "storage", "title": "Storage", "arguments": map[string]any{}, "result": "{}"}}
	w, _ := b.do("POST", "/api/myira/tool", map[string]any{"app": "box", "name": "backup_now", "title": "Back up now",
		"conversation_id": 7, "question_id": 9, "earlier": earlier})
	step := events(t, w)[0]
	if step["type"] != "step" || step["name"] != "backup_now" || step["result"] != `{"ok":true,"op":"backups"}` {
		t.Errorf("no step first: %v", step)
	}
	if got := app.last["earlier"].([]any); len(got) != 1 || got[0].(map[string]any)["name"] != "storage" {
		t.Errorf("earlier steps not sent on: %v", app.last["earlier"])
	}
	if w, _ := b.do("POST", "/api/myira/tool", map[string]any{"app": "box", "name": "storage", "earlier": append(earlier, earlier[0], earlier[0])}); w.Code != 400 {
		t.Errorf("more steps than one request may take: %d", w.Code)
	}
}

func TestTheModelRequestReachesTheAgent(t *testing.T) {
	b := myiraBox(t, &fakeModel{}, &fakeApp{})
	if w, a := b.do("POST", "/api/agent", map[string]any{"request": map[string]any{"op": "model", "action": "download", "name": "e2b"}}); w.Code != 200 {
		t.Errorf("model download: %d %v", w.Code, a)
	}
	if got := b.agent.last(); got["op"] != "model" || got["action"] != "download" || got["by"] != "ada" {
		t.Errorf("the agent was asked %v", got)
	}
}
