// Myira in Box Desktop's panel (box-desktop.md §4, Myira; the assistant
// contract, docs/assistant-contract.md). One panel, whichever Myira is on the
// box: when the Myira app is installed and on for this admin, it answers,
// with everything it answers on /myira; otherwise the OS's own Myira
// (myira_os.go) answers questions about the box. box-desktopd decides for
// each question, so installing or removing the app changes the panel at once.
//
// Either way the OS's tools (myira_tools.go) are offered with the question;
// Myira only picks one, and the panel has box-desktopd run it here as the
// admin (a change only after the admin confirmed it on its card).
//
//	GET    /api/myira             who answers, and whether the model is there
//	POST   /api/myira/ask         a question: the answer as a stream (NDJSON)
//	POST   /api/myira/tool        run the picked tool, then the answer from it
//	GET    /api/myira/chats       the admin's Myira chats (the app keeps them)
//	GET    /api/myira/chats/{id}  one chat
//	DELETE /api/myira/chats/{id}  delete one
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Who answers in the panel.
const (
	byApp = "app" // the Myira app
	byOS  = "os"  // the OS's own Myira
)

// myiraClient: to the Myira app and the model service, on loopback. No
// timeout: an answer takes as long as the model takes.
var myiraClient = &http.Client{}

// appRequest asks the Myira app as the admin, with their ZOOMRIYA ID token,
// renewing it once if it ran out.
func (s *Server) appRequest(ctx context.Context, login *Login, method, path string, body any) (*http.Response, error) {
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	for attempt := 0; ; attempt++ {
		request, err := http.NewRequestWithContext(ctx, method, s.Cfg.MyiraApp+path, bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Authorization", "Bearer "+login.Token)
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		response, err := myiraClient.Do(request)
		if err != nil {
			return nil, err
		}
		if response.StatusCode == http.StatusUnauthorized && attempt == 0 && s.ID.Renew(login) {
			response.Body.Close()
			continue
		}
		return response, nil
	}
}

// whoAnswers: the Myira app when it is installed and on for this admin (it
// answers /me for them), else the OS's own Myira.
func (s *Server) whoAnswers(ctx context.Context, login *Login) string {
	if s.Cfg.MyiraApp == "" {
		return byOS
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	response, err := s.appRequest(ctx, login, http.MethodGet, "/me", nil)
	if err != nil {
		return byOS
	}
	response.Body.Close()
	if response.StatusCode == http.StatusOK {
		return byApp
	}
	return byOS
}

// modelLoaded: the box's model service has a model to answer with.
func (s *Server) modelLoaded(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.Cfg.ModelAPI+"/model/current", nil)
	response, err := myiraClient.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	var current struct {
		Loaded *string `json:"loaded"`
	}
	return json.NewDecoder(response.Body).Decode(&current) == nil && current.Loaded != nil
}

func (s *Server) myiraState(w http.ResponseWriter, r *http.Request, _ Listener, se *Session) {
	who := s.whoAnswers(r.Context(), se.Login())
	writeJSON(w, http.StatusOK, map[string]any{
		"who":   who,
		"chats": who == byApp,
		"model": s.modelLoaded(r.Context()),
	})
}

// HelpTopic is one of Support Center's Help topics, in the admin's language,
// as the page shows it: the OS's own Myira answers how-tos with them, word
// for word.
type HelpTopic struct {
	Title   string `json:"title"`
	Text    string `json:"text"`
	Tool    string `json:"tool"`
	Section string `json:"section"`
}

// Step: a tool already run for this question, and what came of it (the
// app's family_tools.Step).
type Step struct {
	App       string         `json:"app"`
	Name      string         `json:"name"`
	Title     string         `json:"title"`
	Arguments map[string]any `json:"arguments"`
	Result    string         `json:"result"`
	IsError   bool           `json:"is_error"`
	Declined  bool           `json:"declined"`
}

type askBody struct {
	Question       string      `json:"question"`
	Lang           string      `json:"lang"`
	ConversationID *int        `json:"conversation_id"`
	TZ             string      `json:"tz"`
	Help           []HelpTopic `json:"help"`
}

func (b *askBody) clean() *Problem {
	b.Question = strings.TrimSpace(b.Question)
	if b.Question == "" || len([]rune(b.Question)) > 4000 || len(b.Help) > 50 {
		return problem(http.StatusBadRequest, "bad_request")
	}
	if b.Lang != "fr" {
		b.Lang = "en"
	}
	return nil
}

func (s *Server) myiraAsk(w http.ResponseWriter, r *http.Request, _ Listener, se *Session) {
	var b askBody
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	if p := b.clean(); p != nil {
		writeProblem(w, p)
		return
	}
	login := se.Login()
	if s.whoAnswers(r.Context(), login) == byApp {
		s.passOn(w, r, login, "/ask", map[string]any{
			"question": b.Question, "lang": b.Lang, "conversation_id": b.ConversationID,
			"tz": nilIfEmpty(b.TZ), "tools": Offers(),
			// A new chat is marked with Box Desktop's sign in the app's History.
			"surface": "box-desktop",
		})
		return
	}
	s.osMyira().Ask(w, r, login, b)
}

type toolBody struct {
	// The question it was picked for (the OS's own Myira answers from it).
	Question string `json:"question"`
	Lang     string `json:"lang"`
	TZ       string `json:"tz"`
	// The app's chat and the question's turn (the Myira app's /ask/tool).
	ConversationID *int `json:"conversation_id"`
	QuestionID     *int `json:"question_id"`
	// The tool picked, and whether the admin confirmed it (a change) or said no.
	App       string         `json:"app"`
	Name      string         `json:"name"`
	Title     string         `json:"title"`
	Arguments map[string]any `json:"arguments"`
	Declined  bool           `json:"declined"`
	Earlier   []Step         `json:"earlier"`
}

func (s *Server) myiraTool(w http.ResponseWriter, r *http.Request, _ Listener, se *Session) {
	var b toolBody
	if p := readJSON(r, &b); p != nil {
		writeProblem(w, p)
		return
	}
	if b.Lang != "fr" {
		b.Lang = "en"
	}
	if findTool(b.App, b.Name) == nil || len(b.Earlier) > 2 {
		writeProblem(w, problem(http.StatusBadRequest, "bad_request"))
		return
	}
	login := se.Login()
	result, failed := "", false
	if !b.Declined {
		result, failed = s.RunTool(login.Member.Username, b.App, b.Name, b.Lang, b.Arguments)
	}
	step := Step{App: b.App, Name: b.Name, Title: b.Title, Arguments: b.Arguments, Result: result, IsError: failed, Declined: b.Declined}
	if b.ConversationID != nil && b.QuestionID != nil && s.whoAnswers(r.Context(), login) == byApp {
		s.passOn(w, r, login, "/ask/tool", map[string]any{
			"conversation_id": *b.ConversationID, "question_id": *b.QuestionID,
			"app": step.App, "name": step.Name, "title": step.Title, "result": step.Result,
			"is_error": step.IsError, "declined": step.Declined, "lang": b.Lang,
			"arguments": orEmpty(step.Arguments), "earlier": orNone(b.Earlier),
			"tools": Offers(), "tz": nilIfEmpty(b.TZ),
		}, stepEvent(step))
		return
	}
	s.osMyira().FromTool(w, r, login, b, step)
}

// passOn sends the request to the Myira app and its answer to the page as
// it comes, line by line.
func (s *Server) passOn(w http.ResponseWriter, r *http.Request, login *Login, path string, body any, first ...map[string]any) {
	response, err := s.appRequest(r.Context(), login, http.MethodPost, path, body)
	if err != nil {
		out := stream(w)
		for _, event := range first {
			out.send(event)
		}
		out.send(map[string]any{"type": "error", "say": "unavailable"})
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		var refused struct {
			Detail any `json:"detail"`
		}
		json.NewDecoder(io.LimitReader(response.Body, 1<<16)).Decode(&refused)
		event := map[string]any{"type": "error", "say": "unavailable"}
		if detail, ok := refused.Detail.(string); ok && detail != "" {
			event["detail"] = detail
		}
		out := stream(w)
		for _, event := range first {
			out.send(event)
		}
		out.send(event)
		return
	}
	out := stream(w)
	for _, event := range first {
		out.send(event)
	}
	reader := bufio.NewReaderSize(response.Body, 1<<16)
	for {
		line, err := reader.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			if !json.Valid(bytes.TrimSpace(line)) {
				break
			}
			out.raw(bytes.TrimSpace(line))
		}
		if err != nil {
			return
		}
	}
}

// ---- the admin's chats (the Myira app keeps them) ----------------------------------

func (s *Server) myiraChats(w http.ResponseWriter, r *http.Request, _ Listener, se *Session) {
	s.chatRequest(w, r, se, http.MethodGet, "/conversations")
}

func (s *Server) myiraChat(w http.ResponseWriter, r *http.Request, _ Listener, se *Session) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		writeProblem(w, problem(http.StatusNotFound, "not_found"))
		return
	}
	s.chatRequest(w, r, se, r.Method, "/conversations/"+strconv.Itoa(id))
}

func (s *Server) chatRequest(w http.ResponseWriter, r *http.Request, se *Session, method, path string) {
	login := se.Login()
	if s.whoAnswers(r.Context(), login) != byApp {
		// The OS's own Myira keeps no chats.
		if method == http.MethodGet && path == "/conversations" {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		writeProblem(w, problem(http.StatusNotFound, "not_found"))
		return
	}
	response, err := s.appRequest(r.Context(), login, method, path, nil)
	if err != nil {
		writeProblem(w, problem(http.StatusBadGateway, "myira_down"))
		return
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNoContent {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	io.Copy(w, io.LimitReader(response.Body, 8<<20))
}

// ---- the stream to the page ---------------------------------------------------------

type ndjson struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func stream(w http.ResponseWriter) *ndjson {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	return &ndjson{w, flusher}
}

func (n *ndjson) raw(line []byte) {
	n.w.Write(append(line, '\n'))
	if n.flusher != nil {
		n.flusher.Flush()
	}
}

func (n *ndjson) send(event map[string]any) {
	raw, _ := json.Marshal(event)
	n.raw(raw)
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func orEmpty(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

func orNone(steps []Step) []Step {
	if steps == nil {
		return []Step{}
	}
	return steps
}

// stepEvent: what came of a tool, for the page to send back as "earlier"
// when Myira picks another one for the same request.
func stepEvent(step Step) map[string]any {
	return map[string]any{"type": "step", "app": step.App, "name": step.Name, "title": step.Title,
		"arguments": orEmpty(step.Arguments), "result": step.Result, "is_error": step.IsError, "declined": step.Declined}
}
