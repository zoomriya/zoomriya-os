// The API, on loopback only (127.0.0.1:8070, never behind the gateway). The
// same routes, fields, limits and answers the apps' model service had, so
// the apps change nothing but where it is installed from:
//
//	GET  /myira/api/v1/health           anyone: {"status": "ok"}
//	POST /myira/api/v1/complete         a signed-in member: one answer
//	POST /myira/api/v1/complete/stream  a signed-in member: the answer as it is written
//	POST /myira/api/v1/describe         local callers: one picture, one answer
//	GET  /myira/api/v1/model/current    which model is chosen and loaded, and whether it sees
//	GET  /myira/api/v1/model            an admin: the installed models and the choice
//	PUT  /myira/api/v1/model            an admin: choose one
//
// Errors are {"detail": "..."}, as before.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"unicode/utf8"
)

const base = "/myira/api/v1"

type API struct {
	engine *Engine
	auth   *Auth
}

func NewAPI(engine *Engine, auth *Auth) http.Handler {
	a := &API{engine, auth}
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+base+"/health", func(w http.ResponseWriter, _ *http.Request) {
		reply(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST "+base+"/complete", a.member(a.complete))
	mux.HandleFunc("POST "+base+"/complete/stream", a.member(a.completeStream))
	mux.HandleFunc("POST "+base+"/describe", a.describe)
	mux.HandleFunc("GET "+base+"/model/current", a.current)
	mux.HandleFunc("GET "+base+"/model", a.admin(a.model))
	mux.HandleFunc("PUT "+base+"/model", a.admin(a.choose))
	return mux
}

func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func detail(w http.ResponseWriter, status int, message string) {
	reply(w, status, map[string]string{"detail": message})
}

// ---- who is asking --------------------------------------------------------------------

func (a *API) claims(w http.ResponseWriter, r *http.Request) (Claims, bool) {
	token := Token(r)
	if token == "" {
		detail(w, http.StatusUnauthorized, "Sign in first")
		return nil, false
	}
	claims, err := a.auth.Verify(token)
	if err != nil {
		detail(w, http.StatusUnauthorized, "Sign in again")
		return nil, false
	}
	return claims, true
}

func (a *API) member(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.claims(w, r); ok {
			next(w, r)
		}
	}
}

func (a *API) admin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := a.claims(w, r)
		if !ok {
			return
		}
		if !claims.Admin() {
			detail(w, http.StatusForbidden, "Admin access required")
			return
		}
		next(w, r)
	}
}

// ---- requests -------------------------------------------------------------------------

func decode(w http.ResponseWriter, r *http.Request, limit int64, into any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit)).Decode(into); err != nil {
		detail(w, http.StatusUnprocessableEntity, "The request isn't valid JSON")
		return false
	}
	return true
}

type completeRequest struct {
	Messages    []Message      `json:"messages"`
	MaxTokens   *int           `json:"max_tokens"`
	Temperature *float64       `json:"temperature"`
	Stop        []string       `json:"stop"`
	Priority    *string        `json:"priority"`
	JSONSchema  map[string]any `json:"json_schema"`
}

// options checks the request as the apps' service did, and fills in its defaults.
func (c *completeRequest) options() (Options, string) {
	if len(c.Messages) < 1 || len(c.Messages) > 64 {
		return Options{}, "messages: 1 to 64"
	}
	for _, m := range c.Messages {
		if m.Role != "system" && m.Role != "user" && m.Role != "assistant" {
			return Options{}, "messages: role is system, user or assistant"
		}
		if utf8.RuneCountInString(m.Content) > 32_000 {
			return Options{}, "messages: content is at most 32000 characters"
		}
	}
	o := Options{Temperature: 0.2, MaxTokens: 300, Priority: Interactive, Stop: c.Stop, Schema: c.JSONSchema}
	if c.MaxTokens != nil {
		o.MaxTokens = *c.MaxTokens
	}
	if c.Temperature != nil {
		o.Temperature = *c.Temperature
	}
	if c.Priority != nil {
		o.Priority = *c.Priority
	}
	switch {
	case o.MaxTokens < 1 || o.MaxTokens > 1024:
		return Options{}, "max_tokens: 1 to 1024"
	case o.Temperature < 0 || o.Temperature > 1.5:
		return Options{}, "temperature: 0 to 1.5"
	case len(o.Stop) > 8:
		return Options{}, "stop: at most 8"
	}
	if _, err := rank(o.Priority); err != nil {
		return Options{}, "priority: interactive, background or overnight"
	}
	return o, ""
}

// answerError: Unavailable is a 503 with its words; anything else a 500.
func answerError(w http.ResponseWriter, err error) {
	var u *Unavailable
	switch {
	case errors.As(err, &u):
		detail(w, http.StatusServiceUnavailable, u.Message)
	case errors.Is(err, context.Canceled):
		// The asker went away: no one to answer.
	default:
		log.Printf("an answer failed: %v", err)
		detail(w, http.StatusInternalServerError, "Myira couldn't answer. Try again.")
	}
}

// ---- answers --------------------------------------------------------------------------

// complete: one answer, forgotten straight after.
func (a *API) complete(w http.ResponseWriter, r *http.Request) {
	var payload completeRequest
	if !decode(w, r, 4<<20, &payload) {
		return
	}
	options, invalid := payload.options()
	if invalid != "" {
		detail(w, http.StatusUnprocessableEntity, invalid)
		return
	}
	answer, err := a.engine.Complete(r.Context(), payload.Messages, options)
	if err != nil {
		answerError(w, err)
		return
	}
	reply(w, http.StatusOK, map[string]string{"reply": answer})
}

// completeStream: the same answer, sent as it is written, one JSON object a
// line: {"text": "..."} for each piece, then {"done": true}, or {"error":
// "..."} if it stops part way. No model is a 503 before anything is sent.
func (a *API) completeStream(w http.ResponseWriter, r *http.Request) {
	var payload completeRequest
	if !decode(w, r, 4<<20, &payload) {
		return
	}
	options, invalid := payload.options()
	if invalid != "" {
		detail(w, http.StatusUnprocessableEntity, invalid)
		return
	}
	if err := a.engine.Ready(); err != nil {
		answerError(w, err)
		return
	}
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	if flusher != nil {
		flusher.Flush()
	}
	line := func(v any) error {
		raw, _ := json.Marshal(v)
		if _, err := w.Write(append(raw, '\n')); err != nil {
			return err
		}
		if flusher != nil {
			flusher.Flush()
		}
		return nil
	}
	err := a.engine.Stream(r.Context(), payload.Messages, options, func(text string) error {
		return line(map[string]string{"text": text})
	})
	var u *Unavailable
	switch {
	case err == nil:
		_ = line(map[string]bool{"done": true})
	case errors.As(err, &u):
		_ = line(map[string]string{"error": u.Message})
	case errors.Is(err, context.Canceled):
	default:
		log.Printf("an answer stopped part way: %v", err)
		_ = line(map[string]string{"error": "Myira couldn't answer. Try again."})
	}
}

type describeRequest struct {
	ImageJPEGBase64 string  `json:"image_jpeg_base64"`
	Prompt          string  `json:"prompt"`
	MaxTokens       *int    `json:"max_tokens"`
	Priority        *string `json:"priority"`
}

// describe: a picture looked at by the box's model (Family Photos'
// descriptions, Family Vault reading a scan). Asked from Photos' worker,
// where no member is signed in, so it takes no token: the service is local
// only, and an image in gives text out.
func (a *API) describe(w http.ResponseWriter, r *http.Request) {
	var payload describeRequest
	if !decode(w, r, 6<<20, &payload) {
		return
	}
	maxTokens, priority := 90, Background
	if payload.MaxTokens != nil {
		maxTokens = *payload.MaxTokens
	}
	if payload.Priority != nil {
		priority = *payload.Priority
	}
	prompt := utf8.RuneCountInString(payload.Prompt)
	switch {
	case len(payload.ImageJPEGBase64) > 4_000_000:
		detail(w, http.StatusUnprocessableEntity, "image_jpeg_base64: at most 4000000 characters")
		return
	case prompt < 1 || prompt > 4000:
		detail(w, http.StatusUnprocessableEntity, "prompt: 1 to 4000 characters")
		return
	case maxTokens < 1 || maxTokens > 512:
		detail(w, http.StatusUnprocessableEntity, "max_tokens: 1 to 512")
		return
	}
	if _, err := rank(priority); err != nil {
		detail(w, http.StatusUnprocessableEntity, "priority: interactive, background or overnight")
		return
	}
	if _, err := base64.StdEncoding.Strict().DecodeString(payload.ImageJPEGBase64); err != nil {
		detail(w, http.StatusBadRequest, "The image isn't base64")
		return
	}
	answer, model, err := a.engine.Describe(r.Context(), "data:image/jpeg;base64,"+payload.ImageJPEGBase64,
		payload.Prompt, maxTokens, priority)
	if err != nil {
		answerError(w, err)
		return
	}
	var modelAny any
	if model != "" {
		modelAny = model
	}
	reply(w, http.StatusOK, map[string]any{"reply": answer, "model": modelAny})
}

// ---- the model ------------------------------------------------------------------------

// current: which model is chosen and loaded, and whether it can describe
// photos (Photos' worker asks before queuing descriptions). Only file names.
func (a *API) current(w http.ResponseWriter, _ *http.Request) {
	var loaded any
	if file := a.engine.LoadedFile(); file != "" {
		loaded = file
	}
	reply(w, http.StatusOK, map[string]any{
		"file":   a.engine.SelectedFile(),
		"loaded": loaded,
		"sees":   a.engine.CanSee(),
	})
}

func (a *API) model(w http.ResponseWriter, _ *http.Request) {
	reply(w, http.StatusOK, a.engine.StatusPayload())
}

// choose: saved, then loaded in the background.
func (a *API) choose(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		File string `json:"file"`
	}
	if !decode(w, r, 4096, &payload) {
		return
	}
	if n := utf8.RuneCountInString(payload.File); n < 1 || n > 255 {
		detail(w, http.StatusUnprocessableEntity, "file: 1 to 255 characters")
		return
	}
	err := a.engine.Choose(payload.File)
	var u *Unavailable
	switch {
	case errors.Is(err, errNotInstalled):
		detail(w, http.StatusBadRequest, err.Error())
		return
	case errors.As(err, &u):
		detail(w, http.StatusConflict, u.Message)
		return
	case err != nil:
		log.Printf("the model choice wasn't saved: %v", err)
		detail(w, http.StatusInternalServerError, "The choice wasn't saved. Try again.")
		return
	}
	reply(w, http.StatusOK, a.engine.StatusPayload())
}
